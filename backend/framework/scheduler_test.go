package framework

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	tdb, _ := SetupTestDB(m)
	code := m.Run()
	tdb.Teardown()
	os.Exit(code)
}

func TestTryRunLockedSingleRunnerThenFailover(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&CronLock{}))
	t.Cleanup(func() { CleanTable("cron_locks") })

	runs := 0
	job := func() error { runs++; return nil }

	ran, err := TryRunLocked(DB, "t-job", 200*time.Millisecond, job)
	require.NoError(t, err)
	assert.True(t, ran)
	assert.Equal(t, 1, runs)

	ran, err = TryRunLocked(DB, "t-job", 200*time.Millisecond, job)
	require.NoError(t, err)
	assert.False(t, ran)
	assert.Equal(t, 1, runs)

	time.Sleep(220 * time.Millisecond)
	ran, err = TryRunLocked(DB, "t-job", 200*time.Millisecond, job)
	require.NoError(t, err)
	assert.True(t, ran)
	assert.Equal(t, 2, runs)
}

func TestTryRunLockedFnErrorPropagates(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&CronLock{}))
	t.Cleanup(func() { CleanTable("cron_locks") })
	ran, err := TryRunLocked(DB, "t-err", time.Minute, func() error { return assert.AnError })
	assert.True(t, ran)
	assert.Error(t, err)
}
