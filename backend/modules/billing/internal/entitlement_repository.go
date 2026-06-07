package billing

import (
	"time"

	"gorm.io/gorm"
)

type entitlementRepository interface {
	createBatch(b *EntitlementBatch) error
	capacity(userID int, subject string, now time.Time) (int64, error)
	listActiveBatches(userID int, subject string, now time.Time) ([]EntitlementBatch, error)
	listBatches(userID int, subject string) ([]EntitlementBatch, error)
	addUsed(id int, delta int64) error
	insertLedger(e *LedgerEntry) error
	txWith(fn func(txRepo entitlementRepository) error) error
}

type gormEntitlementRepository struct{ db *gorm.DB }

func newEntitlementRepository(db *gorm.DB) entitlementRepository {
	return &gormEntitlementRepository{db: db}
}

func (r *gormEntitlementRepository) createBatch(b *EntitlementBatch) error {
	return r.db.Create(b).Error
}

// activeScope 未过期且仍有余量的批次。
func (r *gormEntitlementRepository) activeScope(userID int, subject string, now time.Time) *gorm.DB {
	return r.db.Model(&EntitlementBatch{}).
		Where("user_id = ? AND subject = ? AND quantity > used AND (expire_at IS NULL OR expire_at > ?)", userID, subject, now)
}

func (r *gormEntitlementRepository) capacity(userID int, subject string, now time.Time) (int64, error) {
	var capacity int64
	err := r.activeScope(userID, subject, now).
		Select("COALESCE(SUM(quantity - used), 0)").Scan(&capacity).Error
	return capacity, err
}

// listActiveBatches 临近到期优先（永久批次最后），用于 FIFO 消耗。
func (r *gormEntitlementRepository) listActiveBatches(userID int, subject string, now time.Time) ([]EntitlementBatch, error) {
	var items []EntitlementBatch
	err := r.activeScope(userID, subject, now).
		Order("expire_at IS NULL, expire_at ASC, id ASC").Find(&items).Error
	return items, err
}

func (r *gormEntitlementRepository) listBatches(userID int, subject string) ([]EntitlementBatch, error) {
	q := r.db.Model(&EntitlementBatch{}).Where("user_id = ?", userID)
	if subject != "" {
		q = q.Where("subject = ?", subject)
	}
	var items []EntitlementBatch
	err := q.Order("id DESC").Find(&items).Error
	return items, err
}

func (r *gormEntitlementRepository) addUsed(id int, delta int64) error {
	return r.db.Model(&EntitlementBatch{}).Where("id = ?", id).
		UpdateColumn("used", gorm.Expr("used + ?", delta)).Error
}

func (r *gormEntitlementRepository) insertLedger(e *LedgerEntry) error { return r.db.Create(e).Error }

// txWith 在事务内执行 fn，fn 收到一个绑定到事务的 repo。
func (r *gormEntitlementRepository) txWith(fn func(txRepo entitlementRepository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return fn(&gormEntitlementRepository{db: tx})
	})
}
