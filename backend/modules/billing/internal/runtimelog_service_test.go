package billing

import (
	"strconv"
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeLog_AggregatesMixedSessionIntoSegments(t *testing.T) {
	t.Cleanup(cleanRuntime)
	uid := 960001
	require.NoError(t, FulfillService.FulfillRuntimePack(uid, 1000, SourceOrder, "seed"))
	require.NoError(t, FulfillService.FulfillNew(uid, KindBootSlot, 1, 30, SourceOrder, "boot"))

	base := time.Date(2026, 6, 21, 8, 0, 0, 0, time.UTC)
	// 两台：early 占名额（boot_slot），late 扣临时。聚合后 late 会话 temp_minutes_charged>0。
	ivs := []RuntimeInterval{
		{Start: base, CpID: "early", RunSessionRef: "se"},
		{Start: base.Add(time.Minute), CpID: "late", RunSessionRef: "sl"},
	}
	_, err := RuntimeEngineService.Settle(uid, base.Add(6*time.Minute), ivs)
	require.NoError(t, err)

	page, err := RuntimeEngineService.RuntimeLog(uid, 1, 20, map[string]bool{"sl": true}, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(2), page.Total)
	assert.Equal(t, 200, page.DailyCapMinutes)

	byCp := map[string]RuntimeLogItem{}
	for _, it := range page.Items {
		byCp[it.CpID] = it
	}
	// late 会话扣了临时时长，且标记为运行中。
	require.Contains(t, byCp, "late")
	assert.True(t, byCp["late"].TempMinutesCharged > 0)
	assert.True(t, byCp["late"].Running)
	assert.Equal(t, QuotaTemp, byCp["late"].QuotaType)
	// early 会话占名额，temp=0。
	assert.Equal(t, 0, byCp["early"].TempMinutesCharged)
	assert.Equal(t, QuotaBootSlot, byCp["early"].QuotaType)
	_ = framework.DB
}

// 周期结算会为同一会话陆续插入多条同 quota_type 的分钟级 charge；费用日志的分段
// 明细应把「连续同类型」合并为一段，而不是每条 charge 一行。
func TestRuntimeLog_CoalescesConsecutiveSameQuotaSegments(t *testing.T) {
	t.Cleanup(cleanRuntime)
	uid := 960002
	require.NoError(t, FulfillService.FulfillNew(uid, KindBootSlot, 1, 30, SourceOrder, "boot"))

	base := time.Date(2026, 6, 21, 8, 0, 0, 0, time.UTC)
	// 模拟周期结算：同一台、同一会话，水位逐步推进，产生多条 boot_slot charge。
	ivs := []RuntimeInterval{{Start: base, CpID: "cp1", RunSessionRef: "s1"}}
	for i := 1; i <= 8; i++ {
		_, err := RuntimeEngineService.Settle(uid, base.Add(time.Duration(i)*time.Minute), ivs)
		require.NoError(t, err)
	}

	page, err := RuntimeEngineService.RuntimeLog(uid, 1, 20, map[string]bool{"s1": true}, nil, nil)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	it := page.Items[0]
	// 全程包月：分段应合并为「一段」，而非 8 段。
	require.Len(t, it.Segments, 1)
	assert.Equal(t, QuotaBootSlot, it.Segments[0].QuotaType)
	assert.True(t, it.Segments[0].From.Equal(base))
	assert.True(t, it.Segments[0].To.Equal(base.Add(8*time.Minute)))
}

// 每段都带「具体原因」：包月段点名当前名额数；临时段说明超额/无名额；封顶段点名封顶值。
func TestRuntimeLog_SegmentReasonIsSpecific_BootSlot(t *testing.T) {
	t.Cleanup(cleanRuntime)
	uid := 960005
	require.NoError(t, FulfillService.FulfillNew(uid, KindBootSlot, 2, 30, SourceOrder, "boot"))
	base := time.Date(2026, 6, 22, 8, 0, 0, 0, time.UTC)
	_, err := RuntimeEngineService.Settle(uid, base.Add(3*time.Minute),
		[]RuntimeInterval{{Start: base, CpID: "cp1", RunSessionRef: "s1"}})
	require.NoError(t, err)

	pg, err := RuntimeEngineService.RuntimeLog(uid, 1, 20, nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, pg.Items, 1)
	require.Len(t, pg.Items[0].Segments, 1)
	r := pg.Items[0].Segments[0].Reason
	assert.Contains(t, r, "包月名额")
	assert.Contains(t, r, "2 个", "应点名当前名额数 2：%s", r)
}

func TestRuntimeLog_SegmentReasonIsSpecific_NoAllowance(t *testing.T) {
	t.Cleanup(cleanRuntime)
	uid := 960006
	require.NoError(t, FulfillService.FulfillRuntimePack(uid, 100, SourceOrder, "rt"))
	base := time.Date(2026, 6, 22, 8, 0, 0, 0, time.UTC)
	_, err := RuntimeEngineService.Settle(uid, base.Add(3*time.Minute),
		[]RuntimeInterval{{Start: base, CpID: "cp1", RunSessionRef: "s1"}})
	require.NoError(t, err)

	pg, err := RuntimeEngineService.RuntimeLog(uid, 1, 20, nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, pg.Items, 1)
	require.GreaterOrEqual(t, len(pg.Items[0].Segments), 1)
	assert.Contains(t, pg.Items[0].Segments[0].Reason, "无可用包月名额")
}

// 同一开机会话的各 charge 时间窗口必须首尾相接、互不重叠：单 tick 内若同时产生
// 临时段与封顶免费段（撞当日封顶），两段窗口不能共用同一区间，否则会重复计 1 分钟。
func TestRuntimeLog_ChargeWindowsTileWithoutOverlap(t *testing.T) {
	t.Cleanup(cleanRuntime)
	uid := 960004
	require.NoError(t, FulfillService.FulfillRuntimePack(uid, 1000, SourceOrder, "rt"))

	base := time.Date(2026, 6, 21, 8, 0, 0, 0, time.UTC)
	cp := "cpX"
	cap, err := PricingConfigService.DailyCapMinutes()
	require.NoError(t, err)
	// 预置当天已扣到「封顶 - 2」：本 tick 结算 5 分钟 → 2 分钟临时 + 3 分钟封顶免费。
	require.NoError(t, RuntimeEngineService.rtWallet.addDaily(uid, cp, utc8Day(base), cap-2))

	// 无包月名额，该台超额走临时。
	_, err = RuntimeEngineService.Settle(uid, base.Add(5*time.Minute),
		[]RuntimeInterval{{Start: base, CpID: cp, RunSessionRef: "sX"}})
	require.NoError(t, err)

	pg, err := RuntimeEngineService.RuntimeLog(uid, 1, 20, nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, pg.Items, 1)
	segs := pg.Items[0].Segments
	require.GreaterOrEqual(t, len(segs), 2)
	// 段按时间排序后，必须首尾相接、不重叠，且整体覆盖 [base, base+5]。
	for i := 1; i < len(segs); i++ {
		assert.Falsef(t, segs[i].From.Before(segs[i-1].To),
			"段 %d 起点 %v 早于上一段终点 %v（窗口重叠）", i, segs[i].From, segs[i-1].To)
	}
	assert.True(t, segs[0].From.Equal(base))
	assert.True(t, segs[len(segs)-1].To.Equal(base.Add(5*time.Minute)))
	// 临时段恰好 2 分钟。
	assert.Equal(t, 2, pg.Items[0].TempMinutesCharged)
	// 封顶免费段的原因点名封顶值。
	for _, sg := range segs {
		if sg.QuotaType == QuotaCappedFree {
			assert.Contains(t, sg.Reason, "封顶")
			assert.Contains(t, sg.Reason, strconv.Itoa(cap))
		}
	}
}

// 费用日志支持按时间段筛选：只返回与 [from, to] 有重叠的开机会话。
func TestRuntimeLog_FiltersByTimeRange(t *testing.T) {
	t.Cleanup(cleanRuntime)
	uid := 960003
	require.NoError(t, FulfillService.FulfillNew(uid, KindBootSlot, 5, 30, SourceOrder, "boot"))

	morning := time.Date(2026, 6, 21, 8, 0, 0, 0, time.UTC)
	evening := time.Date(2026, 6, 21, 20, 0, 0, 0, time.UTC)
	_, err := RuntimeEngineService.Settle(uid, morning.Add(3*time.Minute),
		[]RuntimeInterval{{Start: morning, CpID: "cpA", RunSessionRef: "sA"}})
	require.NoError(t, err)
	_, err = RuntimeEngineService.Settle(uid, evening.Add(3*time.Minute),
		[]RuntimeInterval{{Start: evening, CpID: "cpB", RunSessionRef: "sB"}})
	require.NoError(t, err)

	// 不过滤：两条都在。
	all, err := RuntimeEngineService.RuntimeLog(uid, 1, 20, nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(2), all.Total)

	// from=19:00 → 只剩晚场 cpB。
	from := time.Date(2026, 6, 21, 19, 0, 0, 0, time.UTC)
	pg1, err := RuntimeEngineService.RuntimeLog(uid, 1, 20, nil, &from, nil)
	require.NoError(t, err)
	require.Len(t, pg1.Items, 1)
	assert.Equal(t, "cpB", pg1.Items[0].CpID)
	assert.Equal(t, int64(1), pg1.Total)

	// to=12:00 → 只剩早场 cpA。
	to := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	pg2, err := RuntimeEngineService.RuntimeLog(uid, 1, 20, nil, nil, &to)
	require.NoError(t, err)
	require.Len(t, pg2.Items, 1)
	assert.Equal(t, "cpA", pg2.Items[0].CpID)

	// 两侧都给、窗口内无会话 → 空。
	g1 := time.Date(2026, 6, 21, 13, 0, 0, 0, time.UTC)
	g2 := time.Date(2026, 6, 21, 14, 0, 0, 0, time.UTC)
	pg3, err := RuntimeEngineService.RuntimeLog(uid, 1, 20, nil, &g1, &g2)
	require.NoError(t, err)
	assert.Equal(t, int64(0), pg3.Total)
	assert.Len(t, pg3.Items, 0)
}
