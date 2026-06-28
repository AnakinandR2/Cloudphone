package library

import "time"

// OverviewUsage 用量块（含超额锁定标志）。
type OverviewUsage struct {
	UsedBytes     int64 `json:"used_bytes"`
	CapacityBytes int64 `json:"capacity_bytes"`
	Locked        bool  `json:"locked"` // used_bytes > capacity_bytes 时超额锁定
}

// OverviewSubscription 当前订阅块。
type OverviewSubscription struct {
	TierCode          string     `json:"tier_code"`
	CapacityBytes     int64      `json:"capacity_bytes"`
	MonthlyPriceCents int64      `json:"monthly_price_cents"`
	ExpireAt          *time.Time `json:"expire_at"`
	IsFree            bool       `json:"is_free"`
}

// Overview 概览（§8 GET /library/overview）。
type Overview struct {
	Usage           OverviewUsage        `json:"usage"`
	Subscription    OverviewSubscription `json:"subscription"`
	FreeQuotaBytes  int64                `json:"free_quota_bytes"`
	Tiers           []TierCfg            `json:"tiers"`
	CustomTier      CustomTierCfg        `json:"custom_tier"`
	DurationOptions []DurationOptCfg     `json:"duration_options"`
	Notice          string               `json:"notice"`
	BillingNote     string               `json:"billing_note"`
}

// Overview 组装概览：用量 + 订阅 + 档位目录 + 免费额度 + 时长选项 + notice。
func (s *serviceImpl) Overview(userID int) (*Overview, error) {
	cfg, err := s.pricing.Get()
	if err != nil {
		return nil, err
	}
	cur, err := s.currentSubscription(userID)
	if err != nil {
		return nil, err
	}
	usage, err := s.repo.getUsage(userID)
	if err != nil {
		return nil, err
	}
	ov := &Overview{
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
		FreeQuotaBytes:  cfg.FreeQuotaBytes,
		Tiers:           cfg.Tiers,
		CustomTier:      cfg.CustomTier,
		DurationOptions: cfg.DurationOptions,
		Notice:          cfg.Notice,
		BillingNote:     cfg.BillingNote,
	}
	return ov, nil
}
