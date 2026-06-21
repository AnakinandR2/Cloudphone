package billing

import (
	"testing"
	"time"
)

func st(cp string, on time.Time, newMin, dailyBefore int) runInstanceState {
	return runInstanceState{CpID: cp, RunSessionRef: cp + "-s", PowerOn: on, NewMinutes: newMin, DailyChargedBefore: dailyBefore}
}

func planByCp(plans []runChargePlan) map[string]runChargePlan {
	m := map[string]runChargePlan{}
	for _, p := range plans {
		m[p.CpID] = p
	}
	return m
}

func TestPlanRuntime_BootSlotPriorityByPowerOn(t *testing.T) {
	base := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	states := []runInstanceState{
		st("late", base.Add(2*time.Minute), 1, 0),
		st("early", base, 1, 0),
		st("mid", base.Add(time.Minute), 1, 0),
	}
	// 1 个包月名额 → 最早开机的 early 占名额，其余扣临时。
	plans := planRuntimeCharges(states, 1, 1000, 200)
	m := planByCp(plans)
	if m["early"].BootSlotMin != 1 || m["early"].TempMin != 0 {
		t.Errorf("early should occupy boot slot, got %+v", m["early"])
	}
	if m["mid"].TempMin != 1 || m["late"].TempMin != 1 {
		t.Errorf("mid/late should consume temp, got mid=%+v late=%+v", m["mid"], m["late"])
	}
}

func TestPlanRuntime_AllCoveredByBootSlots(t *testing.T) {
	base := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	states := []runInstanceState{st("a", base, 5, 0), st("b", base.Add(time.Minute), 5, 0)}
	plans := planRuntimeCharges(states, 5, 1000, 200)
	for _, p := range plans {
		if p.TempMin != 0 || p.BootSlotMin != 5 {
			t.Errorf("expected all boot_slot, got %+v", p)
		}
	}
}

func TestPlanRuntime_DailyCapThenFree(t *testing.T) {
	base := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	// 无包月名额，单台本 tick 想扣 10 分钟，但当天已扣 195，封顶 200 → 只能再扣 5，其余 5 免费。
	states := []runInstanceState{st("a", base, 10, 195)}
	plans := planRuntimeCharges(states, 0, 1000, 200)
	p := planByCp(plans)["a"]
	if p.TempMin != 5 {
		t.Errorf("TempMin = %d, want 5 (cap)", p.TempMin)
	}
	if p.CappedFreeMin != 5 {
		t.Errorf("CappedFreeMin = %d, want 5", p.CappedFreeMin)
	}
}

func TestPlanRuntime_AlreadyCappedAllFree(t *testing.T) {
	base := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	states := []runInstanceState{st("a", base, 3, 200)}
	plans := planRuntimeCharges(states, 0, 1000, 200)
	p := planByCp(plans)["a"]
	if p.TempMin != 0 || p.CappedFreeMin != 3 {
		t.Errorf("expected all capped_free, got %+v", p)
	}
}

func TestPlanRuntime_TempWalletExhausted(t *testing.T) {
	base := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	// 两台都要扣，但临时余量只有 3 分钟，按 power_on 顺序消耗。
	states := []runInstanceState{
		st("a", base, 2, 0),
		st("b", base.Add(time.Minute), 4, 0),
	}
	plans := planRuntimeCharges(states, 0, 3, 200)
	m := planByCp(plans)
	if m["a"].TempMin != 2 {
		t.Errorf("a TempMin = %d, want 2", m["a"].TempMin)
	}
	// 余量剩 1，b 只能扣 1。
	if m["b"].TempMin != 1 {
		t.Errorf("b TempMin = %d, want 1 (wallet exhausted)", m["b"].TempMin)
	}
}
