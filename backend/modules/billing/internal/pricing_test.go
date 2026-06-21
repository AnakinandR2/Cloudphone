package billing

import "testing"

// 计价引擎（新模型）：originalCents = 单价 × 数量 × 时长值；
// 数量阶梯折扣 × 时长折扣（相乘）。纯逻辑，不依赖 DB。

func TestComputeQuote_NoDiscount(t *testing.T) {
	cfg := PriceConfig{UnitPriceCents: 3000}
	q, err := computeQuote(cfg, 2, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.BillingUnits != 6 {
		t.Errorf("BillingUnits = %d, want 6", q.BillingUnits)
	}
	if q.OriginalCents != 18000 {
		t.Errorf("OriginalCents = %d, want 18000", q.OriginalCents)
	}
	if q.QtyDiscountBps != DiscountBpsFull || q.DurationDiscountBps != DiscountBpsFull {
		t.Errorf("discounts = (%d,%d), want full (%d,%d)", q.QtyDiscountBps, q.DurationDiscountBps, DiscountBpsFull, DiscountBpsFull)
	}
	if q.PayableCents != 18000 {
		t.Errorf("PayableCents = %d, want 18000", q.PayableCents)
	}
}

func TestComputeQuote_QtyTierPicksHighestThresholdMet(t *testing.T) {
	cfg := PriceConfig{
		UnitPriceCents: 1000,
		QtyTiers: []QtyTier{
			{MinQuantity: 1, DiscountBps: 10000},
			{MinQuantity: 10, DiscountBps: 9000},
			{MinQuantity: 100, DiscountBps: 8000},
		},
	}
	// 数量 50：命中 MinQuantity=10（9折），不命中 100。
	q, err := computeQuote(cfg, 50, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.QtyDiscountBps != 9000 {
		t.Errorf("QtyDiscountBps = %d, want 9000", q.QtyDiscountBps)
	}
	// 1000 × 50 × 0.9 = 45000
	if q.PayableCents != 45000 {
		t.Errorf("PayableCents = %d, want 45000", q.PayableCents)
	}
}

func TestComputeQuote_DurationDiscountApplies(t *testing.T) {
	cfg := PriceConfig{
		UnitPriceCents: 1000,
		DurationOpts: []DurationOption{
			{Value: 1, DiscountBps: 10000},
			{Value: 12, DiscountBps: 7000},
		},
	}
	q, err := computeQuote(cfg, 1, 12)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.DurationDiscountBps != 7000 {
		t.Errorf("DurationDiscountBps = %d, want 7000", q.DurationDiscountBps)
	}
	// 1000 × 12 × 0.7 = 8400
	if q.PayableCents != 8400 {
		t.Errorf("PayableCents = %d, want 8400", q.PayableCents)
	}
}

func TestComputeQuote_DiscountsMultiply(t *testing.T) {
	cfg := PriceConfig{
		UnitPriceCents: 1000,
		QtyTiers:       []QtyTier{{MinQuantity: 1, DiscountBps: 10000}, {MinQuantity: 10, DiscountBps: 9000}},
		DurationOpts:   []DurationOption{{Value: 12, DiscountBps: 8000}},
	}
	// 数量10(9折) × 时长12(8折)：1000 × 10 × 12 × 0.9 × 0.8 = 86400
	q, err := computeQuote(cfg, 10, 12)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.PayableCents != 86400 {
		t.Errorf("PayableCents = %d, want 86400", q.PayableCents)
	}
}

func TestComputeQuote_DurationMustMatchOptionWhenConfigured(t *testing.T) {
	cfg := PriceConfig{
		UnitPriceCents: 1000,
		DurationOpts:   []DurationOption{{Value: 1, DiscountBps: 10000}, {Value: 12, DiscountBps: 7000}},
	}
	// 时长不可手输：7 不在选项里 → 报错。
	if _, err := computeQuote(cfg, 1, 7); err == nil {
		t.Fatalf("expected error for unlisted duration value, got nil")
	}
}

func TestComputeQuote_RejectsNonPositiveInputs(t *testing.T) {
	cfg := PriceConfig{UnitPriceCents: 1000}
	if _, err := computeQuote(cfg, 0, 1); err == nil {
		t.Errorf("expected error for quantity 0")
	}
	if _, err := computeQuote(cfg, 1, 0); err == nil {
		t.Errorf("expected error for durationValue 0")
	}
}
