package billing

import "time"

const (
	OrderPaid = "paid"
)

const (
	PayBalance = "balance"
	PayWechat  = "wechat"
	PayAlipay  = "alipay"
)

var validPayMethods = map[string]bool{PayBalance: true, PayWechat: true, PayAlipay: true}

// Order 旧订单表（billing_orders）。新购买走 BizOrder；此表仍被 TrialService 用于
// countPaidOrders（试用资格判定「是否已付费用户」），故保留。
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
