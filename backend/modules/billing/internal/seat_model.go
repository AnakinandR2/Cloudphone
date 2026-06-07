package billing

import "time"

// 欠费状态
const (
	DunningActive   = "active"
	DunningGrace    = "grace"
	DunningFrozen   = "frozen"
	DunningRecycled = "recycled"
)

// SeatUsage 用户实例席位占用计数（billing 记账，phone 创建/销毁时增减，周期校正）。
type SeatUsage struct {
	UserID            uint      `gorm:"primaryKey" json:"user_id"`
	InstanceSeatsUsed int64     `gorm:"not null;default:0" json:"instance_seats_used"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (SeatUsage) TableName() string { return "billing_seat_usages" }

// DunningState 用户欠费保护状态（cron 维护）。
type DunningState struct {
	UserID    uint      `gorm:"primaryKey" json:"user_id"`
	State     string    `gorm:"type:varchar(20);not null;default:'active'" json:"state"`
	EnteredAt time.Time `json:"entered_at"` // 进入当前(非 active)状态的时间
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (DunningState) TableName() string { return "billing_dunning_states" }

// EnforcementTarget phone 侧执行用：需冻结/回收的用户 + 其容量。
type EnforcementTarget struct {
	UserID   int    `json:"user_id"`
	State    string `json:"state"`
	Capacity int64  `json:"capacity"`
}
