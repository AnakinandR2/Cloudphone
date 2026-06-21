package billing

import (
	"testing"
	"time"
)

// planSeatAssignment 纯逻辑：给定「未过期席位」与「非回收实例」，决定谁坐哪个席位、谁溢出回收。
// 规则：席位按到期时间倒序（最长命的先分给最老的实例，保证热迁移稳定）；
// 实例按创建时间正序；席位不足时，最新创建的实例溢出回收。

func ts(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func TestPlanSeatAssignment_EqualCounts_AllPairedNoRecycle(t *testing.T) {
	seats := []seatRef{{ID: 1, ExpireAt: ts("2026-08-01")}, {ID: 2, ExpireAt: ts("2026-09-01")}}
	instances := []instanceRef{{CpID: "a", CreatedAt: ts("2026-01-01")}, {CpID: "b", CreatedAt: ts("2026-02-01")}}
	plan := planSeatAssignment(seats, instances)
	if len(plan.Recycle) != 0 {
		t.Errorf("Recycle = %v, want empty", plan.Recycle)
	}
	if len(plan.Pairs) != 2 {
		t.Fatalf("Pairs len = %d, want 2", len(plan.Pairs))
	}
}

func TestPlanSeatAssignment_OldestInstanceGetsLongestSeat(t *testing.T) {
	// 席位2到期更晚 → 应分给最老的实例 a，实现热迁移稳定。
	seats := []seatRef{{ID: 1, ExpireAt: ts("2026-08-01")}, {ID: 2, ExpireAt: ts("2026-12-01")}}
	instances := []instanceRef{{CpID: "a", CreatedAt: ts("2026-01-01")}, {CpID: "b", CreatedAt: ts("2026-02-01")}}
	plan := planSeatAssignment(seats, instances)
	got := map[uint]string{}
	for _, p := range plan.Pairs {
		got[p.SeatID] = p.InstanceID
	}
	if got[2] != "a" || got[1] != "b" {
		t.Errorf("assignment = %v, want seat2->a, seat1->b", got)
	}
}

func TestPlanSeatAssignment_MoreSeatsThanInstances_ExtraSeatsIdle(t *testing.T) {
	seats := []seatRef{{ID: 1, ExpireAt: ts("2026-08-01")}, {ID: 2, ExpireAt: ts("2026-09-01")}, {ID: 3, ExpireAt: ts("2026-10-01")}}
	instances := []instanceRef{{CpID: "a", CreatedAt: ts("2026-01-01")}}
	plan := planSeatAssignment(seats, instances)
	if len(plan.Recycle) != 0 {
		t.Errorf("Recycle = %v, want empty", plan.Recycle)
	}
	if len(plan.Pairs) != 1 {
		t.Errorf("Pairs len = %d, want 1 (only one instance to seat)", len(plan.Pairs))
	}
}

func TestPlanSeatAssignment_FewerSeats_NewestInstancesRecycled(t *testing.T) {
	seats := []seatRef{{ID: 1, ExpireAt: ts("2026-08-01")}}
	instances := []instanceRef{
		{CpID: "a", CreatedAt: ts("2026-01-01")},
		{CpID: "b", CreatedAt: ts("2026-02-01")},
		{CpID: "c", CreatedAt: ts("2026-03-01")},
	}
	plan := planSeatAssignment(seats, instances)
	// 只有1个席位 → 最老的 a 保留，b、c（较新）溢出回收。
	if len(plan.Pairs) != 1 || plan.Pairs[0].InstanceID != "a" {
		t.Errorf("Pairs = %v, want only a seated", plan.Pairs)
	}
	if len(plan.Recycle) != 2 {
		t.Fatalf("Recycle = %v, want [b c]", plan.Recycle)
	}
	rec := map[string]bool{plan.Recycle[0]: true, plan.Recycle[1]: true}
	if !rec["b"] || !rec["c"] {
		t.Errorf("Recycle = %v, want b and c", plan.Recycle)
	}
}

func TestPlanSeatAssignment_NoSeats_AllRecycled(t *testing.T) {
	instances := []instanceRef{{CpID: "a", CreatedAt: ts("2026-01-01")}, {CpID: "b", CreatedAt: ts("2026-02-01")}}
	plan := planSeatAssignment(nil, instances)
	if len(plan.Recycle) != 2 {
		t.Errorf("Recycle = %v, want both recycled", plan.Recycle)
	}
}
