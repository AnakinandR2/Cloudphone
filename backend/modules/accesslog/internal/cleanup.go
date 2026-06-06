package accesslog

import (
	"log"
	"time"
)

const (
	cleanupInterval = 24 * time.Hour
	retentionDays   = 30
	cleanupBatchDel = 10000
)

// cleanupLoop 启动即清理一次，之后每 cleanupInterval 清理过期日志。
func cleanupLoop() {
	cleanOldLogs()
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		cleanOldLogs()
	}
}

func cleanOldLogs() {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	for {
		affected, err := logRepo.deleteOlderThan(cutoff, cleanupBatchDel)
		if err != nil {
			log.Printf("[AccessLog] 清理失败: %v", err)
			break
		}
		if affected == 0 {
			break
		}
		log.Printf("[AccessLog] 清理了 %d 条过期日志", affected)
	}
}
