package library

import (
	"encoding/json"
	"math"
	"time"

	"manager-backend/framework/apperr"
)

// Service 素材库业务服务（本步聚焦商业核心：计价 + 四动作 + 履约）。
// 文件/夹/标签/上传/锁定/cron 等留待第 3 步补全。
type serviceImpl struct {
	repo    repository
	pricing *pricingConfigServiceImpl
	now     func() time.Time // 可注入，便于测试

	// presigned 短链有效期（可配，由 module.Init 从 framework.AppConfig 注入）。
	getTTL time.Duration // 下载 GET TTL
	putTTL time.Duration // 上传 PUT TTL
}

// Service 模块内单例，由 module.Init 注入。
var Service *serviceImpl

func newService(repo repository, pricing *pricingConfigServiceImpl, getTTL, putTTL time.Duration) *serviceImpl {
	return &serviceImpl{repo: repo, pricing: pricing, now: time.Now, getTTL: getTTL, putTTL: putTTL}
}

// ---- 报价/下单请求与载荷（§2.2）----

// PackageParams 套餐下单/报价的不透明请求载荷（billing params 反序列化得到）。
//   - 预设档带 tier_code；自定义档带 capacity_gb（tier_code 留空或为 "custom"）。
//   - upgrade/downgrade 不带 days（不可选时长）。
//   - action 可留空，由服务端按"选中月价 vs 当前订阅月价"自动判定。
type PackageParams struct {
	Action     string `json:"action"`
	TierCode   string `json:"tier_code"`
	CapacityGB int    `json:"capacity_gb"`
	Days       int    `json:"days"`
}

// PackageMeta 存入订单项 meta_json 的载荷（§2.2），履约时回传。
type PackageMeta struct {
	Action                string     `json:"action"`
	TierCode              string     `json:"tier_code"`
	CapacityBytes         int64      `json:"capacity_bytes"`
	Days                  int        `json:"days"`
	MonthlyPriceCents     int64      `json:"monthly_price_cents"`
	PrevTierCode          string     `json:"prev_tier_code"`
	PrevMonthlyPriceCents int64      `json:"prev_monthly_price_cents"`
	NewExpireAt           *time.Time `json:"new_expire_at"`
}

// QuoteResult 报价结果（rich 预览 + 权威价 + meta）。
type QuoteResult struct {
	Action            string      `json:"action"`
	TierCode          string      `json:"tier_code"`
	TierName          string      `json:"tier_name"`
	CapacityBytes     int64       `json:"capacity_bytes"`
	Days              int         `json:"days"`
	MonthlyPriceCents int64       `json:"monthly_price_cents"`
	TierDiscountBps   int         `json:"tier_discount_bps"`
	DurationDiscBps   int         `json:"duration_discount_bps"`
	RemainingDays     int         `json:"remaining_days"`
	TotalCents        int64       `json:"total_cents"`
	NewExpireAt       *time.Time  `json:"new_expire_at"`
	Meta              PackageMeta `json:"-"`
}

// ---- 当前订阅快照 ----

// currentSub 取用户当前生效套餐快照（含免费态归一化）。
type currentSub struct {
	tierCode     string
	capacity     int64
	monthlyCents int64
	expireAt     *time.Time
	isFree       bool
}

// currentSubscription 取当前生效订阅的实时视图（统一归一化）：
//   - 无行 / tier_code=free / tier_code 空 → 免费态（容量取实时 FreeQuotaBytes，§6 minor-6）。
//   - 付费订阅但 expire_at 非空且已 <= now（已过期未被 cron 处理）→ 等价过期降级，归一化为免费态
//     （isFree=true、容量=实时 FreeQuotaBytes、monthly=0、expireAt=nil）。这样 QuotePackage 对已过期
//     付费订阅只允许 ActionNew（全价新购），杜绝 remaining_days=0 的 0 元升级（major-2）。
func (s *serviceImpl) currentSubscription(userID int) (currentSub, error) {
	sub, err := s.repo.getSubscription(userID)
	if err != nil {
		return currentSub{}, err
	}
	expired := sub != nil && sub.TierCode != TierFree && sub.TierCode != "" &&
		sub.ExpireAt != nil && !sub.ExpireAt.After(s.now())
	if sub == nil || sub.TierCode == TierFree || sub.TierCode == "" || expired {
		// 免费态容量一律用实时定价配置（忽略订阅行旧快照，minor-6）。
		freeBytes, err := s.pricing.FreeQuotaBytes()
		if err != nil {
			return currentSub{}, err
		}
		return currentSub{tierCode: TierFree, capacity: freeBytes, monthlyCents: 0, expireAt: nil, isFree: true}, nil
	}
	return currentSub{
		tierCode:     sub.TierCode,
		capacity:     sub.CapacityBytes,
		monthlyCents: sub.MonthlyPriceCents,
		expireAt:     sub.ExpireAt,
		isFree:       false,
	}, nil
}

// ---- 选中档位解析 ----

// selectedTier 解析请求中的目标档位（预设或自定义），返回月价/容量/档位折扣/展示名/code。
type selectedTier struct {
	code         string
	name         string
	capacityB    int64
	monthlyCents int64
	tierBps      int
}

func (s *serviceImpl) resolveTier(cfg *LibraryPricingConfigData, p PackageParams) (selectedTier, error) {
	// 自定义档：tier_code 为空或显式 custom，且带 capacity_gb。
	if p.TierCode == "" || p.TierCode == "custom" {
		ct := cfg.CustomTier
		if !ct.Enabled {
			return selectedTier{}, apperr.Validation("未启用自定义档位")
		}
		if p.CapacityGB < ct.MinCapacityGB {
			return selectedTier{}, apperr.Validation("低于自定义档最小容量")
		}
		monthly := int64(p.CapacityGB) * ct.PricePerGBMonthCents
		return selectedTier{
			code:         "custom",
			name:         "自定义容量",
			capacityB:    int64(p.CapacityGB) * GiB,
			monthlyCents: monthly,
			tierBps:      ct.TierDiscountBps,
		}, nil
	}
	t, ok := cfg.tierByCode(p.TierCode)
	if !ok {
		return selectedTier{}, apperr.Validation("未知的容量档位")
	}
	if !t.Enabled {
		return selectedTier{}, apperr.Validation("该档位已停用")
	}
	return selectedTier{
		code:         t.Code,
		name:         t.Name,
		capacityB:    int64(t.CapacityGB) * GiB,
		monthlyCents: t.MonthlyPriceCents,
		tierBps:      t.TierDiscountBps,
	}, nil
}

// remainingDays 剩余天数 = ceil((expire_at − now) / 24h)；expire_at<=now 或 nil 视为 0。
func remainingDays(now time.Time, expireAt *time.Time) int {
	if expireAt == nil {
		return 0
	}
	d := expireAt.Sub(now)
	if d <= 0 {
		return 0
	}
	return int(math.Ceil(d.Hours() / 24.0))
}

// QuotePackage 服务端权威报价 + 四动作判定（§4）。
func (s *serviceImpl) QuotePackage(userID int, params []byte) (QuoteResult, error) {
	var p PackageParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return QuoteResult{}, apperr.Validation("套餐参数格式错误")
		}
	}
	cfg, err := s.pricing.Get()
	if err != nil {
		return QuoteResult{}, err
	}
	tier, err := s.resolveTier(cfg, p)
	if err != nil {
		return QuoteResult{}, err
	}
	cur, err := s.currentSubscription(userID)
	if err != nil {
		return QuoteResult{}, err
	}
	now := s.now()

	// 动作判定（§4.2）：按"选中档位月价 vs 当前订阅月价"。
	action := s.decideAction(cur, tier)

	res := QuoteResult{
		TierCode:          tier.code,
		TierName:          tier.name,
		CapacityBytes:     tier.capacityB,
		MonthlyPriceCents: tier.monthlyCents,
		TierDiscountBps:   tier.tierBps,
		Action:            action,
	}

	switch action {
	case ActionNew:
		dur, err := s.requireDuration(cfg, p.Days)
		if err != nil {
			return QuoteResult{}, err
		}
		res.Days = dur.Days
		res.DurationDiscBps = dur.DiscountBps
		res.TotalCents = standardPrice(tier.monthlyCents, dur.Days, tier.tierBps, dur.DiscountBps)
		exp := now.Add(time.Duration(dur.Days) * 24 * time.Hour)
		res.NewExpireAt = &exp
	case ActionRenew:
		dur, err := s.requireDuration(cfg, p.Days)
		if err != nil {
			return QuoteResult{}, err
		}
		res.Days = dur.Days
		res.DurationDiscBps = dur.DiscountBps
		res.TotalCents = standardPrice(tier.monthlyCents, dur.Days, tier.tierBps, dur.DiscountBps)
		// 续费容量沿用当前订阅容量（同档，理论上等于选中档位容量；显式取当前防漂移）。
		res.CapacityBytes = cur.capacity
		// 续费：max(now, 当前到期) + days。
		base := now
		if cur.expireAt != nil && cur.expireAt.After(now) {
			base = *cur.expireAt
		}
		exp := base.Add(time.Duration(dur.Days) * 24 * time.Hour)
		res.NewExpireAt = &exp
	case ActionUpgrade:
		rd := remainingDays(now, cur.expireAt)
		res.RemainingDays = rd
		res.DurationDiscBps = DiscountBpsFull
		res.TotalCents = upgradePrice(rd, tier.monthlyCents, cur.monthlyCents)
		res.NewExpireAt = cur.expireAt // 到期不变
	case ActionDowngrade:
		res.RemainingDays = remainingDays(now, cur.expireAt)
		res.DurationDiscBps = DiscountBpsFull
		res.TotalCents = 0             // 降级=0
		res.NewExpireAt = cur.expireAt // 到期不变
	}

	res.Meta = PackageMeta{
		Action:                action,
		TierCode:              tier.code,
		CapacityBytes:         res.CapacityBytes,
		Days:                  res.Days,
		MonthlyPriceCents:     tier.monthlyCents,
		PrevTierCode:          cur.tierCode,
		PrevMonthlyPriceCents: cur.monthlyCents,
		NewExpireAt:           res.NewExpireAt,
	}
	return res, nil
}

// decideAction 按 §4.2 判定四种动作（major-1：续费判定改为"同档"，不再用月价相等）。
//
// 判定优先级：
//  1. 免费态（含归一化的过期态）→ 只能 ActionNew（全价新购）。
//  2. 同档 → ActionRenew：选中档位身份 == 当前订阅档位身份
//     （预设档比 tier_code；自定义档比 code=="custom" 且 capacity_bytes 相等）。
//  3. 非同档 → 用月价区分 upgrade/downgrade：
//     upgrade ⟺ 选中月价 > 当前月价，或（月价相等且选中容量 > 当前容量）；否则 downgrade。
//
// 这样两个不同档位即便配成同月价也不会被误判为 renew。
func (s *serviceImpl) decideAction(cur currentSub, tier selectedTier) string {
	if cur.isFree {
		return ActionNew
	}
	if sameTier(cur, tier) {
		return ActionRenew
	}
	if tier.monthlyCents > cur.monthlyCents ||
		(tier.monthlyCents == cur.monthlyCents && tier.capacityB > cur.capacity) {
		return ActionUpgrade
	}
	return ActionDowngrade
}

// sameTier 判定选中档位身份是否等于当前订阅档位身份。
//   - 自定义档：双方 code 都为 "custom" 且容量字节相等。
//   - 预设档：tier_code 相等（且非 custom）。
func sameTier(cur currentSub, tier selectedTier) bool {
	if tier.code == "custom" || cur.tierCode == "custom" {
		return tier.code == "custom" && cur.tierCode == "custom" && tier.capacityB == cur.capacity
	}
	return tier.code == cur.tierCode
}

// requireDuration 校验并取时长选项（new/renew 必须选合法时长）。
func (s *serviceImpl) requireDuration(cfg *LibraryPricingConfigData, days int) (*DurationOptCfg, error) {
	if days <= 0 {
		return nil, apperr.Validation("请选择套餐时长")
	}
	d, ok := cfg.durationByDays(days)
	if !ok {
		return nil, apperr.Validation("不支持的套餐时长")
	}
	return d, nil
}

// FulfillPackage 履约：解析 meta → upsert 订阅快照（§2.2）。支付成功后由 billing 回调。
//
// minor-5：new/renew 的到期在履约时按 max(now, 当前到期) + days 重算（days 取自 meta），
// 避免直接用 quote 时刻快照 meta.NewExpireAt（下单到支付之间可能跨天）。
// upgrade/downgrade 到期不变（沿用当前订阅到期）。容量/月价仍取 meta 快照。
func (s *serviceImpl) FulfillPackage(userID int, orderID uint, metaJSON []byte) error {
	var meta PackageMeta
	if len(metaJSON) == 0 {
		return apperr.Validation("套餐履约载荷为空")
	}
	if err := json.Unmarshal(metaJSON, &meta); err != nil {
		return apperr.Validation("套餐履约载荷格式错误")
	}

	expireAt := meta.NewExpireAt
	switch meta.Action {
	case ActionNew, ActionRenew:
		// 取当前订阅原始到期（不归一化：renew 需基于真实当前到期累加）。
		base := s.now()
		if curSub, err := s.repo.getSubscription(userID); err != nil {
			return err
		} else if curSub != nil && curSub.ExpireAt != nil && curSub.ExpireAt.After(base) {
			base = *curSub.ExpireAt
		}
		exp := base.Add(time.Duration(meta.Days) * 24 * time.Hour)
		expireAt = &exp
	case ActionUpgrade, ActionDowngrade:
		// 到期不变：以当前订阅到期为准（meta.NewExpireAt 即 quote 时的当前到期）。
		if curSub, err := s.repo.getSubscription(userID); err != nil {
			return err
		} else if curSub != nil {
			expireAt = curSub.ExpireAt
		}
	}

	sub := &LibrarySubscription{
		UserID:            uint(userID),
		TierCode:          meta.TierCode,
		CapacityBytes:     meta.CapacityBytes,
		MonthlyPriceCents: meta.MonthlyPriceCents,
		ExpireAt:          expireAt,
		Status:            SubActive,
	}
	return s.repo.upsertSubscription(sub)
}

// ---- billing 注册用闭包入口（签名匹配 billing.BizQuoteFunc / BizFulfillFunc）----

// quoteForBilling 供 billing 注册：返回总价 + meta_json。
func (s *serviceImpl) quoteForBilling(userID int, params []byte) (int64, []byte, error) {
	res, err := s.QuotePackage(userID, params)
	if err != nil {
		return 0, nil, err
	}
	meta, err := json.Marshal(res.Meta)
	if err != nil {
		return 0, nil, err
	}
	return res.TotalCents, meta, nil
}
