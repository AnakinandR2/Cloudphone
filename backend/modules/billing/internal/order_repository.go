package billing

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"manager-backend/framework/apperr"

	"gorm.io/gorm"
)

func itoa(n int) string { return strconv.Itoa(n) }

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

// settle 单事务结算：守卫式标记已付（幂等）→（可选）扣余额+写流水 → 按项发放权益+写流水。
func (r *gormOrderRepository) settle(order *Order, items []OrderItem, deductBalance bool) error {
	now := time.Now()
	return r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&Order{}).
			Where("id = ? AND status = ?", order.ID, OrderPending).
			Updates(map[string]interface{}{"status": OrderPaid, "paid_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return apperr.Conflict("订单状态不可支付（已支付或已取消）")
		}
		if deductBalance {
			ar := tx.Model(&Account{}).
				Where("user_id = ? AND balance_cents >= ?", order.UserID, order.TotalCents).
				Updates(map[string]interface{}{"balance_cents": gorm.Expr("balance_cents - ?", order.TotalCents)})
			if ar.Error != nil {
				return ar.Error
			}
			if ar.RowsAffected == 0 {
				return apperr.Validation("余额不足")
			}
			var acc Account
			if err := tx.Where("user_id = ?", order.UserID).First(&acc).Error; err != nil {
				return err
			}
			if err := tx.Create(&LedgerEntry{
				UserID: order.UserID, Subject: SubjectBalance, Type: LedgerConsume,
				Delta: -order.TotalCents, BalanceAfter: acc.BalanceCents,
				Reason: order.OrderNo, OrderID: order.ID, Operator: "user:" + itoa(int(order.UserID)),
			}).Error; err != nil {
				return err
			}
		}
		entRepo := &gormEntitlementRepository{db: tx}
		for _, it := range items {
			if err := fulfillItem(entRepo, int(order.UserID), it, order.OrderNo, now); err != nil {
				return err
			}
		}
		order.Status = OrderPaid
		order.PaidAt = &now
		return nil
	})
}

// fulfillItem 按订单项品类发放对应资源权益 + 写流水。
func fulfillItem(entRepo entitlementRepository, userID int, it OrderItem, orderNo string, now time.Time) error {
	var subject string
	var qty int64
	var expireAt *time.Time
	switch it.Category {
	case CategoryInstanceFee:
		subject, qty = SubjectInstanceSeat, int64(it.Quantity)
		exp := now.AddDate(0, it.CycleMonths, 0)
		expireAt = &exp
	case CategoryBootPack:
		subject, qty = SubjectBootSeat, int64(it.Quantity)
		exp := now.AddDate(0, it.CycleMonths, 0)
		expireAt = &exp
	case CategoryTimePack:
		subject, qty = SubjectRuntimeMinute, int64(it.Quantity)*60
		expireAt = nil
	default:
		return apperr.Validation("未知商品类别")
	}
	batch := EntitlementBatch{UserID: uint(userID), Subject: subject, Quantity: qty, Source: SourceOrder, SourceRef: orderNo, ExpireAt: expireAt}
	if err := entRepo.createBatch(&batch); err != nil {
		return err
	}
	capacity, err := entRepo.capacity(userID, subject, now)
	if err != nil {
		return err
	}
	return entRepo.insertLedger(&LedgerEntry{
		UserID: uint(userID), Subject: subject, Type: LedgerPurchase,
		Delta: qty, BalanceAfter: capacity, Reason: orderNo, Operator: "user:" + itoa(userID),
	})
}

func isNotFoundOrder(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }

func genOrderNo() string {
	return fmt.Sprintf("BIL%d", time.Now().UnixNano())
}
