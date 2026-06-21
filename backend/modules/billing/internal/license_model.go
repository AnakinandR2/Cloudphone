package billing

import "time"

// LicenseUnit 授权单元：把「云手机实例席位」和「包月开机数」统一为同一概念，用 Kind 区分。
//
// 一行 = 一个可购买的授权单元，有独立 ID、各自的创建/到期时间，可单独或批量续费。
// 同 (UserID, Kind) 且未过期的单元构成「可用池」：席位池决定能创建多少实例，
// 包月开机数池决定能并发开机多少台。实例由 reconcile 自动分配到单元（current_instance_id 物化）。
type LicenseUnit struct {
	ID                uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID            uint      `gorm:"not null;index:idx_license_user_kind,priority:1" json:"user_id"`
	Kind              string    `gorm:"type:varchar(16);not null;index:idx_license_user_kind,priority:2" json:"kind"` // seat / boot_slot
	Status            string    `gorm:"type:varchar(16);not null;default:active" json:"status"`                       // active / expired
	CurrentInstanceID string    `gorm:"type:varchar(64);index" json:"current_instance_id"`                            // 当前坐在该单元上的实例(cpId)，空=空闲；由 reconcile 物化
	Source            string    `gorm:"type:varchar(16);not null" json:"source"`                                      // order / trial / grant
	SourceRef         string    `gorm:"type:varchar(64)" json:"source_ref"`                                           // 订单号 / 试用码 / staff:<id>
	OrderItemID       uint      `gorm:"default:0;index" json:"order_item_id"`
	ExpireAt          time.Time `gorm:"not null;index" json:"expire_at"`
	CreatedAt         time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (LicenseUnit) TableName() string { return "billing_license_units" }

// 授权单元类型。
const (
	KindSeat     = "seat"      // 云手机实例席位：决定「能存在」多少台实例
	KindBootSlot = "boot_slot" // 包月开机数：决定「能并发运行」多少台实例
)

// 授权单元状态。
const (
	LicenseActive  = "active"
	LicenseExpired = "expired"
)

// 授权单元来源（发放入口）复用 entitlement_model.go 的 SourceOrder/SourceTrial/SourceGrant。
