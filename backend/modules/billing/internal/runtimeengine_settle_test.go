package billing

import (
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 端到端结算（DB）：满 1 分钟取整、幂等不重扣、临时时长扣减、200 封顶累计、包月名额优先回落。

func cleanRuntime() {
	framework.CleanTable("billing_runtime_charges")
	framework.CleanTable("billing_runtime_session_progress")
	framework.CleanTable("billing_runtime_minute_wallets")
	framework.CleanTable("billing_runtime_daily_usage")
	framework.CleanTable("billing_license_units")
}

func TestSettle_TempConsumeAndIdempotent(t *testing.T) {
	t.Cleanup(cleanRuntime)
	uid := 940001
	require.NoError(t, FulfillService.FulfillRuntimePack(uid, 1000, SourceOrder, "seed"))

	on := time.Date(2026, 6, 21, 10, 0, 32, 0, time.UTC)
	// 结算到 10:03:50 → 累计 (10:03:00-10:00:32)=约 2 分钟（floor）。实际 floor((10:03:50-10:00:32)/60)=3? 截断到分钟窗口右界 10:03:00。
	windowEnd := time.Date(2026, 6, 21, 10, 3, 50, 0, time.UTC)
	ivs := []RuntimeInterval{{Start: on, CpID: "cp1", RunSessionRef: "sess1"}}
	res, err := RuntimeEngineService.Settle(uid, windowEnd, ivs)
	require.NoError(t, err)
	// floor((10:03:00 - 10:00:32)/60s) = floor(148/60)=2 分钟。
	assert.Equal(t, int64(2), res.ChargedTempMinutes)

	rem, _ := RuntimeWalletService.Remaining(uid)
	assert.Equal(t, int64(998), rem)

	// 再次用同一窗口结算 → 幂等，不重复扣。
	res2, err := RuntimeEngineService.Settle(uid, windowEnd, ivs)
	require.NoError(t, err)
	assert.Equal(t, int64(0), res2.ChargedTempMinutes)
	rem, _ = RuntimeWalletService.Remaining(uid)
	assert.Equal(t, int64(998), rem)
}

func TestSettle_BootSlotPriorityNoTempCharge(t *testing.T) {
	t.Cleanup(cleanRuntime)
	uid := 940002
	require.NoError(t, FulfillService.FulfillRuntimePack(uid, 1000, SourceOrder, "seed"))
	require.NoError(t, FulfillService.FulfillNew(uid, KindBootSlot, 1, 30, SourceOrder, "boot")) // 1 个包月名额

	base := time.Date(2026, 6, 21, 9, 0, 0, 0, time.UTC)
	ivs := []RuntimeInterval{
		{Start: base, CpID: "early", RunSessionRef: "e"},                 // 最早 → 占名额
		{Start: base.Add(time.Minute), CpID: "late", RunSessionRef: "l"}, // → 扣临时
	}
	windowEnd := base.Add(10 * time.Minute)
	res, err := RuntimeEngineService.Settle(uid, windowEnd, ivs)
	require.NoError(t, err)
	assert.Equal(t, int64(10), res.BootSlotMinutes, "early occupies boot slot")
	assert.Equal(t, int64(9), res.ChargedTempMinutes, "late ran 9 min, temp charged")
	rem, _ := RuntimeWalletService.Remaining(uid)
	assert.Equal(t, int64(991), rem)
}

func TestSettle_DailyCapThenFree(t *testing.T) {
	t.Cleanup(cleanRuntime)
	uid := 940003
	require.NoError(t, FulfillService.FulfillRuntimePack(uid, 1000, SourceOrder, "seed"))
	// 预置当天已扣 195 分钟（UTC+8 当天）。
	on := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC) // UTC+8 = 08:00 当天
	day := utc8Day(on)
	require.NoError(t, RuntimeWalletService.repo.addDaily(uid, "cp1", day, 195))

	ivs := []RuntimeInterval{{Start: on, CpID: "cp1", RunSessionRef: "s"}}
	windowEnd := on.Add(10 * time.Minute) // 想扣 10 分钟，但封顶只剩 5。
	res, err := RuntimeEngineService.Settle(uid, windowEnd, ivs)
	require.NoError(t, err)
	assert.Equal(t, int64(5), res.ChargedTempMinutes, "only 5 left to cap")
	assert.Equal(t, int64(5), res.CappedFreeMinutes, "rest free")
}

func TestCanBoot(t *testing.T) {
	t.Cleanup(cleanRuntime)
	uid := 940004
	ok, err := RuntimeEngineService.CanBoot(uid)
	require.NoError(t, err)
	assert.False(t, ok, "no slot/no minutes → cannot boot")

	require.NoError(t, FulfillService.FulfillRuntimePack(uid, 10, SourceOrder, "seed"))
	ok, _ = RuntimeEngineService.CanBoot(uid)
	assert.True(t, ok, "has temp minutes → can boot")
}
