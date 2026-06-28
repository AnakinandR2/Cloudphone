// Package library 是素材库业务模块的公开入口。
//
// 模块实现位于 modules/library/internal，受 Go internal 机制保护；公开包暴露"取文件"对外接口
// （OpenFile / ListUsableFiles，§7），供后续"推送到手机""脚本输入参数"等消费场景使用——
// 属主校验与超额锁定逻辑只在 internal 一处，跨模块只走本门面。
package library

import (
	"io"
	"time"

	libraryinternal "manager-backend/modules/library/internal"

	"gorm.io/gorm"
)

// FileRef 一个可用文件的引用（含短时有效的 presigned GET URL）。
type FileRef struct {
	FileID    uint
	Name      string
	MimeType  string
	SizeBytes int64
	URL       string
}

// FileContentRef 一个可用文件的内容流（调用方负责 Close Reader）。
type FileContentRef struct {
	FileID    uint
	Name      string
	MimeType  string
	SizeBytes int64
	Reader    io.ReadCloser
}

// UsableFilter 消费侧选择器过滤（按类型/标签/关键字）。
type UsableFilter struct {
	FileType string // image|video|audio|document|other；空=不限
	TagID    uint   // >0 时按标签过滤
	Keyword  string
	Page     int
	Size     int
}

// OpenFile 取一个可用文件：内部统一做属主校验 + 超额锁定校验 → presigned GET。
func OpenFile(userID int, fileID uint) (FileRef, error) {
	r, err := libraryinternal.Service.OpenFile(userID, fileID)
	if err != nil {
		return FileRef{}, err
	}
	return FileRef(r), nil
}

// OpenFileWithTTL 取一个可用文件，但 presigned GET URL 用调用方指定的 ttl（而非默认 GET TTL）。
// 校验与 OpenFile 完全一致（属主校验 + 超额锁定校验 + 仅 active）。
// 用于应用按 URL 安装等需更长有效期、够中台下载完成的场景（见 §7.4）。
func OpenFileWithTTL(userID int, fileID uint, ttl time.Duration) (FileRef, error) {
	r, err := libraryinternal.Service.OpenFileWithTTL(userID, fileID, ttl)
	if err != nil {
		return FileRef{}, err
	}
	return FileRef(r), nil
}

// OpenFileContent 取一个可用文件的字节流：内部统一做属主校验 + 超额锁定校验 + 仅 active → S3 GetObject。
// 私有桶字节直读直传（不经浏览器、不二次 HTTP presigned URL）。调用方负责 Close 返回的 Reader。
func OpenFileContent(userID int, fileID uint) (FileContentRef, error) {
	r, err := libraryinternal.Service.OpenFileContent(userID, fileID)
	if err != nil {
		return FileContentRef{}, err
	}
	return FileContentRef(r), nil
}

// DeleteFileForUser 删除某属主的素材库文件：内部做属主校验 + 软删 + 回收 used_bytes（释放配额）。
// 供应用管理「删素材库文件即应用消失」等消费侧使用——属主与配额逻辑仍只在 internal 一处。
func DeleteFileForUser(ownerUserID int, fileID uint) error {
	return libraryinternal.Service.DeleteFile(ownerUserID, fileID)
}

// ListUsableFiles 列出可用文件（带类型/标签过滤），供消费侧选择器使用。
func ListUsableFiles(userID int, filter UsableFilter) ([]FileRef, error) {
	list, err := libraryinternal.Service.ListUsableFiles(userID, libraryinternal.UsableFilter{
		FileType: filter.FileType,
		TagID:    filter.TagID,
		Keyword:  filter.Keyword,
		Page:     filter.Page,
		Size:     filter.Size,
	})
	if err != nil {
		return nil, err
	}
	out := make([]FileRef, 0, len(list))
	for _, r := range list {
		out = append(out, FileRef(r))
	}
	return out, nil
}

// InitForTest 测试用：供其他模块装配 library（建表 + 装配服务 + 注册 billing 业务类型）。
func InitForTest(db *gorm.DB) error { return libraryinternal.InitForTest(db) }
