package billing

import (
	"time"

	"gorm.io/gorm"
)

type runtimeRepository interface {
	getConfig() (*BillingRuntimeConfig, error)
	saveConfig(cfg *BillingRuntimeConfig) error
	getWatermark(userID int) (*RuntimeSettlementWatermark, error)
	setWatermark(userID int, settledAt time.Time) error
	insertSlice(s *RuntimeUsageSlice) error
	listSlices(userID, offset, limit int) ([]RuntimeUsageSlice, int64, error)
}

type gormRuntimeRepository struct{ db *gorm.DB }

func newRuntimeRepository(db *gorm.DB) runtimeRepository { return &gormRuntimeRepository{db: db} }

// getConfig 取单行配置（id=1）；不存在则返回零值默认（由 seed 保证存在）。
func (r *gormRuntimeRepository) getConfig() (*BillingRuntimeConfig, error) {
	var cfg BillingRuntimeConfig
	err := r.db.Where("id = ?", 1).First(&cfg).Error
	if err == gorm.ErrRecordNotFound {
		return &BillingRuntimeConfig{ID: 1}, nil
	}
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *gormRuntimeRepository) saveConfig(cfg *BillingRuntimeConfig) error {
	cfg.ID = 1
	return r.db.Save(cfg).Error
}

func (r *gormRuntimeRepository) getWatermark(userID int) (*RuntimeSettlementWatermark, error) {
	var w RuntimeSettlementWatermark
	err := r.db.Where("user_id = ?", userID).First(&w).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *gormRuntimeRepository) setWatermark(userID int, settledAt time.Time) error {
	return r.db.Save(&RuntimeSettlementWatermark{UserID: uint(userID), SettlededAt: settledAt}).Error
}

func (r *gormRuntimeRepository) insertSlice(s *RuntimeUsageSlice) error {
	return r.db.Create(s).Error
}

func (r *gormRuntimeRepository) listSlices(userID, offset, limit int) ([]RuntimeUsageSlice, int64, error) {
	q := r.db.Model(&RuntimeUsageSlice{}).Where("user_id = ?", userID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []RuntimeUsageSlice
	err := q.Order("created_at DESC").Offset(offset).Limit(limit).Find(&out).Error
	return out, total, err
}
