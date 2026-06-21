package billing

import "time"

// RuntimeInterval 一段运行区间（End=nil 表示运行中，以窗口右界为准）。供 SettleRuntime 入参。
// CpID / RunSessionRef 由计费引擎使用于按台封顶/幂等结算；为空时引擎按 Start 合成会话标识。
type RuntimeInterval struct {
	Start         time.Time
	End           *time.Time
	CpID          string // 实例 cpId（引擎用于按台封顶/落账）
	RunSessionRef string // 开机会话标识（引擎用于幂等结算）
}

// SettleResult 一次结算的扣费结果（新引擎按 quota_type 汇总本次结算扣量）。
type SettleResult struct {
	WindowStart        time.Time
	WindowEnd          time.Time
	ChargedTempMinutes int64
	BootSlotMinutes    int64
	CappedFreeMinutes  int64
}
