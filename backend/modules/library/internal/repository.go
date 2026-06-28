package library

import (
	"errors"
	"strings"
	"time"

	"manager-backend/framework/apperr"
	"manager-backend/framework/query"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// fileListFilter 文件列表过滤条件（§8 GET /library/files）。
type fileListFilter struct {
	FolderID    *uint  // nil=不限；否则按文件夹（0=根）
	FolderIDSet bool   // 是否传了 folder_id
	FileType    string // image|video|audio|document|other
	TagID       uint   // >0 时按标签过滤
	Keyword     string // 文件名模糊匹配
	Order       string // SafeOrder 字段
	Sort        string // ascending|descending
	Page        int
	Size        int
}

// repository 素材库订阅/用量/文件/夹/标签读写。
type repository interface {
	// 订阅。
	getSubscription(userID int) (*LibrarySubscription, error)
	upsertSubscription(sub *LibrarySubscription) error
	// 过期降级 cron：列出已过期的付费订阅。
	listExpiredPaidSubs(now time.Time) ([]LibrarySubscription, error)

	// 用量。
	getUsage(userID int) (*LibraryUsage, error)
	addUsedBytes(userID int, delta int64) error // 增量维护 used_bytes（可负）
	setUsedBytes(userID int, used int64) error  // reconcile 用：直接覆盖
	sumActiveSize(userID int) (int64, error)    // sum(active files.size_bytes)
	sumPendingSize(userID int) (int64, error)   // sum(active+uploading 声明大小)，配额预占

	// 文件。
	createFile(f *LibraryFile) error
	fileNameExists(userID int, folderID uint, name string, excludeID uint) (bool, error)
	// reserveUpload 事务内配额预占：重算 sum(active+uploading 声明大小)，校验 +本次 <= capacity 后再 INSERT。
	// 关闭 presign 的 TOCTOU 超卖窗口（sqlite 写事务串行；mysql/postgres 可对用量行加 FOR UPDATE）。
	reserveUpload(userID int, f *LibraryFile, capacity int64) error
	// confirmActiveWithinCapacity 事务内 confirm：以真实大小修正并置 active，
	// 若真实 active 总和（含本次真实大小）> capacity 则回滚并返回容量不足错误（不污染 used_bytes）。
	confirmActiveWithinCapacity(userID int, fileID uint, realSize, capacity int64) error
	getFile(userID int, id uint) (*LibraryFile, error)
	getFileByID(id uint) (*LibraryFile, error) // 不限属主（cron/门面内部用）
	updateFile(f *LibraryFile) error
	listFiles(userID int, f fileListFilter) ([]LibraryFile, int64, error)
	softDeleteFile(userID int, id uint, now time.Time) error
	listStaleUploads(before time.Time) ([]LibraryFile, error)
	listSoftDeletedFiles() ([]LibraryFile, error)
	hardDeleteFile(id uint) error

	// blob 去重 / 引用计数（§3、§7、§8）。
	getBlobByMD5Size(md5 string, size int64) (*LibraryBlob, error)
	createBlob(b *LibraryBlob) error // (md5,size) 唯一键冲突时返回 errBlobConflict，调用方回读再合并
	incRef(blobID uint) error        // blob 行锁内 ref_count++
	decRef(blobID uint) error        // blob 行锁内 ref_count--（夹紧到 >=0）
	listOrphanBlobs() ([]LibraryBlob, error)
	deleteBlobIfZero(blobID uint) (bool, error) // 行锁内复核 ref_count==0 后删 blob 行，返回是否删除
	deleteBlob(blobID uint) error               // 无条件硬删 blob 行（收编失败、尚无文件引用时的回滚清理）

	// 文件夹。
	createFolder(fo *LibraryFolder) error
	folderNameExists(userID int, parentID uint, name string, excludeID uint) (bool, error)
	getFolder(userID int, id uint) (*LibraryFolder, error)
	updateFolder(fo *LibraryFolder) error
	deleteFolder(userID int, id uint) error
	listFolders(userID int) ([]LibraryFolder, error)
	countFilesInFolder(userID int, folderID uint) (int64, error)
	countSubFolders(userID int, folderID uint) (int64, error)

	// 标签。
	createTag(t *LibraryTag) error
	getTag(userID int, id uint) (*LibraryTag, error)
	updateTag(t *LibraryTag) error
	deleteTag(userID int, id uint) error
	listTags(userID int) ([]LibraryTag, error)
	setFileTags(fileID uint, tagIDs []uint) error
	listFileTagIDs(fileID uint) ([]uint, error)
}

type gormRepository struct{ db *gorm.DB }

func newRepository(db *gorm.DB) repository {
	return &gormRepository{db: db}
}

// getSubscription 取当前订阅；不存在返回 (nil, nil)（= 免费态）。
func (r *gormRepository) getSubscription(userID int) (*LibrarySubscription, error) {
	var sub LibrarySubscription
	err := r.db.Where("user_id = ?", userID).First(&sub).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// upsertSubscription 写入/更新某用户的订阅行（user_id 为主键）。
func (r *gormRepository) upsertSubscription(sub *LibrarySubscription) error {
	sub.UpdatedAt = time.Now()
	return r.db.Save(sub).Error
}

func (r *gormRepository) listExpiredPaidSubs(now time.Time) ([]LibrarySubscription, error) {
	var subs []LibrarySubscription
	err := r.db.Where("tier_code <> ? AND expire_at IS NOT NULL AND expire_at < ?", TierFree, now).Find(&subs).Error
	return subs, err
}

// getUsage 取用量；不存在返回零值用量。
func (r *gormRepository) getUsage(userID int) (*LibraryUsage, error) {
	var u LibraryUsage
	err := r.db.Where("user_id = ?", userID).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &LibraryUsage{UserID: uint(userID)}, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// addUsedBytes 增量维护 used_bytes（行不存在则建）。delta 可负；夹紧到 >=0。
func (r *gormRepository) addUsedBytes(userID int, delta int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var u LibraryUsage
		err := tx.Where("user_id = ?", userID).First(&u).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			used := delta
			if used < 0 {
				used = 0
			}
			return tx.Create(&LibraryUsage{UserID: uint(userID), UsedBytes: used, UpdatedAt: time.Now()}).Error
		}
		if err != nil {
			return err
		}
		u.UsedBytes += delta
		if u.UsedBytes < 0 {
			u.UsedBytes = 0
		}
		u.UpdatedAt = time.Now()
		return tx.Save(&u).Error
	})
}

// setUsedBytes 直接覆盖 used_bytes（reconcile 用）。
func (r *gormRepository) setUsedBytes(userID int, used int64) error {
	if used < 0 {
		used = 0
	}
	u := LibraryUsage{UserID: uint(userID), UsedBytes: used, UpdatedAt: time.Now()}
	return r.db.Save(&u).Error
}

// sumActiveSize 真实占用 = sum(active 文件 size_bytes)。
func (r *gormRepository) sumActiveSize(userID int) (int64, error) {
	var sum *int64
	err := r.db.Model(&LibraryFile{}).
		Where("user_id = ? AND status = ? AND deleted_at IS NULL", userID, FileActive).
		Select("COALESCE(SUM(size_bytes),0)").Scan(&sum).Error
	if err != nil {
		return 0, err
	}
	if sum == nil {
		return 0, nil
	}
	return *sum, nil
}

// sumPendingSize 配额预占占用 = sum(active + uploading 声明 size_bytes)。
func (r *gormRepository) sumPendingSize(userID int) (int64, error) {
	var sum *int64
	err := r.db.Model(&LibraryFile{}).
		Where("user_id = ? AND status IN ? AND deleted_at IS NULL", userID, []string{FileActive, FileUploading}).
		Select("COALESCE(SUM(size_bytes),0)").Scan(&sum).Error
	if err != nil {
		return 0, err
	}
	if sum == nil {
		return 0, nil
	}
	return *sum, nil
}

// ---- 文件 ----

func (r *gormRepository) createFile(f *LibraryFile) error { return r.db.Create(f).Error }

// fileNameExists 判断同夹下是否已有同名文件（active/uploading，未删除；excludeID!=0 排除自身）。
func (r *gormRepository) fileNameExists(userID int, folderID uint, name string, excludeID uint) (bool, error) {
	q := r.db.Model(&LibraryFile{}).
		Where("user_id = ? AND folder_id = ? AND name = ? AND status <> ? AND deleted_at IS NULL",
			userID, folderID, name, FileDeleted)
	if excludeID != 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var cnt int64
	if err := q.Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// reserveUpload 事务内配额预占（major-3）：事务里重算 SUM 后再 INSERT，关闭 TOCTOU 窗口。
// sqlite 写事务天然串行即足够；mysql/postgres 对 library_usage 用量行加行锁（FOR UPDATE）
// 把并发预占串行化到同一用户的用量行上（用量行可能不存在，故先确保存在再锁）。
func (r *gormRepository) reserveUpload(userID int, f *LibraryFile, capacity int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 对该用户用量行加锁，串行化同一用户的并发预占（sqlite 无 FOR UPDATE 但写事务已串行）。
		if err := lockUsageRow(tx, userID); err != nil {
			return err
		}
		var sum *int64
		if err := tx.Model(&LibraryFile{}).
			Where("user_id = ? AND status IN ? AND deleted_at IS NULL", userID, []string{FileActive, FileUploading}).
			Select("COALESCE(SUM(size_bytes),0)").Scan(&sum).Error; err != nil {
			return err
		}
		pending := int64(0)
		if sum != nil {
			pending = *sum
		}
		if pending+f.SizeBytes > capacity {
			return apperr.Validation("容量不足，请扩容")
		}
		return tx.Create(f).Error
	})
}

// confirmActiveWithinCapacity 事务内 confirm（major-3）：修正真实大小并置 active，
// 校验真实 active 总和不超容量；越界则回滚（不修改文件、不累加 used_bytes）。
func (r *gormRepository) confirmActiveWithinCapacity(userID int, fileID uint, realSize, capacity int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockUsageRow(tx, userID); err != nil {
			return err
		}
		// 其他 active 文件总和（排除本行，本行此刻仍是 uploading）。
		var sum *int64
		if err := tx.Model(&LibraryFile{}).
			Where("user_id = ? AND status = ? AND deleted_at IS NULL AND id <> ?", userID, FileActive, fileID).
			Select("COALESCE(SUM(size_bytes),0)").Scan(&sum).Error; err != nil {
			return err
		}
		others := int64(0)
		if sum != nil {
			others = *sum
		}
		if others+realSize > capacity {
			return apperr.Validation("容量不足，请扩容或删除文件后再使用")
		}
		// 修正大小 + 置 active。
		if err := tx.Model(&LibraryFile{}).
			Where("id = ? AND user_id = ?", fileID, userID).
			Updates(map[string]any{"size_bytes": realSize, "status": FileActive, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		// 累加 used_bytes（行内维护，事务原子）。
		return addUsedBytesTx(tx, userID, realSize)
	})
}

// lockUsageRow 确保用量行存在并（在支持的数据库上）加行锁，串行化同用户的配额事务。
func lockUsageRow(tx *gorm.DB, userID int) error {
	var u LibraryUsage
	err := tx.Where("user_id = ?", userID).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 先建出用量行（幂等：并发下 ON CONFLICT 由唯一主键兜底）。
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&LibraryUsage{UserID: uint(userID), UsedBytes: 0, UpdatedAt: time.Now()}).Error; err != nil {
			return err
		}
		err = nil
	}
	if err != nil {
		return err
	}
	// 非 sqlite 加 FOR UPDATE 行锁（sqlite 不支持，写事务已串行，跳过即可）。
	if tx.Dialector.Name() != "sqlite" {
		return tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", userID).First(&LibraryUsage{}).Error
	}
	return nil
}

// addUsedBytesTx 在给定事务内增量维护 used_bytes（行已由 lockUsageRow 建出）。
func addUsedBytesTx(tx *gorm.DB, userID int, delta int64) error {
	var u LibraryUsage
	if err := tx.Where("user_id = ?", userID).First(&u).Error; err != nil {
		return err
	}
	u.UsedBytes += delta
	if u.UsedBytes < 0 {
		u.UsedBytes = 0
	}
	u.UpdatedAt = time.Now()
	return tx.Save(&u).Error
}

func (r *gormRepository) getFile(userID int, id uint) (*LibraryFile, error) {
	var f LibraryFile
	err := r.db.Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).First(&f).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *gormRepository) getFileByID(id uint) (*LibraryFile, error) {
	var f LibraryFile
	err := r.db.Where("id = ?", id).First(&f).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *gormRepository) updateFile(f *LibraryFile) error {
	f.UpdatedAt = time.Now()
	return r.db.Save(f).Error
}

func (r *gormRepository) listFiles(userID int, f fileListFilter) ([]LibraryFile, int64, error) {
	q := r.db.Model(&LibraryFile{}).
		Where("library_files.user_id = ? AND library_files.status = ? AND library_files.deleted_at IS NULL", userID, FileActive)
	if f.FolderIDSet {
		q = q.Where("library_files.folder_id = ?", *f.FolderID)
	}
	if f.FileType != "" {
		q = q.Where("library_files.file_type = ?", f.FileType)
	}
	if f.Keyword != "" {
		q = q.Where("library_files.name LIKE ?", "%"+f.Keyword+"%")
	}
	if f.TagID > 0 {
		q = q.Joins("JOIN library_file_tags lft ON lft.file_id = library_files.id").
			Where("lft.tag_id = ?", f.TagID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	order := query.SafeOrder(f.Order, f.Sort, map[string]bool{
		"created_at": true, "updated_at": true, "name": true, "size_bytes": true,
	}, "library_files.id DESC")
	page, size := f.Page, f.Size
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}
	var list []LibraryFile
	err := q.Order(order).Limit(size).Offset((page - 1) * size).Find(&list).Error
	return list, total, err
}

func (r *gormRepository) softDeleteFile(userID int, id uint, now time.Time) error {
	return r.db.Model(&LibraryFile{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).
		Updates(map[string]any{"status": FileDeleted, "deleted_at": now, "updated_at": now}).Error
}

func (r *gormRepository) listStaleUploads(before time.Time) ([]LibraryFile, error) {
	var list []LibraryFile
	err := r.db.Where("status = ? AND created_at < ?", FileUploading, before).Find(&list).Error
	return list, err
}

func (r *gormRepository) listSoftDeletedFiles() ([]LibraryFile, error) {
	var list []LibraryFile
	err := r.db.Where("status = ? AND deleted_at IS NOT NULL", FileDeleted).Find(&list).Error
	return list, err
}

func (r *gormRepository) hardDeleteFile(id uint) error {
	return r.db.Unscoped().Delete(&LibraryFile{}, id).Error
}

// ---- blob 去重 / 引用计数（§3、§7、§8）----

// errBlobConflict createBlob 命中 (md5,size) 唯一约束时返回，调用方回读 blob 走合并分支。
var errBlobConflict = errors.New("library: blob (md5,size) conflict")

// getBlobByMD5Size 按 (md5, size_bytes) 查 blob；不存在返回 (nil, nil)。
func (r *gormRepository) getBlobByMD5Size(md5 string, size int64) (*LibraryBlob, error) {
	var b LibraryBlob
	err := r.db.Where("md5 = ? AND size_bytes = ?", md5, size).First(&b).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// createBlob 建一个新 blob 行；命中 (md5,size) 唯一约束 → 返回 errBlobConflict
// （调用方应回读 blob 走合并分支：ref++ + 删自身 tempKey）。
func (r *gormRepository) createBlob(b *LibraryBlob) error {
	err := r.db.Create(b).Error
	if err != nil && isUniqueViolation(err) {
		return errBlobConflict
	}
	return err
}

// incRef blob 行锁内 ref_count++（sqlite 写事务串行；mysql/pg 走 FOR UPDATE）。
func (r *gormRepository) incRef(blobID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		b, err := lockBlobRow(tx, blobID)
		if err != nil {
			return err
		}
		b.RefCount++
		b.UpdatedAt = time.Now()
		return tx.Save(b).Error
	})
}

// decRef blob 行锁内 ref_count--（夹紧到 >=0）。
func (r *gormRepository) decRef(blobID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		b, err := lockBlobRow(tx, blobID)
		if err != nil {
			return err
		}
		b.RefCount--
		if b.RefCount < 0 {
			b.RefCount = 0
		}
		b.UpdatedAt = time.Now()
		return tx.Save(b).Error
	})
}

// listOrphanBlobs 列出 ref_count<=0 的孤儿 blob（cron 真删候选）。
func (r *gormRepository) listOrphanBlobs() ([]LibraryBlob, error) {
	var list []LibraryBlob
	err := r.db.Where("ref_count <= 0").Find(&list).Error
	return list, err
}

// deleteBlobIfZero 行锁内复核 ref_count==0 后删 blob 行，返回是否真的删除。
// 复核失败（其间被并发新引用）→ 不删、返回 false，避免删掉仍被引用的物理对象。
func (r *gormRepository) deleteBlobIfZero(blobID uint) (bool, error) {
	deleted := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		b, err := lockBlobRow(tx, blobID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil // 已被删，幂等
			}
			return err
		}
		if b.RefCount > 0 {
			return nil // 复核非 0：被并发新引用，放弃删除
		}
		if err := tx.Delete(&LibraryBlob{}, blobID).Error; err != nil {
			return err
		}
		deleted = true
		return nil
	})
	return deleted, err
}

// deleteBlob 无条件硬删 blob 行。仅用于「createBlob 成功（ref=1）但后续 updateFile 失败、
// 尚无任何文件指向该 blob」的回滚清理：此刻 ref=1 仅来自刚建的 blob 自身，无并发引用风险。
func (r *gormRepository) deleteBlob(blobID uint) error {
	return r.db.Delete(&LibraryBlob{}, blobID).Error
}

// lockBlobRow 在事务内取 blob 行并（在支持的库上）加 FOR UPDATE 行锁。
// 复用 lockUsageRow 同款封装：sqlite 不支持 FOR UPDATE，写事务天然串行即足够。
func lockBlobRow(tx *gorm.DB, blobID uint) (*LibraryBlob, error) {
	q := tx
	if tx.Dialector.Name() != "sqlite" {
		q = tx.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var b LibraryBlob
	if err := q.Where("id = ?", blobID).First(&b).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

// isUniqueViolation 判断错误是否为唯一约束冲突（跨 sqlite/mysql/postgres 文案兜底）。
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "UNIQUE constraint failed") || // sqlite
		strings.Contains(s, "Duplicate entry") || // mysql
		strings.Contains(s, "duplicate key value") // postgres
}

// ---- 文件夹 ----

func (r *gormRepository) createFolder(fo *LibraryFolder) error { return r.db.Create(fo).Error }

// folderNameExists 判断同一用户、同一父夹下是否已有同名文件夹（excludeID!=0 时排除自身，供重命名用）。
func (r *gormRepository) folderNameExists(userID int, parentID uint, name string, excludeID uint) (bool, error) {
	q := r.db.Model(&LibraryFolder{}).
		Where("user_id = ? AND parent_id = ? AND name = ?", userID, parentID, name)
	if excludeID != 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var cnt int64
	if err := q.Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

func (r *gormRepository) getFolder(userID int, id uint) (*LibraryFolder, error) {
	var fo LibraryFolder
	err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&fo).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &fo, nil
}

func (r *gormRepository) updateFolder(fo *LibraryFolder) error {
	fo.UpdatedAt = time.Now()
	return r.db.Save(fo).Error
}

func (r *gormRepository) deleteFolder(userID int, id uint) error {
	return r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&LibraryFolder{}).Error
}

func (r *gormRepository) listFolders(userID int) ([]LibraryFolder, error) {
	var list []LibraryFolder
	err := r.db.Where("user_id = ?", userID).Order("id ASC").Find(&list).Error
	return list, err
}

func (r *gormRepository) countFilesInFolder(userID int, folderID uint) (int64, error) {
	var n int64
	err := r.db.Model(&LibraryFile{}).
		Where("user_id = ? AND folder_id = ? AND status <> ? AND deleted_at IS NULL", userID, folderID, FileDeleted).
		Count(&n).Error
	return n, err
}

func (r *gormRepository) countSubFolders(userID int, folderID uint) (int64, error) {
	var n int64
	err := r.db.Model(&LibraryFolder{}).
		Where("user_id = ? AND parent_id = ?", userID, folderID).Count(&n).Error
	return n, err
}

// ---- 标签 ----

func (r *gormRepository) createTag(t *LibraryTag) error { return r.db.Create(t).Error }

func (r *gormRepository) getTag(userID int, id uint) (*LibraryTag, error) {
	var t LibraryTag
	err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *gormRepository) updateTag(t *LibraryTag) error { return r.db.Save(t).Error }

func (r *gormRepository) deleteTag(userID int, id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND user_id = ?", id, userID).Delete(&LibraryTag{}).Error; err != nil {
			return err
		}
		return tx.Where("tag_id = ?", id).Delete(&LibraryFileTag{}).Error
	})
}

func (r *gormRepository) listTags(userID int) ([]LibraryTag, error) {
	var list []LibraryTag
	err := r.db.Where("user_id = ?", userID).Order("id ASC").Find(&list).Error
	return list, err
}

// setFileTags 覆盖式设置文件标签（先清后插，幂等）。
func (r *gormRepository) setFileTags(fileID uint, tagIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("file_id = ?", fileID).Delete(&LibraryFileTag{}).Error; err != nil {
			return err
		}
		for _, tid := range tagIDs {
			if tid == 0 {
				continue
			}
			if err := tx.Create(&LibraryFileTag{FileID: fileID, TagID: tid}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *gormRepository) listFileTagIDs(fileID uint) ([]uint, error) {
	var ids []uint
	err := r.db.Model(&LibraryFileTag{}).Where("file_id = ?", fileID).
		Pluck("tag_id", &ids).Error
	return ids, err
}
