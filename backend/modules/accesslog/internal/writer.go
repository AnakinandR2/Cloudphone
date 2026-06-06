package accesslog

import (
	"log"
	"time"
)

const (
	logChannelSize = 1024
	batchSize      = 50
	flushInterval  = 3 * time.Second
)

// logChan 中间件与异步写入器之间的缓冲队列。
var logChan chan AccessLog

// asyncWriter 批量落库：累积到 batchSize 或每 flushInterval 刷一次，channel 关闭时收尾。
func asyncWriter() {
	buf := make([]AccessLog, 0, batchSize)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case entry, ok := <-logChan:
			if !ok {
				if len(buf) > 0 {
					flushLogs(buf)
				}
				return
			}
			buf = append(buf, entry)
			if len(buf) >= batchSize {
				flushLogs(buf)
				buf = buf[:0]
			}
		case <-ticker.C:
			if len(buf) > 0 {
				flushLogs(buf)
				buf = buf[:0]
			}
		}
	}
}

func flushLogs(entries []AccessLog) {
	if err := logRepo.createBatch(entries); err != nil {
		log.Printf("[AccessLog] 批量写入失败 (%d 条): %v", len(entries), err)
	}
}
