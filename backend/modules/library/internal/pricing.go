package library

import "math"

// 计价工具（§4.1 公式）。所有金额单位为分。

// roundDiv 四舍五入除法：round(n / d)，d>0。
func roundDiv(n float64, d float64) int64 {
	if d == 0 {
		return 0
	}
	return int64(math.Round(n / d))
}

// applyBps 按基点折扣计价：round(amount × bps / 10000)。bps=10000 即原价。
func applyBps(amount int64, bps int) int64 {
	if bps <= 0 {
		bps = DiscountBpsFull
	}
	return roundDiv(float64(amount)*float64(bps), float64(DiscountBpsFull))
}

// standardPrice 标准价（new / renew）：
//
//	总价 = round( monthly × (days/30) × tierDiscountBps/10000 × durationDiscountBps/10000 )
//
// 单位分，一次性四舍五入（先连乘再 round，避免逐步取整误差累积）。
func standardPrice(monthlyCents int64, days, tierBps, durBps int) int64 {
	if tierBps <= 0 {
		tierBps = DiscountBpsFull
	}
	if durBps <= 0 {
		durBps = DiscountBpsFull
	}
	v := float64(monthlyCents) *
		(float64(days) / 30.0) *
		(float64(tierBps) / float64(DiscountBpsFull)) *
		(float64(durBps) / float64(DiscountBpsFull))
	return int64(math.Round(v))
}

// upgradePrice 升级补差价（§4.1）：
//
//	总价 = round( remaining_days × (new_monthly − old_monthly) / 30 )
//
// 用原始月价、不叠折扣；差价为负时按 0（不退）。
func upgradePrice(remainingDays int, newMonthlyCents, oldMonthlyCents int64) int64 {
	diff := newMonthlyCents - oldMonthlyCents
	if diff <= 0 {
		return 0
	}
	return roundDiv(float64(remainingDays)*float64(diff), 30.0)
}
