package accesslog

import (
	"sync"
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OnStart→OnStop 的优雅关闭路径应把通道里剩余日志 flush 落库。
func TestOnStopFlushesPendingLogs(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("access_logs") })

	// 重置全局，保证与其他用例的执行顺序无关。
	stopOnce = sync.Once{}
	logChan = make(chan AccessLog, 10)

	m := &accessLogModule{}
	require.NoError(t, m.OnStart())

	logChan <- AccessLog{Method: "GET", Path: "/grace", CreatedAt: time.Now()}
	require.NoError(t, m.OnStop()) // 关闭通道并等待写入器 flush

	assert.Equal(t, int64(1), countLogs())
}

func countLogs() int64 {
	var n int64
	framework.DB.Model(&AccessLog{}).Count(&n)
	return n
}

func TestRepoCreateBatchAndDeleteOlderThan(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("access_logs") })
	now := time.Now()
	require.NoError(t, logRepo.createBatch([]AccessLog{
		{Method: "GET", Path: "/a", CreatedAt: now.AddDate(0, 0, -40)},
		{Method: "GET", Path: "/b", CreatedAt: now.AddDate(0, 0, -40)},
		{Method: "GET", Path: "/c", CreatedAt: now},
	}))
	assert.Equal(t, int64(3), countLogs())

	affected, err := logRepo.deleteOlderThan(now.AddDate(0, 0, -30), 100)
	require.NoError(t, err)
	assert.Equal(t, int64(2), affected)
	assert.Equal(t, int64(1), countLogs())
}

func TestRepoCreateBatchEmptyIsNoop(t *testing.T) {
	require.NoError(t, logRepo.createBatch(nil))
}

func TestFlushLogs(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("access_logs") })
	flushLogs([]AccessLog{{Method: "POST", Path: "/x", CreatedAt: time.Now()}})
	assert.Equal(t, int64(1), countLogs())
}

func TestCleanOldLogs(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("access_logs") })
	now := time.Now()
	require.NoError(t, logRepo.createBatch([]AccessLog{
		{Method: "GET", Path: "/old", CreatedAt: now.AddDate(0, 0, -(retentionDays + 1))},
		{Method: "GET", Path: "/new", CreatedAt: now},
	}))
	cleanOldLogs()
	assert.Equal(t, int64(1), countLogs(), "应只删除超出保留期的日志")
}

// asyncWriter 在 channel 关闭时应把缓冲区剩余日志落库。
func TestAsyncWriterFlushesOnClose(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("access_logs") })

	logChan = make(chan AccessLog, 10)
	done := make(chan struct{})
	go func() { asyncWriter(); close(done) }()

	logChan <- AccessLog{Method: "GET", Path: "/async", CreatedAt: time.Now()}
	close(logChan)
	<-done

	assert.Equal(t, int64(1), countLogs())
}

// asyncWriter 累积到 batchSize 即自动刷盘。
func TestAsyncWriterFlushesOnBatchSize(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("access_logs") })

	logChan = make(chan AccessLog, batchSize+10)
	done := make(chan struct{})
	go func() { asyncWriter(); close(done) }()

	for i := 0; i < batchSize; i++ {
		logChan <- AccessLog{Method: "GET", Path: "/b", CreatedAt: time.Now()}
	}
	close(logChan)
	<-done

	assert.Equal(t, int64(batchSize), countLogs())
}
