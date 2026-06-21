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

// SettleRuntime 结算某用户已发生的运行分钟（新引擎：满1分钟取整/包月名额优先/200封顶/临时时长回落，幂等）。
func SettleRuntime(userID int, windowEnd time.Time, intervals []RuntimeInterval) (SettleResult, error) {
	return billinginternal.RuntimeEngineService.Settle(userID, windowEnd, intervals)
}

// GetRuntimeCoverage 读用户覆盖能力（旧护栏用，切换期保留）。
func GetRuntimeCoverage(userID int) (RuntimeCoverage, error) {
	return billinginternal.RuntimeService.RuntimeCoverage(userID)
}

// ---- 新购买/费用模型门面（契约 §3）----

// InstanceRef 跨模块传入的实例引用（cpId + 创建时间）。
type InstanceRef = billinginternal.InstanceRef

// InstanceMeta 实例展示信息（名称/状态），供续费列表/费用日志富化。
type InstanceMeta = billinginternal.InstanceMeta

// SetInstanceMetaProvider 注册实例展示信息提供者（phone 装配时调用）。
// billing 仅持有函数指针，无对 phone 的编译期依赖（依赖反转，遵守 Modulith 边界）。
func SetInstanceMetaProvider(fn func(cpIDs []string) map[string]InstanceMeta) {
	billinginternal.SetInstanceMetaProvider(fn)
}

// SeatCapacity 未过期 seat 授权单元数。
func SeatCapacity(userID int) (int, error) {
	return billinginternal.LicenseService.Capacity(userID, billinginternal.KindSeat)
}

// BootSlotCapacity 未过期 boot_slot 授权单元数。
func BootSlotCapacity(userID int) (int, error) {
	return billinginternal.LicenseService.Capacity(userID, billinginternal.KindBootSlot)
}

// RuntimeMinutesRemaining 临时开机时长余量（分钟）。
func RuntimeMinutesRemaining(userID int) (int64, error) {
	return billinginternal.RuntimeWalletService.Remaining(userID)
}

// ReconcileSeats 席位池 reconcile：传入用户全部非回收实例，返回需进回收站的 cpId（最新溢出）。
func ReconcileSeats(userID int, instances []InstanceRef) (recycle []string, err error) {
	return billinginternal.LicenseService.ReconcileSeats(userID, instances)
}

// CanBoot 开机前置校验：有空闲包月名额或临时时长>0 才允许。
func CanBoot(userID int) (bool, error) {
	return billinginternal.RuntimeEngineService.CanBoot(userID)
}

// ReleaseInstanceOccupancy 实例被删除/回收时释放其在 seat 单元上的占用。
func ReleaseInstanceOccupancy(userID int, cpIDs []string) error {
	return billinginternal.RuntimeEngineService.ReleaseInstanceOccupancy(userID, cpIDs)
}

// RecycleRetentionDays 回收站保留天数（admin 定价配置 recycle_retention_days，默认 30）。
func RecycleRetentionDays() (int, error) {
	return billinginternal.PricingConfigService.RecycleRetentionDays()
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

// GrantSeatLicensesForTest 测试用：给用户发放 n 个 seat 授权单元（新模型 license_units，30 天到期）。
func GrantSeatLicensesForTest(userID, n int) error {
	return billinginternal.FulfillService.FulfillNew(userID, billinginternal.KindSeat, n, 30, billinginternal.SourceGrant, "test")
}

// GrantBootSlotLicensesForTest 测试用：给用户发放 n 个 boot_slot 授权单元（新模型 license_units，30 天到期）。
func GrantBootSlotLicensesForTest(userID, n int) error {
	return billinginternal.FulfillService.FulfillNew(userID, billinginternal.KindBootSlot, n, 30, billinginternal.SourceGrant, "test")
}

// GrantRuntimeMinutesWalletForTest 测试用：给用户的新临时时长钱包加 n 分钟（CanBoot/结算用）。
func GrantRuntimeMinutesWalletForTest(userID, n int) error {
	return billinginternal.FulfillService.FulfillRuntimePack(userID, n, billinginternal.SourceGrant, "test")
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
