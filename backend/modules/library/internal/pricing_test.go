package library

import (
	"testing"
	"time"
)

func TestStandardPrice_FullMonth_NoDiscount(t *testing.T) {
	// t100：月价 1800 分，30 天，无折扣 → 1800。
	got := standardPrice(1800, 30, 10000, 10000)
	if got != 1800 {
		t.Fatalf("want 1800, got %d", got)
	}
}

func TestStandardPrice_90Days_WithDurationDiscount(t *testing.T) {
	// 月价 1800，90 天（3 月），时长折扣 9500 → 1800*3*0.95 = 5130。
	got := standardPrice(1800, 90, 10000, 9500)
	if got != 5130 {
		t.Fatalf("want 5130, got %d", got)
	}
}

func TestStandardPrice_TierAndDurationDiscountStack(t *testing.T) {
	// 月价 8000，365 天，档位折扣 9000，时长折扣 8000。
	// 8000 * (365/30) * 0.9 * 0.8 = 8000*12.1667*0.72 = 70080.0 → round.
	got := standardPrice(8000, 365, 9000, 8000)
	// 精确：8000*365/30=97333.33; *0.9=87600; *0.8=70080
	if got != 70080 {
		t.Fatalf("want 70080, got %d", got)
	}
}

func TestStandardPrice_RoundsHalfUp(t *testing.T) {
	// 构造一个需要四舍五入的：月价 1000，45 天 → 1000*1.5=1500（整），换非整：
	// 月价 1000，10 天 → 1000*(10/30)=333.33 → round 333。
	got := standardPrice(1000, 10, 10000, 10000)
	if got != 333 {
		t.Fatalf("want 333, got %d", got)
	}
}

func TestUpgradePrice_Basic(t *testing.T) {
	// 剩余 30 天，新月价 1800，旧月价 1000 → 30*(800)/30 = 800。
	got := upgradePrice(30, 1800, 1000)
	if got != 800 {
		t.Fatalf("want 800, got %d", got)
	}
}

func TestUpgradePrice_RemainingDaysRounding(t *testing.T) {
	// 剩余 15 天，新月价 1800，旧月价 1000 → 15*800/30 = 400。
	got := upgradePrice(15, 1800, 1000)
	if got != 400 {
		t.Fatalf("want 400, got %d", got)
	}
}

func TestUpgradePrice_NegativeDiffIsZero(t *testing.T) {
	if got := upgradePrice(30, 1000, 1800); got != 0 {
		t.Fatalf("want 0 for non-positive diff, got %d", got)
	}
}

func TestRemainingDays_CeilBoundary(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	// 到期当天剩 1 小时 → ceil → 1 天。
	exp1 := now.Add(time.Hour)
	if got := remainingDays(now, &exp1); got != 1 {
		t.Fatalf("want 1, got %d", got)
	}
	// 剩 25 小时 → ceil(25/24) = 2 天。
	exp2 := now.Add(25 * time.Hour)
	if got := remainingDays(now, &exp2); got != 2 {
		t.Fatalf("want 2, got %d", got)
	}
	// 整 2 天 → ceil(48/24) = 2。
	exp3 := now.Add(48 * time.Hour)
	if got := remainingDays(now, &exp3); got != 2 {
		t.Fatalf("want 2, got %d", got)
	}
	// 已过期 → 0。
	exp4 := now.Add(-time.Hour)
	if got := remainingDays(now, &exp4); got != 0 {
		t.Fatalf("want 0, got %d", got)
	}
	// nil → 0。
	if got := remainingDays(now, nil); got != 0 {
		t.Fatalf("want 0 for nil, got %d", got)
	}
}
