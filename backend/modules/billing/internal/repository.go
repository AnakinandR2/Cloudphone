package billing

import (
	"errors"

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

func (r *gormRepository) applyBalance(userID int, delta int64, typ, reason string, orderID uint, operator string) (*Account, error) {
	return nil, nil // TODO(Task 2)
}

func (r *gormRepository) listLedger(userID, offset, limit int, subject, typ string) ([]LedgerEntry, int64, error) {
	return nil, 0, nil // TODO(Task 3)
}

func (r *gormRepository) countLedger(userID int, subject, typ string) (int64, error) {
	return 0, nil // TODO(Task 3)
}
