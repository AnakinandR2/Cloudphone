package proxy

import "gorm.io/gorm"

// repository 代理持久化。前台读写一律以 userID 约束，从数据层杜绝越权（IDOR）；
// admin* 方法供后台运营查看/删除全量代理池（不限属主）。
type repository interface {
	// 前台（属主隔离）
	count(userID int, kw string) (int64, error)
	list(userID, offset, limit int, kw, orderClause string) ([]Proxy, error)
	listAllOwned(userID int) ([]Proxy, error)
	findByID(userID, id int) (*Proxy, error)
	create(item *Proxy) error
	createBatch(items []Proxy) error
	update(userID, id int, fields map[string]interface{}) error
	delete(userID, id int) error
	// 管理侧（不限属主）
	adminCount(kw, status string) (int64, error)
	adminList(offset, limit int, kw, status, orderClause string) ([]Proxy, error)
	adminFindByID(id int) (*Proxy, error)
	adminDelete(id int) error
}

type gormRepository struct{ db *gorm.DB }

func newRepository(db *gorm.DB) repository { return &gormRepository{db: db} }

// owned 限定为某 user 的代理。
func (r *gormRepository) owned(userID int) *gorm.DB {
	return r.db.Model(&Proxy{}).Where("user_id = ?", userID)
}

// kwFilter 按关键字模糊匹配 name / host。
func kwFilter(q *gorm.DB, kw string) *gorm.DB {
	if kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR host LIKE ?", like, like)
	}
	return q
}

func (r *gormRepository) count(userID int, kw string) (int64, error) {
	var total int64
	err := kwFilter(r.owned(userID), kw).Count(&total).Error
	return total, err
}

func (r *gormRepository) list(userID, offset, limit int, kw, orderClause string) ([]Proxy, error) {
	var items []Proxy
	err := kwFilter(r.owned(userID), kw).Order(orderClause).Offset(offset).Limit(limit).Find(&items).Error
	return items, err
}

// listAllOwned 返回某用户的全部代理（不分页），供前端「绑定代理」下拉选择。
func (r *gormRepository) listAllOwned(userID int) ([]Proxy, error) {
	var items []Proxy
	err := r.owned(userID).Order("id DESC").Find(&items).Error
	return items, err
}

func (r *gormRepository) findByID(userID, id int) (*Proxy, error) {
	var item Proxy
	if err := r.owned(userID).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *gormRepository) create(item *Proxy) error { return r.db.Create(item).Error }

func (r *gormRepository) createBatch(items []Proxy) error { return r.db.Create(&items).Error }

func (r *gormRepository) update(userID, id int, fields map[string]interface{}) error {
	return r.owned(userID).Where("id = ?", id).Updates(fields).Error
}

func (r *gormRepository) delete(userID, id int) error {
	return r.owned(userID).Where("id = ?", id).Delete(&Proxy{}).Error
}

// --- 管理侧（不限属主）---

func (r *gormRepository) adminQuery(kw, status string) *gorm.DB {
	q := kwFilter(r.db.Model(&Proxy{}), kw)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	return q
}

func (r *gormRepository) adminCount(kw, status string) (int64, error) {
	var total int64
	err := r.adminQuery(kw, status).Count(&total).Error
	return total, err
}

func (r *gormRepository) adminList(offset, limit int, kw, status, orderClause string) ([]Proxy, error) {
	var items []Proxy
	err := r.adminQuery(kw, status).Order(orderClause).Offset(offset).Limit(limit).Find(&items).Error
	return items, err
}

func (r *gormRepository) adminFindByID(id int) (*Proxy, error) {
	var item Proxy
	if err := r.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *gormRepository) adminDelete(id int) error {
	return r.db.Delete(&Proxy{}, id).Error
}
