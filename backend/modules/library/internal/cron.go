package library

import (
	"context"
	"log"
	"time"

	"manager-backend/framework"

	"gorm.io/gorm"
)

// staleUploadAge 残留 uploading 行的清理阈值（超过即视为废弃，删 S3 + 删行）。
const staleUploadAge = 24 * time.Hour

// sweepExpired 过期降级（§4.3）：expire_at<now 且 tier!=free 的订阅 → 重置为免费。
// 纯函数（db + now 可注入），返回降级的用户数，便于单测。
//
// minor-4：
//   - 复用 repository.listExpiredPaidSubs（不再裸 db 重查）。
//   - 降级后 status 置为生效态 SubActive（免费态本身是一个生效中的套餐，与 FulfillPackage 一致）。
//   - expire_at 显式写 NULL（Updates 的 map 里 nil 会被 GORM 写成 NULL）。
//   - capacity 取实时定价配置的免费额度（minor-6）。
func sweepExpired(db *gorm.DB, now time.Time) (int, error) {
	freeBytes := defaultPricingConfig().FreeQuotaBytes
	// 从定价配置读免费额度（缺失回退默认）。
	var cfg LibraryPricingConfig
	if err := db.Where("id = ?", 1).First(&cfg).Error; err == nil && cfg.FreeQuotaBytes > 0 {
		freeBytes = cfg.FreeQuotaBytes
	}

	repo := newRepository(db)
	subs, err := repo.listExpiredPaidSubs(now)
	if err != nil {
		return 0, err
	}
	n := 0
	for i := range subs {
		// Select 显式包含 expire_at + status，确保 nil/零值被写入（GORM 默认跳过零值更新）。
		err := db.Model(&LibrarySubscription{}).
			Where("user_id = ?", subs[i].UserID).
			Select("tier_code", "capacity_bytes", "monthly_price_cents", "expire_at", "status", "updated_at").
			Updates(map[string]any{
				"tier_code":           TierFree,
				"capacity_bytes":      freeBytes,
				"monthly_price_cents": 0,
				"expire_at":           nil,
				"status":              SubActive,
				"updated_at":          now,
			}).Error
		if err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// sweepStaleUploads 残留 upload 清理（§5.4）：status=uploading 且超过 staleUploadAge 未确认
// → 删 S3 对象（若已配置且存在）+ 删行，释放配额预占。返回清理的行数。
// S3 删除失败不阻断删行（占位行才是配额预占来源；孤儿对象可由 S3 生命周期兜底）。
func sweepStaleUploads(db *gorm.DB, now time.Time) (int, error) {
	before := now.Add(-staleUploadAge)
	var files []LibraryFile
	if err := db.Where("status = ? AND created_at < ?", FileUploading, before).
		Find(&files).Error; err != nil {
		return 0, err
	}
	repo := newRepository(db)
	n := 0
	for i := range files {
		// 去重文件（blob_id>0）：S3Key 已被 tryInstantHit 改写为 blob 的共享物理 key
		// （秒传 incRef+updateFile 成功但 confirmActiveWithinCapacity 失败，残留 uploading 行）。
		// 此处**绝不**按 file.S3Key 删 S3——那是所有同 blob 去重文件共用的物理对象，
		// 删了会静默损坏其它文件（与 sweepSoftDeletedFiles 的 blob_id 分流一致）。
		// 同时 tryInstantHit 已对该 blob incRef，删行前须 decRef 释放这条悬挂引用，
		// 否则 ref_count 永久虚高、孤儿 blob 清理永不回收。
		if files[i].BlobID > 0 {
			if err := repo.decRef(files[i].BlobID); err != nil {
				return n, err
			}
		} else if framework.S3Library != nil && files[i].S3Key != "" {
			// 存量/真实上传残留：对象归该 uploading 行独有，硬删前删对象。
			if exists, _ := framework.S3Library.Exists(context.Background(), files[i].S3Key); exists {
				_ = framework.S3Library.DeleteObject(context.Background(), files[i].S3Key)
			}
		}
		if err := db.Unscoped().Delete(&LibraryFile{}, files[i].ID).Error; err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// sweepSoftDeletedFiles 硬删已软删的文件行（§7），按 blob_id 分流物理对象删除：
//   - 去重文件（blob_id>0）：对象归 blob 所有，**不**按文件 key 删 S3（仅由孤儿 blob 清理负责）；
//     该文件软删时已 decRef，这里只硬删行。
//   - 存量文件（blob_id==0，旧路径）：保持原逻辑——硬删前 DeleteObject(file.S3Key)。
//
// 返回硬删的行数。S3 删除失败不阻断删行（孤儿对象可由 S3 生命周期兜底）。
func sweepSoftDeletedFiles(db *gorm.DB, _ time.Time) (int, error) {
	repo := newRepository(db)
	files, err := repo.listSoftDeletedFiles()
	if err != nil {
		return 0, err
	}
	n := 0
	for i := range files {
		f := files[i]
		if f.BlobID == 0 && framework.S3Library != nil && f.S3Key != "" {
			// 存量旧路径：对象归该文件独有，硬删前删对象。
			if exists, _ := framework.S3Library.Exists(context.Background(), f.S3Key); exists {
				_ = framework.S3Library.DeleteObject(context.Background(), f.S3Key)
			}
		}
		if err := repo.hardDeleteFile(f.ID); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// sweepOrphanBlobs 孤儿 blob 清理（§7、§8 新增）：扫 ref_count<=0 的 blob，
// 行锁内再次确认仍为 0 → DeleteObject(blob.S3Key) + 删 blob 行（防与并发新引用竞态）。
// 复核非 0（被并发新引用）→ 跳过，不删对象、不删行。返回真删的 blob 数。
func sweepOrphanBlobs(db *gorm.DB, _ time.Time) (int, error) {
	repo := newRepository(db)
	blobs, err := repo.listOrphanBlobs()
	if err != nil {
		return 0, err
	}
	n := 0
	for i := range blobs {
		b := blobs[i]
		deleted, err := repo.deleteBlobIfZero(b.ID)
		if err != nil {
			return n, err
		}
		if !deleted {
			continue // 被并发新引用，保留对象
		}
		// 行已删（且锁内复核 ref==0）→ 真删 S3 对象。失败不回滚（孤儿对象可由生命周期兜底）。
		if framework.S3Library != nil && b.S3Key != "" {
			_ = framework.S3Library.DeleteObject(context.Background(), b.S3Key)
		}
		n++
	}
	return n, nil
}

// runDailySweeps 执行一轮 cron（过期降级 + 残留清理 + 软删硬删 + 孤儿 blob 真删），记日志、不抛 panic。
func (m *libraryModule) runDailySweeps(now time.Time) {
	if m.db == nil {
		return
	}
	if n, err := sweepExpired(m.db, now); err != nil {
		log.Printf("[library] sweepExpired error: %v", err)
	} else if n > 0 {
		log.Printf("[library] sweepExpired: %d subscriptions downgraded to free", n)
	}
	if n, err := sweepStaleUploads(m.db, now); err != nil {
		log.Printf("[library] sweepStaleUploads error: %v", err)
	} else if n > 0 {
		log.Printf("[library] sweepStaleUploads: %d stale uploads cleaned", n)
	}
	if n, err := sweepSoftDeletedFiles(m.db, now); err != nil {
		log.Printf("[library] sweepSoftDeletedFiles error: %v", err)
	} else if n > 0 {
		log.Printf("[library] sweepSoftDeletedFiles: %d soft-deleted files hard-deleted", n)
	}
	if n, err := sweepOrphanBlobs(m.db, now); err != nil {
		log.Printf("[library] sweepOrphanBlobs error: %v", err)
	} else if n > 0 {
		log.Printf("[library] sweepOrphanBlobs: %d orphan blobs purged", n)
	}
}
