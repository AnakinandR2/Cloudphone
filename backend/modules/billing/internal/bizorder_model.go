package billing

import "time"

// 新购买模型订单（与旧 Order 并存；旧表 billing_orders 保留给 catalog/order_service 切换期使用）。

// 业务类型。
const (
	BizRecharge      = "recharge"
	BizSeatNew       = "seat_new"
	BizSeatRenew     = "seat_renew"
	BizBootSlotNew   = "boot_slot_new"
	BizBootSlotRenew = "boot_slot_renew"
	BizRuntimePack   = "runtime_pack"
)

// 订单状态。
const (
	BizOrderUnpaid  = "unpaid"
	BizOrderPaid    = "paid"
	BizOrderExpired = "expired"
)

var validBizTypes = map[string]bool{
	BizRecharge: true, BizSeatNew: true, BizSeatRenew: true,
	BizBootSlotNew: true, BizBootSlotRenew: true, BizRuntimePack: true,
}

// BizOrder 新模型订单。
type BizOrder struct {
	ID         uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     uint       `gorm:"not null;index:idx_bizorder_user" json:"user_id"`
	BizType    string     `gorm:"type:varchar(20);not null" json:"biz_type"`
	Status     string     `gorm:"type:varchar(16);not null" json:"status"`
	TotalCents int64      `gorm:"not null" json:"total_cents"`
	PayMethod  string     `gorm:"type:varchar(20);not null" json:"pay_method"`
	CreatedAt  time.Time  `gorm:"autoCreateTime" json:"created_at"`
	PaidAt     *time.Time `json:"paid_at"`
	ExpiredAt  *time.Time `json:"expired_at"`
}

func (BizOrder) TableName() string { return "billing_biz_orders" }

// BizOrderItem 新模型订单项。
type BizOrderItem struct {
	ID                  uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	OrderID             uint   `gorm:"not null;index:idx_bizitem_order" json:"order_id"`
	TargetKind          string `gorm:"type:varchar(16);not null" json:"target_kind"` // seat/boot_slot/runtime_minute/balance
	Quantity            int    `gorm:"not null" json:"quantity"`
	DurationValue       int    `gorm:"not null;default:0" json:"duration_value"`
	DurationUnit        string `gorm:"type:varchar(8)" json:"duration_unit"` // month/day
	UnitPriceCents      int64  `gorm:"not null" json:"unit_price_cents"`
	QtyDiscountBps      int    `gorm:"not null;default:10000" json:"qty_discount_bps"`
	DurationDiscountBps int    `gorm:"not null;default:10000" json:"duration_discount_bps"`
	AmountCents         int64  `gorm:"not null" json:"amount_cents"`
	RenewUnitIDs        string `gorm:"type:varchar(512)" json:"renew_unit_ids"` // 续费单元 ID（逗号分隔）
}

func (BizOrderItem) TableName() string { return "billing_biz_order_items" }

// BizOrderCreate 下单请求（契约 §1.5）。
type BizOrderCreate struct {
	BizType       string `json:"biz_type" binding:"required"`
	Quantity      int    `json:"quantity"`
	DurationValue int    `json:"duration_value"`
	UnitIDs       []uint `json:"unit_ids"`
	Minutes       int    `json:"minutes"`
	AmountCents   int64  `json:"amount_cents"`
	PayMethod     string `json:"pay_method" binding:"required"`
}

// BizQuoteRequest 报价请求（契约 §1.3）。
type BizQuoteRequest struct {
	BizType       string `json:"biz_type" binding:"required"`
	Quantity      int    `json:"quantity"`
	DurationValue int    `json:"duration_value"`
	Minutes       int    `json:"minutes"`
}

// PayResult 支付结果（余额=即时 paid；第三方=待支付 + stub 参数）。
type PayResult struct {
	Status    string                 `json:"status"` // paid / pending
	PayMethod string                 `json:"pay_method"`
	PayParams map[string]interface{} `json:"pay_params,omitempty"`
}

// BizOrderResult 下单/支付返回。
type BizOrderResult struct {
	Order BizOrder  `json:"order"`
	Pay   PayResult `json:"pay"`
}
