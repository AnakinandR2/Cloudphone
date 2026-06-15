package billing

import (
	"testing"
	"time"
)

func tAt(h, m int) time.Time { return time.Date(2026, 6, 13, h, m, 0, 0, time.Local) }

// TestSettleRuntimeCoverageAndIdempotency 覆盖优先级扣费 + 水位幂等。
func TestSettleRuntimeCoverageAndIdempotency(t *testing.T) {
	const uid = 920001
	// 1 个开机席位、1 分钟时长包、余额 1000 分、单价 10 分/分钟。
	if _, err := EntitlementService.Grant(uid, SubjectBootSeat, 1, nil, SourceAdjust, "t", LedgerAdjustGrant, "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := EntitlementService.Grant(uid, SubjectRuntimeMinute, 1, nil, SourceAdjust, "t", LedgerAdjustGrant, "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := BillingService.Topup(uid, 1000, "t", "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := RuntimeService.SaveConfig(10, 0); err != nil {
		t.Fatal(err)
	}

	t0 := tAt(10, 0)
	off := tAt(10, 3)
	ivs := []RuntimeInterval{{Start: t0, End: &off}, {Start: t0, End: &off}} // 两台并发 [10:00,10:03)

	// 首次：只落水位、不计费。
	r0, err := RuntimeService.SettleRuntime(uid, t0, ivs)
	if err != nil || r0.BillableUnitMinutes != 0 {
		t.Fatalf("首次应不计费: %+v err=%v", r0, err)
	}
	// 推进到 10:03：2 台并发 1 席位 → 每分钟 1 计费 × 3 = 3 台·分钟。
	r1, err := RuntimeService.SettleRuntime(uid, tAt(10, 3), ivs)
	if err != nil {
		t.Fatal(err)
	}
	if r1.BillableUnitMinutes != 3 || r1.ChargedPackMinutes != 1 || r1.ChargedBalanceCents != 20 || r1.UnfundedMinutes != 0 {
		t.Fatalf("覆盖优先级错: %+v (期望 billable=3 pack=1 balance=20)", r1)
	}
	// 重跑同窗口：水位已到 10:03，幂等不再扣。
	r2, err := RuntimeService.SettleRuntime(uid, tAt(10, 3), ivs)
	if err != nil || r2.BillableUnitMinutes != 0 {
		t.Fatalf("幂等失败: %+v err=%v", r2, err)
	}
	// 余额应只扣 20 分（1000→980）。
	acc, _ := BillingService.GetAccount(uid)
	if acc.BalanceCents != 980 {
		t.Fatalf("余额应为 980，实际 %d", acc.BalanceCents)
	}
}

func TestMeterMinutes(t *testing.T) {
	on1003 := tAt(10, 3)
	on1002 := tAt(10, 2)

	cases := []struct {
		name             string
		intervals        []RuntimeInterval
		seat             int64
		start, end       time.Time
		wantBill, wantCv int64
	}{
		{"单台无席位=按分钟计费", []RuntimeInterval{{Start: tAt(10, 0), End: &on1003}}, 0, tAt(10, 0), tAt(10, 3), 3, 0},
		{"单台1席位=全覆盖免费", []RuntimeInterval{{Start: tAt(10, 0), End: &on1003}}, 1, tAt(10, 0), tAt(10, 3), 0, 3},
		{
			"两台并发1席位=每分钟1免1计",
			[]RuntimeInterval{{Start: tAt(10, 0), End: &on1003}, {Start: tAt(10, 1), End: &on1003}},
			1, tAt(10, 0), tAt(10, 3), 2, 3,
		},
		{"运行中(End=nil)按窗口右界", []RuntimeInterval{{Start: tAt(10, 0), End: nil}}, 0, tAt(10, 0), tAt(10, 2), 2, 0},
		{"空窗口", []RuntimeInterval{{Start: tAt(10, 0), End: &on1002}}, 0, tAt(10, 3), tAt(10, 3), 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bill, cv := meterMinutes(c.intervals, c.seat, c.start, c.end)
			if bill != c.wantBill || cv != c.wantCv {
				t.Fatalf("billable=%d covered=%d, want billable=%d covered=%d", bill, cv, c.wantBill, c.wantCv)
			}
		})
	}
}
