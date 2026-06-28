package app

import "time"

// 解析状态：上传后旁挂元数据的服务端解析进度（finalize/上传时落定）。
const (
	ParseStatusParsing = "parsing"
	ParseStatusReady   = "ready"
	ParseStatusFailed  = "failed"
)

// AppUserMeta 是「用户应用」素材库文件的旁挂元数据，与 library 文件一对一
// （PK = library_files.id）。属主与配额由 library 保证，这里只存解析所得的
// 应用元信息（名称/版本/包名/图标/MD5/解析状态），供列表展示与按 URL 安装组载荷。
// 删除素材库文件即应用消失，本行随 file_id 一并清理。
type AppUserMeta struct {
	LibraryFileID uint   `gorm:"primaryKey"`     // = library_files.id
	UserID        uint   `gorm:"index;not null"` // 上传者（与 library 文件属主一致）
	PackageName   string `gorm:"size:255;index"`
	Version       string `gorm:"size:64"`
	AppName       string `gorm:"size:255"`  // 解析所得，可与文件名不同
	IconURL       string `gorm:"size:1024"` // 公有桶图标 URL
	MD5           string `gorm:"size:64;index"`
	ParseStatus   string `gorm:"size:16"` // parsing|ready|failed
	ParseError    string `gorm:"size:512"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (AppUserMeta) TableName() string { return "app_user_meta" }

// AppMarket 是应用市场应用：平台资产，二进制存平台公有桶，不计任何用户配额。
// admin 经后端 multipart 上传 → 解析 → PutObject → 落本行；用户端只读浏览。
type AppMarket struct {
	ID          uint   `gorm:"primaryKey"`
	S3Key       string `gorm:"size:512;uniqueIndex"` // 公有桶对象键
	AppName     string `gorm:"size:255"`
	PackageName string `gorm:"size:255;index"`
	Version     string `gorm:"size:64"`
	IconURL     string `gorm:"size:1024"`
	MD5         string `gorm:"size:64"`
	FileSize    int64  `gorm:"not null;default:0"`
	ParseStatus string `gorm:"size:16"` // parsing|ready|failed
	ParseError  string `gorm:"size:512"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (AppMarket) TableName() string { return "app_market" }
