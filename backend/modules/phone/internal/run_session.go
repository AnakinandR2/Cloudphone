package phone

import (
	"context"
	"time"

	"manager-backend/framework/midplat"
)

// RunSession 是从中台运行日志（§2.9）同步来的「开机→关机」运行会话。
// 运行中会话 PowerOffAt 为 NULL；按 LogNo 唯一去重，供计费结算与护栏使用。
type RunSession struct {
	ID                 uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	LogNo              string     `gorm:"type:varchar(64);uniqueIndex:uniq_run_sess_logno" json:"log_no"`
	CpID               string     `gorm:"type:varchar(64);index:idx_run_sess_cp" json:"cp_id"`
	UserID             uint       `gorm:"not null;index:idx_run_sess_user_on" json:"user_id"`
	VmUID              string     `gorm:"type:varchar(64)" json:"vm_uid"`
	PowerOnAt          time.Time  `gorm:"index:idx_run_sess_user_on" json:"power_on_at"`
	PowerOffAt         *time.Time `gorm:"index:idx_run_sess_off" json:"power_off_at"`
	SessionStatus      string     `gorm:"type:varchar(20)" json:"session_status"` // RUNNING / SHUTDOWN（中台码）
	PowerOffReasonCode string     `gorm:"type:varchar(30)" json:"power_off_reason_code"`
	SyncedAt           time.Time  `json:"synced_at"`
}

// 中台运行日志时间格式：yyyy-MM-dd HH:mm:ss（无时区，按本地时区解析）。
const runLogTimeLayout = "2006-01-02 15:04:05"

// parseRunLogTime 解析中台时间串；空 / 「运行中」/ 无法解析 → (零值, false)。
func parseRunLogTime(s string) (time.Time, bool) {
	if s == "" || s == "运行中" {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation(runLogTimeLayout, s, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// toRunSession 把一条中台日志映射为 RunSession（user 由调用方解析后传入）。
// 返回 ok=false 表示开机时间无法解析（脏数据）应跳过。
func toRunSession(e midplat.RunLogEntry, userID uint, now time.Time) (RunSession, bool) {
	on, ok := parseRunLogTime(e.PowerOnTime)
	if !ok {
		return RunSession{}, false
	}
	rs := RunSession{
		LogNo:              e.LogNo,
		CpID:               e.CpID,
		UserID:             userID,
		VmUID:              e.VmUID,
		PowerOnAt:          on,
		SessionStatus:      sessionStatusCode(e),
		PowerOffReasonCode: e.PowerOffReasonCode,
		SyncedAt:           now,
	}
	if off, ok := parseRunLogTime(e.PowerOffTime); ok {
		rs.PowerOffAt = &off
	}
	return rs, true
}

// sessionStatusCode 归一化会话状态为中台码（运行中→RUNNING，否则→SHUTDOWN）。
func sessionStatusCode(e midplat.RunLogEntry) string {
	if _, ok := parseRunLogTime(e.PowerOffTime); !ok {
		return "RUNNING"
	}
	return "SHUTDOWN"
}

// syncRunSessions 拉取中台运行日志并 upsert 到本地（§2.9 无时间窗，按 LogNo 幂等）。
// 策略：分页拉取，按 LogNo upsert；cp 不属于我方用户的丢弃；
// 单轮最多 maxPages 页，遇到「整页都是已知且已关机」即停（假设最新在前）。
func (s *serviceImpl) syncRunSessions(ctx context.Context) {
	if s.ops == nil {
		return
	}
	const (
		pageSize = 100
		maxPages = 50
	)
	now := time.Now()
	for page := 1; page <= maxPages; page++ {
		res, err := s.ops.RunLogs(ctx, "", page, pageSize)
		if err != nil || res == nil || len(res.Data) == 0 {
			return
		}
		// 解析本页涉及的 cpId → userId。
		cpIDs := make([]string, 0, len(res.Data))
		for _, e := range res.Data {
			cpIDs = append(cpIDs, e.CpID)
		}
		owners, err := s.repo.ownersByCpIDs(cpIDs)
		if err != nil {
			return
		}
		freshOrOpen := false
		for _, e := range res.Data {
			uid, ok := owners[e.CpID]
			if !ok {
				continue // 非我方用户（已删/转移），不入库
			}
			rs, ok := toRunSession(e, uid, now)
			if !ok {
				continue
			}
			isNew, err := s.repo.upsertRunSession(&rs)
			if err != nil {
				continue
			}
			// 新行或仍运行中 → 本页有变化，继续翻页。
			if isNew || rs.PowerOffAt == nil {
				freshOrOpen = true
			}
		}
		// 整页都是已知且已关机：后续页更旧，停。
		if !freshOrOpen {
			return
		}
		// 已到末页。
		if int64(page*pageSize) >= res.TotalSize {
			return
		}
	}
}
