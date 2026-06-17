// Package billing 是计费业务模块的公开入口。
//
// 模块实现位于 modules/billing/internal，受 Go internal 机制保护；公开包仅用于在 main 中
// blank-import 以触发自注册。如需对外发布契约（供其他模块依赖），在此再导出。
package billing

import (
	"time"

	billinginternal "manager-backend/modules/billing/internal"

	"gorm.io/gorm"
)

// 跨模块门面（供 phone 等调用；billing 不反向依赖任何业务模块）。

// EnforcementTarget 欠费执行目标。
type EnforcementTarget = billinginternal.EnforcementTarget

func TryOccupyInstanceSeat(userID int) error {
	return billinginternal.SeatService.TryOccupyInstanceSeat(userID)
}
func ReleaseInstanceSeat(userID int) error {
	return billinginternal.SeatService.ReleaseInstanceSeat(userID)
}
func ReconcileInstanceSeats(userID, count int) error {
	return billinginternal.SeatService.ReconcileInstanceSeats(userID, count)
}
func IsFrozen(userID int) (bool, error) { return billinginternal.SeatService.IsFrozen(userID) }
func InstanceSeatCapacity(userID int) (int64, error) {
	return billinginternal.SeatService.InstanceSeatCapacity(userID)
}
func ListDunningEnforcement() ([]billinginternal.EnforcementTarget, error) {
	return billinginternal.SeatService.ListDunningEnforcement()
}

// 欠费状态常量（供 phone 等跨模块引用）。
const (
	DunningFrozen   = billinginternal.DunningFrozen
	DunningRecycled = billinginternal.DunningRecycled
)

// ---- 时长费计量（Metering，供 phone 结算编排 + 护栏调用）----

// RuntimeInterval 一段运行区间（End=nil 表示运行中）。
type RuntimeInterval = billinginternal.RuntimeInterval

// RuntimeCoverage 覆盖能力快照。
type RuntimeCoverage = billinginternal.RuntimeCoverage

// SettleResult 一次结算扣费结果。
type SettleResult = billinginternal.SettleResult

// SettleRuntime 结算某用户已发生的运行分钟（覆盖优先级扣费，幂等）。
func SettleRuntime(userID int, windowEnd time.Time, intervals []RuntimeInterval) (SettleResult, error) {
	return billinginternal.RuntimeService.SettleRuntime(userID, windowEnd, intervals)
}

// GetRuntimeCoverage 读用户覆盖能力（护栏用）。
func GetRuntimeCoverage(userID int) (RuntimeCoverage, error) {
	return billinginternal.RuntimeService.RuntimeCoverage(userID)
}

// InitForTest 测试用：供其他模块装配 billing。
func InitForTest(db *gorm.DB) error { return billinginternal.InitForTest(db) }

// SetDunningForTest 测试用：直接置某用户欠费状态。
func SetDunningForTest(userID int, state string) error {
	return billinginternal.SetDunningForTest(userID, state)
}

// GrantInstanceSeatsForTest 测试用：给用户发放 n 个实例席位。
func GrantInstanceSeatsForTest(userID, n int) error {
	_, err := billinginternal.EntitlementService.Grant(userID, billinginternal.SubjectInstanceSeat, int64(n), nil, billinginternal.SourceAdjust, "test", billinginternal.LedgerAdjustGrant, "staff:1")
	return err
}

// GrantBootSeatsForTest 测试用：给用户发放 n 个开机席位（并发免时长费）。
func GrantBootSeatsForTest(userID, n int) error {
	_, err := billinginternal.EntitlementService.Grant(userID, billinginternal.SubjectBootSeat, int64(n), nil, billinginternal.SourceAdjust, "test", billinginternal.LedgerAdjustGrant, "staff:1")
	return err
}

// GrantRuntimeMinutesForTest 测试用：给用户发放 n 分钟时长包。
func GrantRuntimeMinutesForTest(userID, n int) error {
	_, err := billinginternal.EntitlementService.Grant(userID, billinginternal.SubjectRuntimeMinute, int64(n), nil, billinginternal.SourceAdjust, "test", billinginternal.LedgerAdjustGrant, "staff:1")
	return err
}

// SetRuntimeUnitPriceForTest 测试用：设置时长费单价（分/台/分钟），>0 表示启用时长费。
func SetRuntimeUnitPriceForTest(cents int64) error {
	_, err := billinginternal.RuntimeService.SaveConfig(cents, 0)
	return err
}
