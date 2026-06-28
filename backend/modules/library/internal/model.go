package library

import "time"

// 素材库（网盘 + 容量套餐）数据模型。所有表用 library_ 前缀。
// sqlite 下索引名全局唯一，本模块索引统一以 idx_library_ / uq_library_ 前缀避免与他表重名。

// GiB 容量单位：1 GB = 1024^3 字节（GiB）。capacity_bytes = capacity_gb × GiB。
const GiB int64 = 1024 * 1024 * 1024

// DiscountBpsFull 折扣基点满值（10000 = 原价无折扣）。
const DiscountBpsFull = 10000

// 文件状态。
const (
	FileUploading = "uploading"
	FileActive    = "active"
	FileDeleted   = "deleted"
)

// 文件类型。
const (
	FileTypeImage    = "image"
	FileTypeVideo    = "video"
	FileTypeAudio    = "audio"
	FileTypeDocument = "document"
	FileTypeOther    = "other"
	FileTypeApp      = "app" // apk / xapk 等安装包
)

// 订阅档位特殊码。
const TierFree = "free"

// 订阅状态。
const (
	SubActive  = "active"
	SubExpired = "expired"
)

// 四种套餐动作（与 billing 注册的 biz_type 一一对应）。
const (
	ActionNew       = "new"
	ActionRenew     = "renew"
	ActionUpgrade   = "upgrade"
	ActionDowngrade = "downgrade"
)

// 对应 billing biz_type。
const (
	BizLibNew       = "lib_new"
	BizLibUpgrade   = "lib_upgrade"
	BizLibRenew     = "lib_renew"
	BizLibDowngrade = "lib_downgrade"
)

// actionToBizType 把内部 action 映射到 billing biz_type。
func actionToBizType(action string) string {
	switch action {
	case ActionNew:
		return BizLibNew
	case ActionUpgrade:
		return BizLibUpgrade
	case ActionRenew:
		return BizLibRenew
	case ActionDowngrade:
		return BizLibDowngrade
	}
	return ""
}

// ---- 文件与组织（§3.1）----

// LibraryFile 单个逻辑素材文件。内容去重后多个文件可指向同一物理 blob（BlobID>0），
// S3Key 指向所属 blob 的物理对象 key（去重后多个文件 S3Key 相同，所有读路径零改动）。
// 存量文件 BlobID=0、走旧的「文件自有 S3Key、删除即删对象」兼容路径（见 cron.go）。
type LibraryFile struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserID    uint       `gorm:"index:idx_library_files_user;not null" json:"-"`
	FolderID  uint       `gorm:"index:idx_library_files_folder;not null;default:0" json:"folder_id"` // 0=根
	Name      string     `gorm:"size:255" json:"name"`
	Ext       string     `gorm:"size:32" json:"ext"`
	MimeType  string     `gorm:"size:128" json:"mime_type"`
	FileType  string     `gorm:"size:16" json:"file_type"` // image|video|audio|document|other
	SizeBytes int64      `gorm:"not null;default:0" json:"size_bytes"`
	S3Key     string     `gorm:"size:512;index:idx_library_files_s3key" json:"-"`          // 去重后多个文件共享同一 blob 物理 key，故非唯一
	Status    string     `gorm:"size:16;index:idx_library_files_status" json:"status"`     // uploading|active|deleted
	MD5       string     `gorm:"size:64;index:idx_library_files_md5" json:"md5"`           // 内容 md5（去重查重键之一）
	SliceMD5  string     `gorm:"size:32" json:"-"`                                         // md5(前 256KB)，秒传持有证明（冗余便于校验）
	BlobID    uint       `gorm:"index:idx_library_files_blob;not null;default:0" json:"-"` // >0 指向所属 blob；0=存量旧路径
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index:idx_library_files_deleted" json:"-"`
}

func (LibraryFile) TableName() string { return "library_files" }

// LibraryBlob 物理内容对象（全库去重单位）。唯一键 (md5, size_bytes) = 物理去重键。
// 相同 (md5, size_bytes) 全库只存一份 S3 对象，多个逻辑文件以 RefCount 共享；
// 最后一个引用删除（RefCount 归零）时由 cron 真删 S3 对象 + 删 blob 行。
type LibraryBlob struct {
	ID        uint   `gorm:"primaryKey"`
	MD5       string `gorm:"size:32;uniqueIndex:uq_library_blobs_md5size,priority:1;not null"`
	SizeBytes int64  `gorm:"uniqueIndex:uq_library_blobs_md5size,priority:2;not null"`
	SliceMD5  string `gorm:"size:32"`           // md5(前 256KB)，秒传持有证明
	S3Key     string `gorm:"size:512;not null"` // 物理对象 key（首个上传者的 key 直接收编）
	RefCount  int64  `gorm:"not null;default:0"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (LibraryBlob) TableName() string { return "library_blobs" }

// LibraryFolder 文件夹（递归树）。
type LibraryFolder struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index:idx_library_folders_user;not null" json:"-"`
	ParentID  uint      `gorm:"not null;default:0" json:"parent_id"` // 0=根
	Name      string    `gorm:"size:255" json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (LibraryFolder) TableName() string { return "library_folders" }

// LibraryTag 标签。
type LibraryTag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index:idx_library_tags_user;not null" json:"-"`
	Name      string    `gorm:"size:64" json:"name"`
	Color     string    `gorm:"size:16" json:"color"`
	CreatedAt time.Time `json:"created_at"`
}

func (LibraryTag) TableName() string { return "library_tags" }

// LibraryFileTag 文件-标签多对多（唯一约束 file_id+tag_id）。
type LibraryFileTag struct {
	ID     uint `gorm:"primaryKey" json:"id"`
	FileID uint `gorm:"index:idx_library_filetags_file;uniqueIndex:uq_library_filetags_pair,priority:1;not null" json:"file_id"`
	TagID  uint `gorm:"index:idx_library_filetags_tag;uniqueIndex:uq_library_filetags_pair,priority:2;not null" json:"tag_id"`
}

func (LibraryFileTag) TableName() string { return "library_file_tags" }

// ---- 配额与套餐（§3.2）----

// LibrarySubscription 每用户一行 = 当前生效套餐。免费态：无行，或 tier_code=free / expire_at=nil。
type LibrarySubscription struct {
	UserID            uint       `gorm:"primaryKey" json:"-"`
	TierCode          string     `gorm:"size:32" json:"tier_code"` // free 或档位码
	CapacityBytes     int64      `gorm:"not null;default:0" json:"capacity_bytes"`
	MonthlyPriceCents int64      `gorm:"not null;default:0" json:"monthly_price_cents"` // 基准价快照，供升级补差价
	ExpireAt          *time.Time `json:"expire_at"`                                     // 免费=nil 永不过期
	Status            string     `gorm:"size:16" json:"status"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (LibrarySubscription) TableName() string { return "library_subscriptions" }

// LibraryUsage 每用户一行，配额快读副本（真相可由 sum(active files.size_bytes) 重算）。
type LibraryUsage struct {
	UserID    uint      `gorm:"primaryKey" json:"-"`
	UsedBytes int64     `gorm:"not null;default:0" json:"used_bytes"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (LibraryUsage) TableName() string { return "library_usage" }
