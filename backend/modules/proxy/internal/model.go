package proxy

import "time"

// 代理健康状态枚举。
const (
	StatusUnknown = "unknown" // 未检测
	StatusOK      = "ok"      // 探测通过
	StatusFail    = "fail"    // 探测失败/超时
)

// Proxy 用户自有的 SOCKS5 代理（业务层资源，中台不存代理池）。按属主隔离：每条归属一个前台用户。
type Proxy struct {
	ID       uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID   uint   `gorm:"not null;index:idx_proxy_user" json:"user_id"`
	Name     string `gorm:"type:varchar(100);not null" json:"name"`
	Protocol string `gorm:"type:varchar(20);not null;default:'socks5'" json:"protocol"`
	Host     string `gorm:"type:varchar(255);not null" json:"host"`
	Port     int    `gorm:"not null" json:"port"`
	Username string `gorm:"type:varchar(100)" json:"username"`
	// Password 敏感字段：入库保存，但 json:"-" 确保绝不出现在任何响应里。
	Password string `gorm:"type:varchar(255)" json:"-"`
	Region   string `gorm:"type:varchar(100)" json:"region"`
	// 健康检查结果：由「测试代理」经 SOCKS5 实测写入。
	Status        string     `gorm:"type:varchar(20);not null;default:'unknown'" json:"status"`
	Latency       int        `gorm:"not null;default:0" json:"latency"` // 毫秒
	EgressIP      string     `gorm:"type:varchar(64)" json:"egress_ip"` // 经代理访问得到的真实出口 IP
	LastCheckedAt *time.Time `json:"last_checked_at"`
	// 出口 IP 归属（由测试时调 ipvibe 自动识别）。
	Country   string    `gorm:"type:varchar(64)" json:"country"`
	City      string    `gorm:"type:varchar(64)" json:"city"`
	ASN       string    `gorm:"type:varchar(32)" json:"asn"`       // 如 AS21859
	ASNName   string    `gorm:"type:varchar(128)" json:"asn_name"` // 如 Zenlayer Inc
	Company   string    `gorm:"type:varchar(128)" json:"company"`  // 如 Zenlayer IP Block @Hong Kong
	ConnType  string    `gorm:"type:varchar(32)" json:"conn_type"` // 如 Corporate
	Remark    string    `gorm:"type:varchar(255)" json:"remark"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Proxy) TableName() string { return "proxies" }

// ProxyCreate 创建请求（前台用户为自己新增代理）。
type ProxyCreate struct {
	Name     string `json:"name" binding:"required" example:"美西代理1"`
	Protocol string `json:"protocol" example:"socks5"`
	Host     string `json:"host" binding:"required" example:"104.21.45.7"`
	Port     int    `json:"port" binding:"required" example:"1080"`
	Username string `json:"username" example:"u1"`
	Password string `json:"password" example:"secret"`
	Region   string `json:"region" example:"美国·洛杉矶"`
	Remark   string `json:"remark"`
}

// ProxyProbeRequest 即时探测请求（添加/编辑表单里测试用，按参数探测、不落库）。
type ProxyProbeRequest struct {
	Host     string `json:"host" binding:"required" example:"104.21.45.7"`
	Port     int    `json:"port" binding:"required" example:"1080"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// ProbeOutcome 一次即时探测的结果（不落库，直接返回给前端展示）。
type ProbeOutcome struct {
	Status   string `json:"status"` // ok / fail
	Latency  int    `json:"latency"`
	EgressIP string `json:"egress_ip"`
	Country  string `json:"country"`
	City     string `json:"city"`
	ASN      string `json:"asn"`
	ASNName  string `json:"asn_name"`
	Company  string `json:"company"`
	ConnType string `json:"conn_type"`
	Message  string `json:"message"` // 失败原因
}

// ProxyBatch 批量导入请求（前端已把每行解析成结构化条目）。
type ProxyBatch struct {
	Proxies []ProxyCreate `json:"proxies"`
}

// ProxyUpdate 更新请求（字段留空表示不更新）。
type ProxyUpdate struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Region   string `json:"region"`
	Remark   string `json:"remark"`
}
