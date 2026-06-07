package billing

import "time"

// Account 计费账户：一用户一份，持有钱包余额（分）。
type Account struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       uint      `gorm:"not null;uniqueIndex:idx_billing_account_user" json:"user_id"`
	BalanceCents int64     `gorm:"not null;default:0" json:"balance_cents"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Account) TableName() string { return "billing_accounts" }

// LedgerEntry 统一流水：余额与（后续计划的）资源包每一次增减的不可变记录。
// 既是「费用日志」的底座，也是「手动调整/退款」的载体。
type LedgerEntry struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       uint      `gorm:"not null;index:idx_billing_ledger_user" json:"user_id"`
	Subject      string    `gorm:"type:varchar(20);not null" json:"subject"` // 科目：balance（计划1）；资源科目后续计划加
	Type         string    `gorm:"type:varchar(20);not null" json:"type"`    // topup/consume/adjust_grant/adjust_deduct/...
	Delta        int64     `gorm:"not null" json:"delta"`                    // 增减量：balance 科目=分；资源科目=台/分钟（正=增 负=减）
	BalanceAfter int64     `gorm:"not null" json:"balance_after"`            // 变更后余量快照：balance=分；资源=可用容量
	Reason       string    `gorm:"type:varchar(255)" json:"reason"`
	OrderID      uint      `gorm:"default:0;index" json:"order_id"`  // 关联订单（计划4）
	Operator     string    `gorm:"type:varchar(64)" json:"operator"` // user:<id> / staff:<id> / system
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (LedgerEntry) TableName() string { return "billing_ledger_entries" }

const SubjectBalance = "balance"

const (
	LedgerTopup        = "topup"
	LedgerConsume      = "consume"
	LedgerAdjustGrant  = "adjust_grant"
	LedgerAdjustDeduct = "adjust_deduct"
)

// TopupRequest 充值请求（计划1 为桩：直接入账）。
type TopupRequest struct {
	AmountCents int64 `json:"amount_cents" binding:"required"`
}

// AdjustRequest 后台余额调整（正=赠送 负=扣减），理由必填。
type AdjustRequest struct {
	DeltaCents int64  `json:"delta_cents" binding:"required"`
	Reason     string `json:"reason" binding:"required"`
}
