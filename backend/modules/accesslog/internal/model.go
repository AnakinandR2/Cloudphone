package accesslog

import "time"

// AccessLog 接口访问日志模型
type AccessLog struct {
	ID     uint `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID uint `gorm:"not null;default:0;index:idx_user_created,priority:1" json:"user_id"`
	// Scope 行为主体身份域：staff（后台）/ user（前台）/ anonymous（未登录）。
	// 因 UserID 在不同身份域间会重号，需配合 Scope 才能唯一定位行为主体。
	Scope           string    `gorm:"type:varchar(20);not null;default:'';index:idx_scope" json:"scope"`
	Username        string    `gorm:"type:varchar(50);not null;default:''" json:"username"`
	Method          string    `gorm:"type:varchar(10);not null;default:''" json:"method"`
	Path            string    `gorm:"type:varchar(500);not null;default:''" json:"path"`
	StatusCode      int       `gorm:"not null;default:0" json:"status_code"`
	LatencyMs       int       `gorm:"not null;default:0" json:"latency_ms"`
	ClientIP        string    `gorm:"type:varchar(45);not null;default:''" json:"client_ip"`
	UserAgent       string    `gorm:"type:varchar(500);not null;default:''" json:"user_agent"`
	RequestHeaders  string    `gorm:"type:text" json:"request_headers"`
	RequestBody     string    `gorm:"type:text" json:"request_body"`
	ResponseHeaders string    `gorm:"type:text" json:"response_headers"`
	ResponseBody    string    `gorm:"type:text" json:"response_body"`
	CreatedAt       time.Time `gorm:"not null;index:idx_created_at;index:idx_user_created,priority:2" json:"created_at"`
}

func (AccessLog) TableName() string { return "access_logs" }

// AccessLogListItem 列表响应（不含 headers 和 body，减少传输量）
type AccessLogListItem struct {
	ID         uint      `json:"id"`
	UserID     uint      `json:"user_id"`
	Scope      string    `json:"scope"`
	Username   string    `json:"username"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	StatusCode int       `json:"status_code"`
	LatencyMs  int       `json:"latency_ms"`
	ClientIP   string    `json:"client_ip"`
	CreatedAt  time.Time `json:"created_at"`
}
