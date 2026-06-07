package billing

import (
	"errors"

	"gorm.io/gorm"
)

type catalogRepository interface {
	createSku(s *Sku) error
	updateSku(id int, fields map[string]interface{}) error
	deleteSku(id int) error
	getSku(id int) (*Sku, error)
	getSkuByCode(code string) (*Sku, error)
	listSkus(listedOnly bool) ([]Sku, error)

	createTier(t *DiscountTier) error
	updateTier(id int, fields map[string]interface{}) error
	deleteTier(id int) error
	getTier(id int) (*DiscountTier, error)
	listTiersBySku(skuID int) ([]DiscountTier, error)
	listTiersBySkuCycle(skuID, cycleMonths int) ([]DiscountTier, error)
}

type gormCatalogRepository struct{ db *gorm.DB }

func newCatalogRepository(db *gorm.DB) catalogRepository { return &gormCatalogRepository{db: db} }

func (r *gormCatalogRepository) createSku(s *Sku) error { return r.db.Create(s).Error }

func (r *gormCatalogRepository) updateSku(id int, fields map[string]interface{}) error {
	return r.db.Model(&Sku{}).Where("id = ?", id).Updates(fields).Error
}

func (r *gormCatalogRepository) deleteSku(id int) error {
	return r.db.Where("id = ?", id).Delete(&Sku{}).Error
}

func (r *gormCatalogRepository) getSku(id int) (*Sku, error) {
	var s Sku
	if err := r.db.Where("id = ?", id).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *gormCatalogRepository) getSkuByCode(code string) (*Sku, error) {
	var s Sku
	if err := r.db.Where("code = ?", code).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *gormCatalogRepository) listSkus(listedOnly bool) ([]Sku, error) {
	q := r.db.Model(&Sku{})
	if listedOnly {
		q = q.Where("listed = ?", true)
	}
	var items []Sku
	err := q.Order("sort ASC, id ASC").Find(&items).Error
	return items, err
}

func (r *gormCatalogRepository) createTier(t *DiscountTier) error { return r.db.Create(t).Error }

func (r *gormCatalogRepository) updateTier(id int, fields map[string]interface{}) error {
	return r.db.Model(&DiscountTier{}).Where("id = ?", id).Updates(fields).Error
}

func (r *gormCatalogRepository) deleteTier(id int) error {
	return r.db.Where("id = ?", id).Delete(&DiscountTier{}).Error
}

func (r *gormCatalogRepository) getTier(id int) (*DiscountTier, error) {
	var t DiscountTier
	if err := r.db.Where("id = ?", id).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *gormCatalogRepository) listTiersBySku(skuID int) ([]DiscountTier, error) {
	var items []DiscountTier
	err := r.db.Where("sku_id = ?", skuID).Order("cycle_months ASC, min_quantity ASC").Find(&items).Error
	return items, err
}

func (r *gormCatalogRepository) listTiersBySkuCycle(skuID, cycleMonths int) ([]DiscountTier, error) {
	var items []DiscountTier
	err := r.db.Where("sku_id = ? AND cycle_months = ?", skuID, cycleMonths).
		Order("min_quantity ASC").Find(&items).Error
	return items, err
}

// isNotFound 便于 service 把 GORM not-found 翻译成领域错误。
func isNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
