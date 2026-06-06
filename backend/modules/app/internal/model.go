package app

import "time"

// 应用本地状态（中台异步创建：先「创建中」，应用就绪出现在 /app/page 后转「正常」）。
const (
	StatusCreating = "CREATING"
	StatusNormal   = "NORMAL"
)

// CustomerApp 是「上传应用 ↔ 本地绑定」。两类来源共用此表，用 Store 区分：
//   - Store=false：前台用户上传的「我的应用」，按 UserID 属主隔离；
//   - Store=true ：admin 上传的「应用商店」应用，面向全部用户，UserID=0。
//
// 应用库在中台是租户全局资源（无 owner），这张表让我们能按属主隔离、展示创建状态、
// 删除时校验归属（避免误删他人/全局应用），并把商店应用与用户上传区分开。
type CustomerApp struct {
	ID uint `gorm:"primaryKey" json:"id"`
	// 上传者前台用户 ID；应用商店(Store=true)由 admin 上传、无前台属主，UserID=0。
	UserID uint `gorm:"index;not null" json:"-"`
	// 应用商店标记：admin 上传 = true，面向全部用户；普通用户上传 = false。
	Store       bool      `gorm:"index;default:false" json:"store"`
	CpAppID     int64     `gorm:"index" json:"cpAppId"` // 中台 app_info 主键，删除/匹配用
	AppMD5      string    `gorm:"size:64;index" json:"appMd5"`
	AppName     string    `gorm:"size:255" json:"appName"`
	PackageName string    `gorm:"size:255" json:"packageName"`
	Version     string    `gorm:"size:64" json:"version"`
	FileSize    string    `gorm:"size:32" json:"fileSize"`
	IconPath    string    `gorm:"size:1024" json:"iconPath"`
	Status      string    `gorm:"size:16" json:"status"` // CREATING / NORMAL
	CreatedAt   time.Time `json:"createTime"`
	UpdatedAt   time.Time `json:"updateTime"`
}

func (CustomerApp) TableName() string { return "customer_apps" }
