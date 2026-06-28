package library

import (
	"time"

	"manager-backend/framework/apperr"
)

// gap-7：后台查/调单个用户的素材库用量与订阅（spec §8）。

// AdminUserView 后台查看某用户的用量 + 订阅（含免费态归一化的实时视图）。
type AdminUserView struct {
	UserID       int                  `json:"user_id"`
	Usage        OverviewUsage        `json:"usage"`
	Subscription OverviewSubscription `json:"subscription"`
}

// AdminGetUser 取某用户的实时用量 + 订阅视图（走与前台一致的归一化，过期付费即视为免费态）。
func (s *serviceImpl) AdminGetUser(userID int) (*AdminUserView, error) {
	cur, err := s.currentSubscription(userID)
	if err != nil {
		return nil, err
	}
	usage, err := s.repo.getUsage(userID)
	if err != nil {
		return nil, err
	}
	return &AdminUserView{
		UserID: userID,
		Usage: OverviewUsage{
			UsedBytes:     usage.UsedBytes,
			CapacityBytes: cur.capacity,
			Locked:        usage.UsedBytes > cur.capacity,
		},
		Subscription: OverviewSubscription{
			TierCode:          cur.tierCode,
			CapacityBytes:     cur.capacity,
			MonthlyPriceCents: cur.monthlyCents,
			ExpireAt:          cur.expireAt,
			IsFree:            cur.isFree,
		},
	}, nil
}

// AdminGrantRequest 后台赠送/设置一个套餐（最小实现）。
//   - 预设档传 tier_code；自定义/直给容量传 capacity_gb（>0 时优先生效）。
//   - days>0 时设 expire_at = now + days；days<=0 视为永不过期（免费态语义）。
type AdminGrantRequest struct {
	TierCode   string `json:"tier_code"`
	CapacityGB int    `json:"capacity_gb"`
	Days       int    `json:"days"`
}

// AdminGrant 后台赠送/调整某用户的套餐（source 标记为 admin_grant）。
//   - tier_code=free（或容量按免费额度）→ 重置为免费态。
//   - 否则写入一个生效订阅：容量取 capacity_gb×GiB（缺省回退预设档容量），月价取预设档月价快照，
//     到期 = now+days（days<=0 则永不过期）。
func (s *serviceImpl) AdminGrant(userID int, req AdminGrantRequest) (*AdminUserView, error) {
	now := s.now()

	// 免费态赠送：tier_code=free 或未给任何档位/容量。
	if req.TierCode == TierFree || (req.TierCode == "" && req.CapacityGB <= 0) {
		freeBytes, err := s.pricing.FreeQuotaBytes()
		if err != nil {
			return nil, err
		}
		sub := &LibrarySubscription{
			UserID:            uint(userID),
			TierCode:          TierFree,
			CapacityBytes:     freeBytes,
			MonthlyPriceCents: 0,
			ExpireAt:          nil,
			Status:            SubActive,
		}
		if err := s.repo.upsertSubscription(sub); err != nil {
			return nil, err
		}
		return s.AdminGetUser(userID)
	}

	cfg, err := s.pricing.Get()
	if err != nil {
		return nil, err
	}
	capacityBytes := int64(req.CapacityGB) * GiB
	var monthly int64
	tierCode := req.TierCode
	if req.TierCode != "" && req.TierCode != "custom" {
		t, ok := cfg.tierByCode(req.TierCode)
		if !ok {
			return nil, apperr.Validation("未知的容量档位")
		}
		monthly = t.MonthlyPriceCents
		if capacityBytes <= 0 {
			capacityBytes = int64(t.CapacityGB) * GiB
		}
	} else {
		// 自定义/直给容量：月价按自定义档每 GiB 月价估算（仅作快照，便于后续补差价参考）。
		tierCode = "custom"
		monthly = int64(req.CapacityGB) * cfg.CustomTier.PricePerGBMonthCents
	}
	if capacityBytes <= 0 {
		return nil, apperr.Validation("容量必须大于0")
	}

	var expireAt *time.Time
	if req.Days > 0 {
		exp := now.Add(time.Duration(req.Days) * 24 * time.Hour)
		expireAt = &exp
	}
	sub := &LibrarySubscription{
		UserID:            uint(userID),
		TierCode:          tierCode,
		CapacityBytes:     capacityBytes,
		MonthlyPriceCents: monthly,
		ExpireAt:          expireAt,
		Status:            SubActive,
	}
	if err := s.repo.upsertSubscription(sub); err != nil {
		return nil, err
	}
	return s.AdminGetUser(userID)
}
