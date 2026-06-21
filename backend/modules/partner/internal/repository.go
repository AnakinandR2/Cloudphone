package partner

import "gorm.io/gorm"

// repository 合作商 + 点击明细持久化。合作商是运营全局内容（不按属主隔离）。
type repository interface {
	// 合作商
	count(kw string, enabledOnly bool) (int64, error)
	list(offset, limit int, kw string, enabledOnly bool, orderClause string) ([]Partner, error)
	listEnabled() ([]Partner, error)
	findByID(id int) (*Partner, error)
	create(item *Partner) error
	update(id int, fields map[string]interface{}) error
	delete(id int) error
	incrClick(id int) error
	// 点击明细
	createClick(c *PartnerClick) error
	countClicks(partnerID int) (int64, error)
	listClicks(partnerID, offset, limit int) ([]PartnerClick, error)
}

type gormRepository struct{ db *gorm.DB }

func newRepository(db *gorm.DB) repository { return &gormRepository{db: db} }

func partnerQuery(db *gorm.DB, kw string, enabledOnly bool) *gorm.DB {
	q := db.Model(&Partner{})
	if enabledOnly {
		q = q.Where("enabled = ?", true)
	}
	if kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR intro LIKE ?", like, like)
	}
	return q
}

func (r *gormRepository) count(kw string, enabledOnly bool) (int64, error) {
	var total int64
	err := partnerQuery(r.db, kw, enabledOnly).Count(&total).Error
	return total, err
}

func (r *gormRepository) list(offset, limit int, kw string, enabledOnly bool, orderClause string) ([]Partner, error) {
	var items []Partner
	err := partnerQuery(r.db, kw, enabledOnly).Order(orderClause).Offset(offset).Limit(limit).Find(&items).Error
	return items, err
}

// listEnabled 返回全部启用合作商（不分页），按 sort、id 排序，供 my/www 展示。
func (r *gormRepository) listEnabled() ([]Partner, error) {
	var items []Partner
	err := r.db.Model(&Partner{}).Where("enabled = ?", true).Order("sort ASC, id ASC").Find(&items).Error
	return items, err
}

func (r *gormRepository) findByID(id int) (*Partner, error) {
	var item Partner
	if err := r.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *gormRepository) create(item *Partner) error { return r.db.Create(item).Error }

func (r *gormRepository) update(id int, fields map[string]interface{}) error {
	return r.db.Model(&Partner{}).Where("id = ?", id).Updates(fields).Error
}

func (r *gormRepository) delete(id int) error {
	return r.db.Delete(&Partner{}, id).Error
}

// incrClick 原子自增点击总数缓存，避免并发「读-改-写」覆盖。
func (r *gormRepository) incrClick(id int) error {
	return r.db.Model(&Partner{}).Where("id = ?", id).
		UpdateColumn("click_count", gorm.Expr("click_count + 1")).Error
}

func (r *gormRepository) createClick(c *PartnerClick) error { return r.db.Create(c).Error }

func (r *gormRepository) countClicks(partnerID int) (int64, error) {
	var total int64
	err := r.db.Model(&PartnerClick{}).Where("partner_id = ?", partnerID).Count(&total).Error
	return total, err
}

func (r *gormRepository) listClicks(partnerID, offset, limit int) ([]PartnerClick, error) {
	var items []PartnerClick
	err := r.db.Where("partner_id = ?", partnerID).Order("id DESC").Offset(offset).Limit(limit).Find(&items).Error
	return items, err
}
