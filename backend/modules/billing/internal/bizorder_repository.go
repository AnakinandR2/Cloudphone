package billing

import (
	"errors"

	"manager-backend/framework/query"

	"gorm.io/gorm"
)

type bizOrderRepository interface {
	create(o *BizOrder, items []BizOrderItem) error
	getOwned(userID, id int) (*BizOrder, []BizOrderItem, error)
	get(id int) (*BizOrder, []BizOrderItem, error)
	markPaid(id int) error
	listOwned(userID, offset, limit int, status string) ([]BizOrder, int64, error)
	listAll(offset, limit, userID int, status string) ([]BizOrder, int64, error)
}

type gormBizOrderRepository struct{ db *gorm.DB }

func newBizOrderRepository(db *gorm.DB) bizOrderRepository { return &gormBizOrderRepository{db: db} }

func (r *gormBizOrderRepository) create(o *BizOrder, items []BizOrderItem) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(o).Error; err != nil {
			return err
		}
		for i := range items {
			items[i].OrderID = o.ID
		}
		if len(items) > 0 {
			return tx.Create(&items).Error
		}
		return nil
	})
}

func (r *gormBizOrderRepository) getOwned(userID, id int) (*BizOrder, []BizOrderItem, error) {
	var o BizOrder
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&o).Error; err != nil {
		return nil, nil, err
	}
	var items []BizOrderItem
	if err := r.db.Where("order_id = ?", o.ID).Find(&items).Error; err != nil {
		return nil, nil, err
	}
	return &o, items, nil
}

func (r *gormBizOrderRepository) get(id int) (*BizOrder, []BizOrderItem, error) {
	var o BizOrder
	if err := r.db.Where("id = ?", id).First(&o).Error; err != nil {
		return nil, nil, err
	}
	var items []BizOrderItem
	if err := r.db.Where("order_id = ?", o.ID).Find(&items).Error; err != nil {
		return nil, nil, err
	}
	return &o, items, nil
}

func (r *gormBizOrderRepository) markPaid(id int) error {
	return r.db.Model(&BizOrder{}).Where("id = ?", id).
		Update("status", BizOrderPaid).Error
}

func (r *gormBizOrderRepository) listOwned(userID, offset, limit int, status string) ([]BizOrder, int64, error) {
	q := r.db.Model(&BizOrder{}).Where("user_id = ?", userID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	ord := query.SafeOrder("id", "descending", map[string]bool{"id": true, "created_at": true}, "id DESC")
	var out []BizOrder
	err := q.Order(ord).Offset(offset).Limit(limit).Find(&out).Error
	return out, total, err
}

func (r *gormBizOrderRepository) listAll(offset, limit, userID int, status string) ([]BizOrder, int64, error) {
	q := r.db.Model(&BizOrder{})
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	ord := query.SafeOrder("id", "descending", map[string]bool{"id": true, "created_at": true}, "id DESC")
	var out []BizOrder
	err := q.Order(ord).Offset(offset).Limit(limit).Find(&out).Error
	return out, total, err
}

func isNotFoundBizOrder(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
