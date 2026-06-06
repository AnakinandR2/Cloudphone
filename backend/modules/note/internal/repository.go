package note

import "gorm.io/gorm"

// repository 笔记持久化。所有读写一律以 userID 约束，从数据层杜绝越权（IDOR）。
type repository interface {
	count(userID int, title string) (int64, error)
	list(userID, offset, limit int, title, orderClause string) ([]Note, error)
	findByID(userID, id int) (*Note, error)
	create(item *Note) error
	update(userID, id int, fields map[string]interface{}) error
	delete(userID, id int) error
}

type gormRepository struct{ db *gorm.DB }

func newRepository(db *gorm.DB) repository { return &gormRepository{db: db} }

// owned 限定为某 user 的笔记。
func (r *gormRepository) owned(userID int) *gorm.DB {
	return r.db.Model(&Note{}).Where("user_id = ?", userID)
}

func (r *gormRepository) count(userID int, title string) (int64, error) {
	q := r.owned(userID)
	if title != "" {
		q = q.Where("title LIKE ?", "%"+title+"%")
	}
	var total int64
	err := q.Count(&total).Error
	return total, err
}

func (r *gormRepository) list(userID, offset, limit int, title, orderClause string) ([]Note, error) {
	q := r.owned(userID)
	if title != "" {
		q = q.Where("title LIKE ?", "%"+title+"%")
	}
	var items []Note
	err := q.Order(orderClause).Offset(offset).Limit(limit).Find(&items).Error
	return items, err
}

func (r *gormRepository) findByID(userID, id int) (*Note, error) {
	var item Note
	if err := r.owned(userID).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *gormRepository) create(item *Note) error { return r.db.Create(item).Error }

func (r *gormRepository) update(userID, id int, fields map[string]interface{}) error {
	return r.owned(userID).Where("id = ?", id).Updates(fields).Error
}

func (r *gormRepository) delete(userID, id int) error {
	return r.owned(userID).Where("id = ?", id).Delete(&Note{}).Error
}
