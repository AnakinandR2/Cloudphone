package phone

import (
	"testing"
	"time"

	"manager-backend/framework"
	"manager-backend/framework/midplat"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToRunSession(t *testing.T) {
	now := time.Now()
	// 已关机会话。
	rs, ok := toRunSession(midplat.RunLogEntry{
		LogNo: "LOG1", CpID: "cp-a", PowerOnTime: "2026-06-13 09:00:00",
		PowerOffTime: "2026-06-13 10:00:00", PowerOffReasonCode: "SHUTDOWN",
	}, 7, now)
	require.True(t, ok)
	assert.Equal(t, "SHUTDOWN", rs.SessionStatus)
	require.NotNil(t, rs.PowerOffAt)
	assert.Equal(t, uint(7), rs.UserID)

	// 运行中会话：powerOffTime=运行中 → PowerOffAt nil、状态 RUNNING。
	rs2, ok := toRunSession(midplat.RunLogEntry{
		LogNo: "LOG2", CpID: "cp-a", PowerOnTime: "2026-06-13 11:00:00", PowerOffTime: "运行中",
	}, 7, now)
	require.True(t, ok)
	assert.Nil(t, rs2.PowerOffAt)
	assert.Equal(t, "RUNNING", rs2.SessionStatus)

	// 开机时间无法解析 → 跳过。
	_, ok = toRunSession(midplat.RunLogEntry{LogNo: "LOG3", PowerOnTime: ""}, 7, now)
	assert.False(t, ok)
}

func TestUpsertRunSessionIdempotent(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("run_sessions") })
	repo := newRepository(framework.DB)
	off := time.Date(2026, 6, 13, 10, 0, 0, 0, time.Local)

	// 首次插入（运行中）。
	rs := &RunSession{LogNo: "LOGX", CpID: "cp-x", UserID: 9, PowerOnAt: off.Add(-time.Hour), SessionStatus: "RUNNING"}
	isNew, err := repo.upsertRunSession(rs)
	require.NoError(t, err)
	assert.True(t, isNew)

	// 同 LogNo 再来（已关机）→ 非新行，补齐关机时间。
	rs2 := &RunSession{LogNo: "LOGX", CpID: "cp-x", UserID: 9, PowerOnAt: off.Add(-time.Hour), PowerOffAt: &off, SessionStatus: "SHUTDOWN"}
	isNew, err = repo.upsertRunSession(rs2)
	require.NoError(t, err)
	assert.False(t, isNew)

	running, err := repo.runningSessions()
	require.NoError(t, err)
	assert.Empty(t, running, "已关机后不应再算运行中")
}

func TestOwnersByCpIDs(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	repo := newRepository(framework.DB)
	require.NoError(t, framework.DB.Create(&CloudPhone{UserID: 11, Name: "a", CpID: "cp-own", Status: StatusRunning}).Error)

	m, err := repo.ownersByCpIDs([]string{"cp-own", "cp-missing"})
	require.NoError(t, err)
	assert.Equal(t, uint(11), m["cp-own"])
	_, ok := m["cp-missing"]
	assert.False(t, ok)
}
