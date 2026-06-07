package billing

import "time"

// SKU 类别
const (
	CategoryInstanceFee = "instance_fee" // 实例费：分/台/月
	CategoryBootPack    = "boot_pack"    // 包月开机包：分/台/月
	CategoryTimePack    = "time_pack"    // 时长包：分/小时
)

// DiscountBpsFull 全价（无折扣）基点。8500=8.5折=付85%。
const DiscountBpsFull = 10000

// Sku 可购买商品（运营维护）。
type Sku struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Code           string    `gorm:"type:varchar(64);not null;uniqueIndex:idx_billing_sku_code" json:"code"`
	Category       string    `gorm:"type:varchar(20);not null" json:"category"`
	Name           string    `gorm:"type:varchar(100);not null" json:"name"`
	Description    string    `gorm:"type:varchar(255)" json:"description"`
	UnitPriceCents int64     `gorm:"not null" json:"unit_price_cents"`
	Unit           string    `gorm:"type:varchar(20)" json:"unit"`
	Listed         bool      `gorm:"not null;default:true" json:"listed"`
	Sort           int       `gorm:"not null;default:0" json:"sort"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Sku) TableName() string { return "billing_skus" }

// DiscountTier 折扣阶梯：同周期下，达到 MinQuantity 起按 DiscountBps 计价。
type DiscountTier struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	SkuID       uint      `gorm:"not null;index:idx_billing_tier_sku" json:"sku_id"`
	CycleMonths int       `gorm:"not null;default:0" json:"cycle_months"`
	MinQuantity int       `gorm:"not null;default:1" json:"min_quantity"`
	DiscountBps int       `gorm:"not null;default:10000" json:"discount_bps"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (DiscountTier) TableName() string { return "billing_discount_tiers" }

type SkuCreate struct {
	Code           string `json:"code" binding:"required"`
	Category       string `json:"category" binding:"required"`
	Name           string `json:"name" binding:"required"`
	Description    string `json:"description"`
	UnitPriceCents int64  `json:"unit_price_cents" binding:"required"`
	Unit           string `json:"unit"`
	Listed         *bool  `json:"listed"`
	Sort           int    `json:"sort"`
}

type SkuUpdate struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	UnitPriceCents *int64 `json:"unit_price_cents"`
	Unit           string `json:"unit"`
	Listed         *bool  `json:"listed"`
	Sort           *int   `json:"sort"`
}

type TierCreate struct {
	CycleMonths int `json:"cycle_months"`
	MinQuantity int `json:"min_quantity" binding:"required"`
	DiscountBps int `json:"discount_bps" binding:"required"`
}

type TierUpdate struct {
	CycleMonths *int `json:"cycle_months"`
	MinQuantity *int `json:"min_quantity"`
	DiscountBps *int `json:"discount_bps"`
}

type SkuWithTiers struct {
	Sku   Sku            `json:"sku"`
	Tiers []DiscountTier `json:"tiers"`
}

type QuoteResult struct {
	SkuCode        string `json:"sku_code"`
	SkuName        string `json:"sku_name"`
	Category       string `json:"category"`
	CycleMonths    int    `json:"cycle_months"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	BillingUnits   int    `json:"billing_units"`
	OriginalCents  int64  `json:"original_cents"`
	DiscountBps    int    `json:"discount_bps"`
	PayableCents   int64  `json:"payable_cents"`
}
