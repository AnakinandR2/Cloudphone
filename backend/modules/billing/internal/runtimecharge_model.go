package billing

import "time"

// 运行计费落账：每 tick、每台被计费实例写一条 runtime_charges。

// 占用类型。
const (
	QuotaBootSlot   = "boot_slot"   // 占包月名额，charged=0
	QuotaTemp       = "temp"        // 扣临时时长
	QuotaCappedFree = "capped_free" // 当天超封顶，免费
	QuotaMixed      = "mixed"       // 聚合展示用：一段开机会话内混合
)

// RuntimeCharge 一条运行扣费记录（分钟级）。
type RuntimeCharge struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         uint      `gorm:"not null;index:idx_rtc_user" json:"user_id"`
	InstanceID     string    `gorm:"type:varchar(64);not null;index:idx_rtc_inst" json:"instance_id"`
	RunSessionRef  string    `gorm:"type:varchar(64);not null;index:idx_rtc_session" json:"run_session_ref"`
	WindowStart    time.Time `json:"window_start"`
	WindowEnd      time.Time `json:"window_end"`
	QuotaType      string    `gorm:"type:varchar(16);not null" json:"quota_type"`
	ChargedMinutes int       `gorm:"not null;default:0" json:"charged_minutes"`
	// Reason 落账时生成的「具体原因」：含当时名额数 / 本台开机序 / 封顶值等，便于排查。
	Reason    string    `gorm:"type:varchar(255)" json:"reason"`
	CreatedAt time.Time `gorm:"autoCreateTime;index:idx_rtc_user" json:"created_at"`
}

func (RuntimeCharge) TableName() string { return "billing_runtime_charges" }

// RuntimeSessionProgress 某开机会话已结算到的整分钟数（结算幂等核心：避免重复扣）。
type RuntimeSessionProgress struct {
	RunSessionRef  string    `gorm:"primaryKey;type:varchar(64)" json:"run_session_ref"`
	UserID         uint      `gorm:"not null;index" json:"user_id"`
	InstanceID     string    `gorm:"type:varchar(64);not null" json:"instance_id"`
	SettledMinutes int       `gorm:"not null;default:0" json:"settled_minutes"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (RuntimeSessionProgress) TableName() string { return "billing_runtime_session_progress" }
