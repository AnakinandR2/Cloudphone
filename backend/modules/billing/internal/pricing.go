package billing

import "manager-backend/framework/apperr"

// 计价引擎（新购买模型）。纯逻辑、不依赖 DB：服务端权威计价的核心。
//
// 折扣规则（已与需求方确认）：数量阶梯折扣 × 时长折扣，相乘。
// bps 语义沿用现有约定：DiscountBpsFull(10000)=全价无折扣，8500=8.5折。

// QtyTier 数量阶梯：数量 ≥ MinQuantity 时适用 DiscountBps。
type QtyTier struct {
	MinQuantity int
	DiscountBps int
}

// DurationOption 时长选项：席位按月、包月开机数按天；时长不可手输，必须命中某个选项。
type DurationOption struct {
	Value       int // 时长值（月数 / 天数）
	DiscountBps int
}

// PriceConfig 某一可购买资源(kind)的计价配置。
type PriceConfig struct {
	UnitPriceCents int64
	QtyTiers       []QtyTier
	DurationOpts   []DurationOption
}

// PriceQuote 一次计价结果（服务端权威，前端只展示）。
type PriceQuote struct {
	Quantity            int
	DurationValue       int
	UnitPriceCents      int64
	BillingUnits        int // 数量 × 时长值
	OriginalCents       int64
	QtyDiscountBps      int
	DurationDiscountBps int
	PayableCents        int64
}

func computeQuote(cfg PriceConfig, quantity, durationValue int) (PriceQuote, error) {
	if quantity < 1 {
		return PriceQuote{}, apperr.Validation("数量必须≥1")
	}
	if durationValue < 1 {
		return PriceQuote{}, apperr.Validation("时长必须≥1")
	}
	durationBps, err := resolveDurationBps(cfg.DurationOpts, durationValue)
	if err != nil {
		return PriceQuote{}, err
	}
	qtyBps := resolveQtyBps(cfg.QtyTiers, quantity)
	billingUnits := quantity * durationValue
	originalCents := cfg.UnitPriceCents * int64(billingUnits)
	payableCents := applyBps(applyBps(originalCents, qtyBps), durationBps)
	return PriceQuote{
		Quantity:            quantity,
		DurationValue:       durationValue,
		UnitPriceCents:      cfg.UnitPriceCents,
		BillingUnits:        billingUnits,
		OriginalCents:       originalCents,
		QtyDiscountBps:      qtyBps,
		DurationDiscountBps: durationBps,
		PayableCents:        payableCents,
	}, nil
}

// applyBps 按基点折扣计价，四舍五入到分。
func applyBps(cents int64, bps int) int64 {
	return (cents*int64(bps) + int64(DiscountBpsFull)/2) / int64(DiscountBpsFull)
}

// resolveQtyBps 取数量命中的最优阶梯（MinQuantity ≤ quantity 中门槛最高者）。无阶梯=全价。
func resolveQtyBps(tiers []QtyTier, quantity int) int {
	bps := DiscountBpsFull
	bestMin := -1
	for _, t := range tiers {
		if t.MinQuantity <= quantity && t.MinQuantity > bestMin {
			bestMin = t.MinQuantity
			bps = t.DiscountBps
		}
	}
	return bps
}

// resolveDurationBps 时长不可手输：配置了选项时必须精确命中某个 Value，否则报错。
// 未配置任何时长选项时（如临时时长包按数量计价）按全价处理。
func resolveDurationBps(opts []DurationOption, durationValue int) (int, error) {
	if len(opts) == 0 {
		return DiscountBpsFull, nil
	}
	for _, o := range opts {
		if o.Value == durationValue {
			return o.DiscountBps, nil
		}
	}
	return 0, apperr.Validation("非法的购买时长")
}
