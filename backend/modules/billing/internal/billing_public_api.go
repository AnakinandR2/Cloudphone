package billing

import (
	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// 公开营销接口（免鉴权）：供营销站 www 的 /pricing 价格页取数。
// 挂在引擎根的独立基址 /api/open/v1/billing/*（沿用 openapi 模块约定，避免 /api/v1 前缀叠加）。
// 仅返回裁剪过的「营销视图」DTO，不含内部字段（回收保留天数、计费说明等）。

// publicKindPricing 某可购买资源(kind)的对外定价。
type publicKindPricing struct {
	UnitPriceCents  int64            `json:"unit_price_cents"`
	UnitLabel       string           `json:"unit_label"`
	DurationUnit    string           `json:"duration_unit"`
	QtyTiers        []QtyTierCfg     `json:"qty_tiers"`
	DurationOptions []DurationOptCfg `json:"duration_options"`
}

// publicRuntimePack 临时开机时长的对外定价（含每席位每月赠送）。
type publicRuntimePack struct {
	UnitPriceCentsPerMinute int64         `json:"unit_price_cents_per_minute"`
	MinMinutes              int           `json:"min_minutes"`
	Packs                   []RuntimePack `json:"packs"`
	DailyCapMinutes         int           `json:"daily_cap_minutes"`
	GiftMinutesPerSeatMonth int           `json:"gift_minutes_per_seat_month"`
}

type publicPricing struct {
	Kinds           map[string]publicKindPricing `json:"kinds"`
	RuntimePack     publicRuntimePack            `json:"runtime_pack"`
	RechargePresets []int64                      `json:"recharge_presets_cents"`
}

// OpenGetPricing GET /api/open/v1/billing/pricing —— 公开定价（营销页）。
func OpenGetPricing(c *gin.Context) {
	cfg, err := PricingConfigService.Get()
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	kinds := map[string]publicKindPricing{}
	for _, k := range []string{KindSeat, KindBootSlot} {
		kp, ok := cfg.Kinds[k]
		if !ok {
			continue
		}
		kinds[k] = publicKindPricing{
			UnitPriceCents:  kp.UnitPriceCents,
			UnitLabel:       kp.UnitLabel,
			DurationUnit:    kp.DurationUnit,
			QtyTiers:        kp.QtyTiers,
			DurationOptions: kp.DurationOptions,
		}
	}
	framework.OKWithData(c, publicPricing{
		Kinds: kinds,
		RuntimePack: publicRuntimePack{
			UnitPriceCentsPerMinute: cfg.Runtime.UnitPriceCentsPerMinute,
			MinMinutes:              cfg.Runtime.MinMinutes,
			Packs:                   cfg.Runtime.Packs,
			DailyCapMinutes:         cfg.Runtime.DailyCapMinutes,
			GiftMinutesPerSeatMonth: cfg.Runtime.GiftMinutesPerSeatMonth,
		},
		RechargePresets: cfg.RechargePresets,
	})
}

// publicTrialItem 试用发放项（对外）。
type publicTrialItem struct {
	Subject    string `json:"subject"`
	Quantity   int64  `json:"quantity"`
	ExpireDays int    `json:"expire_days"`
}

type publicTrialPolicy struct {
	Name  string            `json:"name"`
	Items []publicTrialItem `json:"items"`
}

// OpenGetTrialOverview GET /api/open/v1/billing/trial-overview —— 公开营销试用（单条）。
// 返回被标记营销展示且启用的那条；无则 policy=null（营销页据此隐藏试用区）。
func OpenGetTrialOverview(c *gin.Context) {
	p, err := TrialService.FeaturedPolicy()
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	if p == nil {
		framework.OKWithData(c, gin.H{"policy": nil})
		return
	}
	items := make([]publicTrialItem, 0, len(p.Items))
	for _, it := range p.Items {
		items = append(items, publicTrialItem{Subject: it.Subject, Quantity: it.Quantity, ExpireDays: it.ExpireDays})
	}
	framework.OKWithData(c, gin.H{"policy": publicTrialPolicy{Name: p.Name, Items: items}})
}

// registerBillingPublicRoutes 公开营销接口（免鉴权），挂引擎根 /api/open/v1/billing。
func registerBillingPublicRoutes(r *gin.Engine) {
	g := r.Group("/api/open/v1/billing")
	{
		g.GET("/pricing", OpenGetPricing)
		g.GET("/trial-overview", OpenGetTrialOverview)
	}
}
