package framework

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F1 回归：周期任务 fn panic 时 runTick 必须 recover，goroutine/进程存活，下一轮仍执行。
func TestPeriodicRunner_TickRecoversPanic(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&CronLock{}))
	const name = "test-cron-recover"
	t.Cleanup(func() { DB.Where("name = ?", name).Delete(&CronLock{}) })

	calls := 0
	p := NewPeriodicRunner(DB, name, time.Hour, time.Minute, func() error {
		calls++
		panic("boom")
	})

	require.NotPanics(t, func() { p.runTick() }, "runTick 应 recover fn 的 panic")
	// 释放租约锁模拟下一 tick 可再抢，验证 recover 后 fn 仍会被执行。
	DB.Where("name = ?", name).Delete(&CronLock{})
	require.NotPanics(t, func() { p.runTick() }, "recover 后应能继续下一轮")
	assert.Equal(t, 2, calls, "两轮 fn 都应被调用（panic 被 recover，进程未崩）")
}
