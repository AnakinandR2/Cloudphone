package billing

import (
	"fmt"
	"sort"
	"time"
)

// 运行计费引擎（新模型）：满 1 分钟取整 / 包月名额优先 / 临时时长回落 / 每台每天封顶。
// 纯逻辑核心 planRuntimeCharges 不依赖 DB / 时钟，便于穷举测试计费规则。

// runInstanceState 单台实例在本次结算的输入状态。
type runInstanceState struct {
	CpID          string
	RunSessionRef string
	PowerOn       time.Time
	// NewMinutes 本 tick 新产生、尚未结算的整分钟数（已满 1 分钟规则取整后）。
	NewMinutes int
	// DailyChargedBefore 该台当天(UTC+8)已扣临时分钟（封顶判定用）。
	DailyChargedBefore int
}

// runChargePlan 单台实例本 tick 的扣费拆分（一段开机会话内可能产生多种 quota_type）。
type runChargePlan struct {
	CpID          string
	RunSessionRef string
	BootSlotMin   int // 占包月名额（charged=0）
	TempMin       int // 扣临时时长
	CappedFreeMin int // 当天超封顶，免费
}

// planRuntimeCharges 决定本 tick 每台实例的扣费拆分。
//
//	states        当前运行中实例（含本 tick 待结算分钟数）
//	bootSlots     可用包月名额数
//	tempRemaining 临时时长余量（分钟）
//	dailyCap      每台每天封顶值（分钟，UTC+8）
//
// 规则：按 power_on 升序，前 bootSlots 台占名额（不扣时长）；其余台扣临时时长，
// 当天累计达 dailyCap 后剩余分钟免费（capped_free）；临时余量不足时仅扣余量能覆盖的分钟。
func planRuntimeCharges(states []runInstanceState, bootSlots int, tempRemaining int64, dailyCap int) []runChargePlan {
	ss := append([]runInstanceState(nil), states...)
	sort.SliceStable(ss, func(i, j int) bool {
		if ss[i].PowerOn.Equal(ss[j].PowerOn) {
			return ss[i].CpID < ss[j].CpID
		}
		return ss[i].PowerOn.Before(ss[j].PowerOn)
	})

	plans := make([]runChargePlan, 0, len(ss))
	remaining := tempRemaining
	for idx, s := range ss {
		p := runChargePlan{CpID: s.CpID, RunSessionRef: s.RunSessionRef}
		if s.NewMinutes <= 0 {
			plans = append(plans, p)
			continue
		}
		if idx < bootSlots {
			// 占包月名额，不扣临时时长。
			p.BootSlotMin = s.NewMinutes
			plans = append(plans, p)
			continue
		}
		// 回落到临时时长：先看当天封顶余量。
		mins := s.NewMinutes
		capLeft := dailyCap - s.DailyChargedBefore
		if capLeft < 0 {
			capLeft = 0
		}
		chargeable := mins
		if chargeable > capLeft {
			chargeable = capLeft
		}
		// 临时余量约束。
		if int64(chargeable) > remaining {
			chargeable = int(remaining)
		}
		p.TempMin = chargeable
		remaining -= int64(chargeable)
		// 当天封顶后剩余 → 免费继续运行。
		p.CappedFreeMin = mins - chargeable
		plans = append(plans, p)
	}
	return plans
}

// utc8Day 把时刻折算到 UTC+8 自然日字符串（封顶按 UTC+8 计）。
func utc8Day(t time.Time) string {
	loc := time.FixedZone("UTC+8", 8*3600)
	return t.In(loc).Format("2006-01-02")
}

// sessionRef 为缺失 RunSessionRef 的区间合成稳定标识（cpId@powerOnUnix）。
func sessionRef(cpID string, powerOn time.Time) string {
	if cpID == "" {
		cpID = "unknown"
	}
	return fmt.Sprintf("%s@%d", cpID, powerOn.Unix())
}
