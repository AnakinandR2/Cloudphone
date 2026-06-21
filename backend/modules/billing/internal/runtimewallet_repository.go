package billing

import (
	"gorm.io/gorm"
)

// runtimeWalletRepository 临时时长余量 + 每日已扣量的数据访问。
type runtimeWalletRepository interface {
	remaining(userID int) (int64, error)
	addMinutes(userID int, delta int64) (int64, error)            // 返回变更后余量
	consume(userID int, minutes int64) (int64, error)             // 扣减（不足报错），返回变更后余量
	dailyCharged(userID int, instanceID, day string) (int, error) // 某台某天已扣
	addDaily(userID int, instanceID, day string, minutes int) error
}

type gormRuntimeWalletRepository struct{ db *gorm.DB }

func newRuntimeWalletRepository(db *gorm.DB) runtimeWalletRepository {
	return &gormRuntimeWalletRepository{db: db}
}

func (r *gormRuntimeWalletRepository) remaining(userID int) (int64, error) {
	var w RuntimeMinuteWallet
	err := r.db.Where("user_id = ?", userID).First(&w).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	return w.RemainingMinutes, err
}

func (r *gormRuntimeWalletRepository) addMinutes(userID int, delta int64) (int64, error) {
	var w RuntimeMinuteWallet
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(RuntimeMinuteWallet{UserID: uint(userID)}).FirstOrCreate(&w).Error; err != nil {
			return err
		}
		if err := tx.Model(&RuntimeMinuteWallet{}).Where("user_id = ?", userID).
			Update("remaining_minutes", gorm.Expr("remaining_minutes + ?", delta)).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ?", userID).First(&w).Error
	})
	return w.RemainingMinutes, err
}

func (r *gormRuntimeWalletRepository) consume(userID int, minutes int64) (int64, error) {
	var w RuntimeMinuteWallet
	err := r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&RuntimeMinuteWallet{}).
			Where("user_id = ? AND remaining_minutes - ? >= 0", userID, minutes).
			Update("remaining_minutes", gorm.Expr("remaining_minutes - ?", minutes))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound // 余量不足
		}
		return tx.Where("user_id = ?", userID).First(&w).Error
	})
	return w.RemainingMinutes, err
}

func (r *gormRuntimeWalletRepository) dailyCharged(userID int, instanceID, day string) (int, error) {
	var u RuntimeDailyUsage
	err := r.db.Where("user_id = ? AND instance_id = ? AND day_utc8 = ?", userID, instanceID, day).First(&u).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	return u.ChargedMinutes, err
}

func (r *gormRuntimeWalletRepository) addDaily(userID int, instanceID, day string, minutes int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var u RuntimeDailyUsage
		err := tx.Where(RuntimeDailyUsage{UserID: uint(userID), InstanceID: instanceID, DayUTC8: day}).
			FirstOrCreate(&u).Error
		if err != nil {
			return err
		}
		return tx.Model(&RuntimeDailyUsage{}).Where("id = ?", u.ID).
			Update("charged_minutes", gorm.Expr("charged_minutes + ?", minutes)).Error
	})
}
