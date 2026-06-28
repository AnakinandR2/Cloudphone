package library

import "time"

// 素材库定价配置（admin 单行 JSON，id 固定 1，启动幂等 seed）。参考 billing 的 PricingConfig 范式。

// LibraryPricingConfig 单行定价配置（id 固定 1）。结构化字段以 JSON 序列化进 text 列。
type LibraryPricingConfig struct {
	ID             uint      `gorm:"primaryKey" json:"id"` // 固定 1
	FreeQuotaBytes int64     `gorm:"not null;default:0" json:"free_quota_bytes"`
	TiersJSON      string    `gorm:"type:text" json:"-"` // []TierCfg
	CustomTierJSON string    `gorm:"type:text" json:"-"` // CustomTierCfg
	DurationsJSON  string    `gorm:"type:text" json:"-"` // []DurationOptCfg
	Notice         string    `gorm:"type:text" json:"notice"`
	BillingNote    string    `gorm:"type:text" json:"billing_note"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (LibraryPricingConfig) TableName() string { return "library_pricing_config" }

// TierCfg 预设容量档位。
type TierCfg struct {
	Code              string `json:"code"` // 如 t50 / t100 / t500
	Name              string `json:"name"`
	CapacityGB        int    `json:"capacity_gb"`
	MonthlyPriceCents int64  `json:"monthly_price_cents"`
	TierDiscountBps   int    `json:"tier_discount_bps"` // 10000=原价
	Enabled           bool   `json:"enabled"`
	Sort              int    `json:"sort"`
}

// CustomTierCfg 自定义档（按 GiB 计价）。
type CustomTierCfg struct {
	Enabled              bool  `json:"enabled"`
	MinCapacityGB        int   `json:"min_capacity_gb"`
	PricePerGBMonthCents int64 `json:"price_per_gb_month_cents"`
	TierDiscountBps      int   `json:"tier_discount_bps"`
}

// DurationOptCfg 时长选项（与 billing 同形：天 + 折扣）。
type DurationOptCfg struct {
	Days        int `json:"days"`
	DiscountBps int `json:"discount_bps"`
}

// LibraryPricingConfigData 反序列化后的完整配置（服务层与 handler 使用）。
type LibraryPricingConfigData struct {
	FreeQuotaBytes  int64            `json:"free_quota_bytes"`
	Tiers           []TierCfg        `json:"tiers"`
	CustomTier      CustomTierCfg    `json:"custom_tier"`
	DurationOptions []DurationOptCfg `json:"duration_options"`
	Notice          string           `json:"notice"`
	BillingNote     string           `json:"billing_note"`
}

// tierByCode 在配置里按 code 查启用档位。
func (d *LibraryPricingConfigData) tierByCode(code string) (*TierCfg, bool) {
	for i := range d.Tiers {
		if d.Tiers[i].Code == code {
			return &d.Tiers[i], true
		}
	}
	return nil, false
}

// durationByDays 按天数查时长选项。
func (d *LibraryPricingConfigData) durationByDays(days int) (*DurationOptCfg, bool) {
	for i := range d.DurationOptions {
		if d.DurationOptions[i].Days == days {
			return &d.DurationOptions[i], true
		}
	}
	return nil, false
}
