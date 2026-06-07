package billing

import "time"

// 资源科目（统一流水 Subject 取值之一；与 SubjectBalance 并列）
const (
	SubjectInstanceSeat  = "instance_seat"  // 实例席位（容量型，台）
	SubjectBootSeat      = "boot_seat"      // 并发开机席位（容量型，台）
	SubjectRuntimeMinute = "runtime_minute" // 时长包余额（消耗型，分钟）
)

// 批次来源
const (
	SourceOrder  = "order"
	SourceGrant  = "grant"
	SourceAdjust = "adjust"
	SourceTrial  = "trial"
)

// resourceSubjects 合法的资源科目集合。
var resourceSubjects = map[string]bool{
	SubjectInstanceSeat: true, SubjectBootSeat: true, SubjectRuntimeMinute: true,
}

// EntitlementBatch 资源批次：一次发放的一份额度（带到期）。
// 可用容量(subject) = Σ(Quantity - Used) 于 未过期且 Quantity>Used 的批次。
type EntitlementBatch struct {
	ID        uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint       `gorm:"not null;index:idx_billing_ent_user_subject" json:"user_id"`
	Subject   string     `gorm:"type:varchar(20);not null;index:idx_billing_ent_user_subject" json:"subject"`
	Quantity  int64      `gorm:"not null" json:"quantity"`
	Used      int64      `gorm:"not null;default:0" json:"used"`
	ExpireAt  *time.Time `json:"expire_at"`
	Source    string     `gorm:"type:varchar(20)" json:"source"`
	SourceRef string     `gorm:"type:varchar(255)" json:"source_ref"`
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (EntitlementBatch) TableName() string { return "billing_entitlement_batches" }

// CapacitySnapshot 某用户三类资源的可用容量概览。
type CapacitySnapshot struct {
	InstanceSeat  int64 `json:"instance_seat"`
	BootSeat      int64 `json:"boot_seat"`
	RuntimeMinute int64 `json:"runtime_minute"`
}
