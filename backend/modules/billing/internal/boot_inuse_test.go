package billing

import "testing"

// 包月开机数「在用」= 当前运行中的台数（由 phone 提供），但不超过持有的名额数。
func TestBootSlotInUse_MinOfRunningAndTotal(t *testing.T) {
	t.Cleanup(func() { SetRunningInstanceCountProvider(nil) })

	// 运行 1 台、持有 1 个名额 → 在用 1。
	SetRunningInstanceCountProvider(func(userID int) int { return 1 })
	if got := bootSlotInUse(42, 1); got != 1 {
		t.Errorf("running=1 total=1 → in_use=%d, want 1", got)
	}

	// 运行 3 台、只有 2 个名额 → 在用 2（其余靠临时时长，不计名额在用）。
	SetRunningInstanceCountProvider(func(userID int) int { return 3 })
	if got := bootSlotInUse(42, 2); got != 2 {
		t.Errorf("running=3 total=2 → in_use=%d, want 2", got)
	}

	// 没有运行中的台 → 在用 0。
	SetRunningInstanceCountProvider(func(userID int) int { return 0 })
	if got := bootSlotInUse(42, 2); got != 0 {
		t.Errorf("running=0 → in_use=%d, want 0", got)
	}
}

// 未注册 provider（如测试或 phone 未装配）时回退为 0，不 panic。
func TestBootSlotInUse_NilProviderZero(t *testing.T) {
	t.Cleanup(func() { SetRunningInstanceCountProvider(nil) })
	SetRunningInstanceCountProvider(nil)
	if got := bootSlotInUse(42, 5); got != 0 {
		t.Errorf("nil provider → in_use=%d, want 0", got)
	}
}
