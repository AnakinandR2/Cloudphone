package billing

import (
	"time"

	"gorm.io/gorm"
)

// licenseRepository 授权单元的数据访问（事务/SQL 收敛于此）。
type licenseRepository interface {
	create(units []LicenseUnit) error
	activeUnits(userID int, kind string, now time.Time) ([]LicenseUnit, error)
	listByUserKind(userID int, kind string) ([]LicenseUnit, error)
	getByIDs(userID int, ids []uint, kind string) ([]LicenseUnit, error)
	extendExpiry(ids []uint, addFrom func(cur time.Time) time.Time) error
	setInstance(unitID uint, instanceID string) error
	clearAllOccupancy(userID int, kind string) error
	clearInstanceFor(userID int, kind string, instanceIDs []string) error
	markExpired(now time.Time) (int64, error)
}

type gormLicenseRepository struct{ db *gorm.DB }

func newLicenseRepository(db *gorm.DB) licenseRepository { return &gormLicenseRepository{db: db} }

// licenseInsertBatchSize 分批插入的批大小。LicenseUnit 约 11 个绑定列，
// 200 行 ≈ 2200 变量，远低于 SQLite(32766)/MySQL·PG(65535) 的单语句变量上限，
// 即使将来放大数量上限也不会击穿。
const licenseInsertBatchSize = 200

func (r *gormLicenseRepository) create(units []LicenseUnit) error {
	if len(units) == 0 {
		return nil
	}
	return r.db.CreateInBatches(units, licenseInsertBatchSize).Error
}

// activeUnits 返回某用户某类未过期的可用单元（status=active 且 expire_at>now），按到期倒序。
func (r *gormLicenseRepository) activeUnits(userID int, kind string, now time.Time) ([]LicenseUnit, error) {
	var units []LicenseUnit
	err := r.db.Where("user_id = ? AND kind = ? AND status = ? AND expire_at > ?", userID, kind, LicenseActive, now).
		Order("expire_at DESC").Find(&units).Error
	return units, err
}

func (r *gormLicenseRepository) listByUserKind(userID int, kind string) ([]LicenseUnit, error) {
	var units []LicenseUnit
	err := r.db.Where("user_id = ? AND kind = ?", userID, kind).Order("expire_at DESC").Find(&units).Error
	return units, err
}

func (r *gormLicenseRepository) getByIDs(userID int, ids []uint, kind string) ([]LicenseUnit, error) {
	var units []LicenseUnit
	err := r.db.Where("user_id = ? AND kind = ? AND id IN ?", userID, kind, ids).Find(&units).Error
	return units, err
}

// extendExpiry 对选中单元逐个延长到期（续费）：新到期 = max(now, 当前到期) 之上叠加，由 addFrom 计算。
func (r *gormLicenseRepository) extendExpiry(ids []uint, addFrom func(cur time.Time) time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var units []LicenseUnit
		if err := tx.Where("id IN ?", ids).Find(&units).Error; err != nil {
			return err
		}
		for _, u := range units {
			newExpire := addFrom(u.ExpireAt)
			if err := tx.Model(&LicenseUnit{}).Where("id = ?", u.ID).
				Updates(map[string]interface{}{"expire_at": newExpire, "status": LicenseActive}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *gormLicenseRepository) setInstance(unitID uint, instanceID string) error {
	return r.db.Model(&LicenseUnit{}).Where("id = ?", unitID).
		Update("current_instance_id", instanceID).Error
}

// clearAllOccupancy 清空某用户某类所有单元的占用（reconcile 重新落座前调用）。
func (r *gormLicenseRepository) clearAllOccupancy(userID int, kind string) error {
	return r.db.Model(&LicenseUnit{}).
		Where("user_id = ? AND kind = ? AND current_instance_id <> ''", userID, kind).
		Update("current_instance_id", "").Error
}

// clearInstanceFor 清除指定实例在某类单元上的占用（实例被回收/删除时调用）。
func (r *gormLicenseRepository) clearInstanceFor(userID int, kind string, instanceIDs []string) error {
	if len(instanceIDs) == 0 {
		return nil
	}
	return r.db.Model(&LicenseUnit{}).
		Where("user_id = ? AND kind = ? AND current_instance_id IN ?", userID, kind, instanceIDs).
		Update("current_instance_id", "").Error
}

// markExpired 把已过期的 active 单元置为 expired，并清除其实例占用。返回受影响行数。
func (r *gormLicenseRepository) markExpired(now time.Time) (int64, error) {
	res := r.db.Model(&LicenseUnit{}).
		Where("status = ? AND expire_at <= ?", LicenseActive, now).
		Updates(map[string]interface{}{"status": LicenseExpired, "current_instance_id": ""})
	return res.RowsAffected, res.Error
}
