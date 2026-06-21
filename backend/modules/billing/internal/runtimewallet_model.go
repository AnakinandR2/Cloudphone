package billing

import "time"

// 临时开机时长余量（消耗池）+ 每台每天已扣量（封顶用）。
// 真相可由 ledger 推导，此处单独存快读副本用于开机校验与展示。

// RuntimeMinuteWallet 用户临时开机时长余量（分钟）。一用户一行。
type RuntimeMinuteWallet struct {
	UserID           uint      `gorm:"primaryKey" json:"user_id"`
	RemainingMinutes int64     `gorm:"not null;default:0" json:"remaining_minutes"`
	UpdatedAt        time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (RuntimeMinuteWallet) TableName() string { return "billing_runtime_minute_wallets" }

// RuntimeDailyUsage 「某台·某天(UTC+8)」已扣临时时长，达封顶值停扣。
type RuntimeDailyUsage struct {
	ID             uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         uint   `gorm:"not null;index:idx_rt_daily,priority:1" json:"user_id"`
	InstanceID     string `gorm:"type:varchar(64);not null;index:idx_rt_daily,priority:2" json:"instance_id"`
	DayUTC8        string `gorm:"type:varchar(10);not null;index:idx_rt_daily,priority:3" json:"day_utc8"` // YYYY-MM-DD
	ChargedMinutes int    `gorm:"not null;default:0" json:"charged_minutes"`
}

func (RuntimeDailyUsage) TableName() string { return "billing_runtime_daily_usage" }

// 临时时长流水类型（subject=runtime_minute）。
const (
	LedgerRenew          = "renew"           // 续费授权单元
	LedgerRuntimeConsume = "runtime_consume" // 运行扣临时时长
)
