package billing

import "time"

// DiscountBpsFull 折扣基点满值（10000 = 原价无折扣）。bps 折扣的真相口径。
const DiscountBpsFull = 10000

// 管理可持久化的定价配置（admin 后台读写，用户端 purchase-config / quote 读取）。
// 用单行 JSON 配置存储（id 固定 1），避免为每个折扣档位单独建表；启动时幂等 seed 默认值。

// PricingConfig 单行定价配置（id 固定 1）。各结构化字段以 JSON 序列化进 text 列。
type PricingConfig struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	PaymentMethodsJSON  string    `gorm:"type:text" json:"-"`
	RechargePresetsJSON string    `gorm:"type:text" json:"-"`
	KindsJSON           string    `gorm:"type:text" json:"-"`
	RuntimeJSON         string    `gorm:"type:text" json:"-"`
	UpdatedAt           time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (PricingConfig) TableName() string { return "billing_pricing_config" }

// PaymentMethod 支付方式（开关 + 排序）。
type PaymentMethod struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Sort    int    `json:"sort"`
}

// KindPricing 某 kind（seat/boot_slot）的定价配置。
type KindPricing struct {
	UnitPriceCents  int64            `json:"unit_price_cents"`
	UnitLabel       string           `json:"unit_label"`
	QtyOptions      []int            `json:"qty_options"`
	QtyTiers        []QtyTierCfg     `json:"qty_tiers"`
	DurationUnit    string           `json:"duration_unit"` // month / day
	DurationOptions []DurationOptCfg `json:"duration_options"`
	Notice          string           `json:"notice"`
	BillingNote     string           `json:"billing_note"`
}

// QtyTierCfg 数量阶梯（JSON 形状贴合契约）。
type QtyTierCfg struct {
	MinQuantity int `json:"min_quantity"`
	DiscountBps int `json:"discount_bps"`
}

// DurationOptCfg 时长选项（JSON 形状贴合契约）。
type DurationOptCfg struct {
	Value       int `json:"value"`
	DiscountBps int `json:"discount_bps"`
}

// RuntimePackCfg 临时开机时长包配置。
type RuntimePackCfg struct {
	UnitPriceCentsPerMinute int64         `json:"unit_price_cents_per_minute"`
	MinMinutes              int           `json:"min_minutes"`
	Packs                   []RuntimePack `json:"packs"`
	Notice                  string        `json:"notice"`
	DailyCapMinutes         int           `json:"daily_cap_minutes"`
	RecycleRetentionDays    int           `json:"recycle_retention_days"`
}

// RuntimePack 时长包预设。
type RuntimePack struct {
	Minutes     int `json:"minutes"`
	DiscountBps int `json:"discount_bps"`
}

// PricingConfigData 反序列化后的完整配置（服务层与 handler 使用）。
type PricingConfigData struct {
	PaymentMethods  []PaymentMethod        `json:"payment_methods"`
	RechargePresets []int64                `json:"recharge_presets_cents"`
	Kinds           map[string]KindPricing `json:"kinds"`
	Runtime         RuntimePackCfg         `json:"runtime_pack"`
}
