package billing

import "time"

const (
	OrderPending   = "pending"
	OrderPaid      = "paid"
	OrderCancelled = "cancelled"
)

const (
	PayBalance = "balance"
	PayWechat  = "wechat"
	PayAlipay  = "alipay"
)

var validPayMethods = map[string]bool{PayBalance: true, PayWechat: true, PayAlipay: true}

type Order struct {
	ID         uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	OrderNo    string     `gorm:"type:varchar(40);not null;uniqueIndex:idx_billing_order_no" json:"order_no"`
	UserID     uint       `gorm:"not null;index:idx_billing_order_user" json:"user_id"`
	Status     string     `gorm:"type:varchar(20);not null" json:"status"`
	PayMethod  string     `gorm:"type:varchar(20);not null" json:"pay_method"`
	TotalCents int64      `gorm:"not null" json:"total_cents"`
	PaidAt     *time.Time `json:"paid_at"`
	CreatedAt  time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Order) TableName() string { return "billing_orders" }

type OrderItem struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	OrderID        uint      `gorm:"not null;index:idx_billing_orderitem_order" json:"order_id"`
	SkuCode        string    `gorm:"type:varchar(64);not null" json:"sku_code"`
	SkuName        string    `gorm:"type:varchar(100)" json:"sku_name"`
	Category       string    `gorm:"type:varchar(20);not null" json:"category"`
	CycleMonths    int       `gorm:"not null;default:0" json:"cycle_months"`
	Quantity       int       `gorm:"not null" json:"quantity"`
	UnitPriceCents int64     `gorm:"not null" json:"unit_price_cents"`
	DiscountBps    int       `gorm:"not null;default:10000" json:"discount_bps"`
	OriginalCents  int64     `gorm:"not null" json:"original_cents"`
	PayableCents   int64     `gorm:"not null" json:"payable_cents"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (OrderItem) TableName() string { return "billing_order_items" }

type OrderItemRequest struct {
	SkuCode     string `json:"sku_code" binding:"required"`
	CycleMonths int    `json:"cycle_months"`
	Quantity    int    `json:"quantity" binding:"required"`
}

type OrderCreate struct {
	Items     []OrderItemRequest `json:"items" binding:"required,min=1"`
	PayMethod string             `json:"pay_method" binding:"required"`
}

type OrderDetail struct {
	Order Order       `json:"order"`
	Items []OrderItem `json:"items"`
}
