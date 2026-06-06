package example

import "gorm.io/gorm"

// repository 封装示例模块的持久化，隔离 GORM 细节，便于 service 单测与替换实现。
type repository interface {
	count(title string) (int64, error)
	list(offset, limit int, title, orderClause string) ([]ExampleItem, error)
	findByID(id int) (*ExampleItem, error)
	create(item *ExampleItem) error
	update(id int, fields map[string]interface{}) error
	delete(id int) error
}

type gormRepository struct{ db *gorm.DB }

func newRepository(db *gorm.DB) repository { return &gormRepository{db: db} }

func (r *gormRepository) baseQuery(title string) *gorm.DB {
	q := r.db.Model(&ExampleItem{})
	if title != "" {
		q = q.Where("title LIKE ?", "%"+title+"%")
	}
	return q
}

func (r *gormRepository) count(title string) (int64, error) {
	var total int64
	err := r.baseQuery(title).Count(&total).Error
	return total, err
}

func (r *gormRepository) list(offset, limit int, title, orderClause string) ([]ExampleItem, error) {
	var items []ExampleItem
	err := r.baseQuery(title).Order(orderClause).Offset(offset).Limit(limit).Find(&items).Error
	return items, err
}

func (r *gormRepository) findByID(id int) (*ExampleItem, error) {
	var item ExampleItem
	if err := r.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *gormRepository) create(item *ExampleItem) error {
	return r.db.Create(item).Error
}

func (r *gormRepository) update(id int, fields map[string]interface{}) error {
	return r.db.Model(&ExampleItem{}).Where("id = ?", id).Updates(fields).Error
}

func (r *gormRepository) delete(id int) error {
	return r.db.Delete(&ExampleItem{}, id).Error
}
