package billing

import "time"

// 时长费相关流水类型。
const (
	LedgerRuntimeMinute  = "runtime_minute"  // 扣减时长包分钟（subject=runtime_minute）
	LedgerRuntimeBalance = "runtime_balance" // 扣减钱包余额抵时长费（subject=balance）
)

// BillingRuntimeConfig 时长费单行配置（id 固定 1，启动时幂等 seed）。
type BillingRuntimeConfig struct {
	ID                      uint      `gorm:"primaryKey" json:"id"`
	UnitPriceCentsPerMinute int64     `gorm:"not null;default:0" json:"unit_price_cents_per_minute"` // 元/台/分钟（分）
	LowBalanceAlertCents    int64     `gorm:"not null;default:0" json:"low_balance_alert_cents"`     // 低余额告警阈值（分）
	UpdatedAt               time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// RuntimeUsageSlice 一次结算一用户的扣费切片（用量页展示 + 对账）。
type RuntimeUsageSlice struct {
	ID                  uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID              uint      `gorm:"not null;index:idx_rt_slice_user" json:"user_id"`
	WindowStart         time.Time `json:"window_start"`
	WindowEnd           time.Time `json:"window_end"`
	BillableUnitMinutes int64     `json:"billable_unit_minutes"` // 席位覆盖后的台·分钟
	CoveredSeatMinutes  int64     `json:"covered_seat_minutes"`  // 被开机席位覆盖（免费）的台·分钟
	ChargedPackMinutes  int64     `json:"charged_pack_minutes"`  // 从时长包扣的分钟
	ChargedBalanceCents int64     `json:"charged_balance_cents"` // 从钱包扣的金额
	UnfundedMinutes     int64     `json:"unfunded_minutes"`      // 余额不足未能覆盖的台·分钟
	UnitPriceCents      int64     `json:"unit_price_cents"`
	CreatedAt           time.Time `gorm:"autoCreateTime;index:idx_rt_slice_user" json:"created_at"`
}

// RuntimeSettlementWatermark 每用户已结算到的时刻（结算幂等核心，分钟对齐）。
type RuntimeSettlementWatermark struct {
	UserID      uint      `gorm:"primaryKey" json:"user_id"`
	SettlededAt time.Time `gorm:"column:settled_until" json:"settled_until"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// RuntimeInterval 一段运行区间（End=nil 表示运行中，以窗口右界为准）。供 SettleRuntime 入参。
// CpID / RunSessionRef 由新计费引擎使用（旧引擎只读 Start/End）；为空时引擎按 Start 合成会话标识。
type RuntimeInterval struct {
	Start         time.Time
	End           *time.Time
	CpID          string // 实例 cpId（新引擎用于按台封顶/落账）
	RunSessionRef string // 开机会话标识（新引擎用于幂等结算）
}

// RuntimeCoverage 给护栏读的覆盖能力快照。
type RuntimeCoverage struct {
	AvailableBootSeats   int64 `json:"available_boot_seats"`
	RemainingPackMinutes int64 `json:"remaining_pack_minutes"`
	BalanceCents         int64 `json:"balance_cents"`
	UnitPriceCents       int64 `json:"unit_price_cents"`
}

// SettleResult 一次结算的扣费结果。
// 旧引擎填 BillableUnitMinutes/ChargedPackMinutes/ChargedBalanceCents/UnfundedMinutes；
// 新引擎填 ChargedTempMinutes/BootSlotMinutes/CappedFreeMinutes（按 quota_type 汇总本次结算扣量）。
type SettleResult struct {
	WindowStart         time.Time
	WindowEnd           time.Time
	BillableUnitMinutes int64
	ChargedPackMinutes  int64
	ChargedBalanceCents int64
	UnfundedMinutes     int64

	// 新引擎汇总：
	ChargedTempMinutes int64
	BootSlotMinutes    int64
	CappedFreeMinutes  int64
}
