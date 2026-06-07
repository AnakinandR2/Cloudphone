package billing

import (
	"errors"

	"manager-backend/framework/apperr"

	"gorm.io/gorm"
)

type repository interface {
	getOrCreateAccount(userID int) (*Account, error)
	applyBalance(userID int, delta int64, typ, reason string, orderID uint, operator string) (*Account, error)
	listLedger(userID, offset, limit int, subject, typ string) ([]LedgerEntry, int64, error)
	countLedger(userID int, subject, typ string) (int64, error)
}

type gormRepository struct{ db *gorm.DB }

func newRepository(db *gorm.DB) repository { return &gormRepository{db: db} }

// getOrCreateAccount 取当前用户账户，不存在则建（余额 0）。
// 并发竞态下若另一请求已抢先创建（唯一索引冲突），回退为直接读取。
func (r *gormRepository) getOrCreateAccount(userID int) (*Account, error) {
	var acc Account
	err := r.db.Where(Account{UserID: uint(userID)}).FirstOrCreate(&acc).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		err = r.db.Where("user_id = ?", userID).First(&acc).Error
	}
	if err != nil {
		return nil, err
	}
	return &acc, nil
}

// applyBalance 原子地变更余额并写一条流水。
// 先确保账户存在（并发安全），再用「带条件的 UPDATE」防止丢失更新与扣成负数：
// 余额不足时 RowsAffected=0 → 报错。整笔变更与流水在同一事务内提交。
func (r *gormRepository) applyBalance(userID int, delta int64, typ, reason string, orderID uint, operator string) (*Account, error) {
	if _, err := r.getOrCreateAccount(userID); err != nil {
		return nil, err
	}
	var acc Account
	err := r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&Account{}).
			Where("user_id = ? AND balance_cents + ? >= 0", userID, delta).
			UpdateColumn("balance_cents", gorm.Expr("balance_cents + ?", delta))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return apperr.Validation("余额不足")
		}
		if err := tx.Where("user_id = ?", userID).First(&acc).Error; err != nil {
			return err
		}
		entry := LedgerEntry{
			UserID: uint(userID), Subject: SubjectBalance, Type: typ,
			DeltaCents: delta, BalanceAfterCents: acc.BalanceCents,
			Reason: reason, OrderID: orderID, Operator: operator,
		}
		return tx.Create(&entry).Error
	})
	if err != nil {
		return nil, err
	}
	return &acc, nil
}

func (r *gormRepository) ledgerScope(userID int, subject, typ string) *gorm.DB {
	q := r.db.Model(&LedgerEntry{}).Where("user_id = ?", userID)
	if subject != "" {
		q = q.Where("subject = ?", subject)
	}
	if typ != "" {
		q = q.Where("type = ?", typ)
	}
	return q
}

func (r *gormRepository) countLedger(userID int, subject, typ string) (int64, error) {
	var total int64
	err := r.ledgerScope(userID, subject, typ).Count(&total).Error
	return total, err
}

func (r *gormRepository) listLedger(userID, offset, limit int, subject, typ string) ([]LedgerEntry, int64, error) {
	total, err := r.countLedger(userID, subject, typ)
	if err != nil {
		return nil, 0, err
	}
	var items []LedgerEntry
	err = r.ledgerScope(userID, subject, typ).Order("id DESC").Offset(offset).Limit(limit).Find(&items).Error
	return items, total, err
}
