package billing

import (
	"time"

	"manager-backend/framework"
)

// 欠费保护 cron 默认参数。
const (
	dunningInterval   = time.Hour
	dunningLease      = 10 * time.Minute
	defaultGraceDays  = 3
	defaultFrozenDays = 7
)

var dunningRunner *framework.PeriodicRunner
