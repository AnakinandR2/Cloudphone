// Package billing 是计费业务模块的公开入口。
//
// 模块实现位于 modules/billing/internal，受 Go internal 机制保护；公开包仅用于在 main 中
// blank-import 以触发自注册。如需对外发布契约（供其他模块依赖），在此再导出。
package billing

import billinginternal "manager-backend/modules/billing/internal"

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
