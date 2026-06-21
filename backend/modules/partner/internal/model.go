package partner

import "time"

// Partner 代理IP合作商（运营全局内容，非按用户隔离）。
// 运营在 admin 维护名称/logo/介绍/配图/推广链接/排序/启用；my、www 展示并引导点击推广链接。
type Partner struct {
	ID       uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Name     string `gorm:"type:varchar(100);not null" json:"name"`
	LogoURL  string `gorm:"type:varchar(512)" json:"logo_url"`
	ImageURL string `gorm:"type:varchar(512)" json:"image_url"` // 配图（单张）
	Intro    string `gorm:"type:text" json:"intro"`             // 纯文本介绍
	PromoURL string `gorm:"type:varchar(512);not null" json:"promo_url"`
	Sort     int    `gorm:"not null;default:0" json:"sort"`       // 排序权重，小在前
	// Enabled 是否启用。不设 gorm default：bool 默认值 false 配 default tag 会被 GORM 当零值忽略，
	// 导致「创建即禁用」写不进去；service 层始终显式赋值（留空默认 true）。
	Enabled bool `gorm:"not null" json:"enabled"`
	// ClickCount 点击总数缓存（明细见 partner_clicks），列表展示用；自增走原子 UPDATE。
	ClickCount int       `gorm:"not null;default:0" json:"click_count"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Partner) TableName() string { return "partners" }

// PartnerClick 一次推广链接点击的明细。匿名（未登录，含 www 游客）时 UserID 为 0。
// 软标识尽量多采集，缺失存空串；未知字段塞 Extra(JSON) 兜底，免于改表。
type PartnerClick struct {
	ID        uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	PartnerID uint   `gorm:"not null;index:idx_click_partner" json:"partner_id"`
	UserID    uint   `gorm:"not null;default:0;index:idx_click_user" json:"user_id"` // 0 表示匿名
	IP        string `gorm:"type:varchar(64)" json:"ip"`
	UserAgent string `gorm:"type:varchar(512)" json:"user_agent"`
	Referer   string `gorm:"type:varchar(512)" json:"referer"`
	// 前端持久化的访客标识 / 会话标识（软标识）。
	AnonymousID string `gorm:"type:varchar(64);index:idx_click_anon" json:"anonymous_id"`
	SessionID   string `gorm:"type:varchar(64)" json:"session_id"`
	// UTM 营销参数。
	UTMSource   string    `gorm:"type:varchar(128)" json:"utm_source"`
	UTMMedium   string    `gorm:"type:varchar(128)" json:"utm_medium"`
	UTMCampaign string    `gorm:"type:varchar(128)" json:"utm_campaign"`
	UTMTerm     string    `gorm:"type:varchar(128)" json:"utm_term"`
	UTMContent  string    `gorm:"type:varchar(128)" json:"utm_content"`
	Channel     string    `gorm:"type:varchar(64);index:idx_click_channel" json:"channel"` // my / www / 落地页标识
	Extra       string    `gorm:"type:text" json:"extra"`                                  // JSON 兜底
	CreatedAt   time.Time `gorm:"autoCreateTime;index:idx_click_created" json:"created_at"`
}

func (PartnerClick) TableName() string { return "partner_clicks" }

// PartnerCreate 创建请求（admin）。
type PartnerCreate struct {
	Name     string `json:"name" binding:"required" example:"亮数代理"`
	LogoURL  string `json:"logo_url"`
	ImageURL string `json:"image_url"`
	Intro    string `json:"intro"`
	PromoURL string `json:"promo_url" binding:"required" example:"https://partner.example.com/?ref=us"`
	Sort     int    `json:"sort"`
	Enabled  *bool  `json:"enabled"` // 指针：留空默认启用
}

// PartnerUpdate 更新请求（admin）。字段语义：除 Enabled/Sort 外留空表示不更新。
type PartnerUpdate struct {
	Name     string  `json:"name"`
	LogoURL  *string `json:"logo_url"`
	ImageURL *string `json:"image_url"`
	Intro    *string `json:"intro"`
	PromoURL string  `json:"promo_url"`
	Sort     *int    `json:"sort"`
	Enabled  *bool   `json:"enabled"`
}

// ClickRequest 点击上报请求（公开，my/www）。软标识由前端传，IP/UA/Referer 由服务端取。
type ClickRequest struct {
	AnonymousID string `json:"anonymous_id"`
	SessionID   string `json:"session_id"`
	UTMSource   string `json:"utm_source"`
	UTMMedium   string `json:"utm_medium"`
	UTMCampaign string `json:"utm_campaign"`
	UTMTerm     string `json:"utm_term"`
	UTMContent  string `json:"utm_content"`
	Channel     string `json:"channel"`
	Extra       string `json:"extra"`
}

// PublicPartner my/www 展示用的精简视图（不含点击数等运营字段）。
type PublicPartner struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	LogoURL  string `json:"logo_url"`
	ImageURL string `json:"image_url"`
	Intro    string `json:"intro"`
	PromoURL string `json:"promo_url"`
}

func toPublic(p Partner) PublicPartner {
	return PublicPartner{
		ID: p.ID, Name: p.Name, LogoURL: p.LogoURL, ImageURL: p.ImageURL,
		Intro: p.Intro, PromoURL: p.PromoURL,
	}
}
