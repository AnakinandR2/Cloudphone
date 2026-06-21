package billing

import (
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

	page, err := RuntimeEngineService.RuntimeLog(uid, 1, 20, map[string]bool{"sl": true})
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
