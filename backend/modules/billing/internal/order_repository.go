package billing

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type orderRepository interface {
	createOrder(order *Order, items []OrderItem) error
	getByID(id int) (*Order, error)
	getOwned(userID, id int) (*Order, error)
	listItems(orderID int) ([]OrderItem, error)
	listOrders(userID, offset, limit int, status string) ([]Order, int64, error)
	adminListOrders(offset, limit int, userID int, status string) ([]Order, int64, error)
	settle(order *Order, items []OrderItem, deductBalance bool) error
}

type gormOrderRepository struct{ db *gorm.DB }

func newOrderRepository(db *gorm.DB) orderRepository { return &gormOrderRepository{db: db} }

func (r *gormOrderRepository) createOrder(order *Order, items []OrderItem) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		for i := range items {
			items[i].OrderID = order.ID
			if err := tx.Create(&items[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *gormOrderRepository) getByID(id int) (*Order, error) {
	var o Order
	if err := r.db.Where("id = ?", id).First(&o).Error; err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *gormOrderRepository) getOwned(userID, id int) (*Order, error) {
	var o Order
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&o).Error; err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *gormOrderRepository) listItems(orderID int) ([]OrderItem, error) {
	var items []OrderItem
	err := r.db.Where("order_id = ?", orderID).Order("id ASC").Find(&items).Error
	return items, err
}

func (r *gormOrderRepository) listOrders(userID, offset, limit int, status string) ([]Order, int64, error) {
	q := r.db.Model(&Order{}).Where("user_id = ?", userID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []Order
	err := q.Order("id DESC").Offset(offset).Limit(limit).Find(&items).Error
	return items, total, err
}

func (r *gormOrderRepository) adminListOrders(offset, limit int, userID int, status string) ([]Order, int64, error) {
	q := r.db.Model(&Order{})
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
	var items []Order
	err := q.Order("id DESC").Offset(offset).Limit(limit).Find(&items).Error
	return items, total, err
}

func (r *gormOrderRepository) settle(order *Order, items []OrderItem, deductBalance bool) error {
	return errors.New("not implemented") // TODO(Task 2)
}

func isNotFoundOrder(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }

func genOrderNo() string {
	return fmt.Sprintf("BIL%d", time.Now().UnixNano())
}
