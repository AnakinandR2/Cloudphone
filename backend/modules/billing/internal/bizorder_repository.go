package billing

import (
	"errors"
	"time"

	"manager-backend/framework"
	"manager-backend/framework/query"

	"gorm.io/gorm"
)

type bizOrderRepository interface {
	create(o *BizOrder, items []BizOrderItem) error
	getOwned(userID, id int) (*BizOrder, []BizOrderItem, error)
	get(id int) (*BizOrder, []BizOrderItem, error)
	// markPaid 以状态守卫 CAS 把订单置为已付：仅当当前状态不是 paid 才更新，
	// 返回受影响行数（0 表示已被并发赢家处理）。用于防重复扣款/重复履约。
	markPaid(id int) (int64, error)
	setGift(id, minutes int) error
	itemsByOrders(orderIDs []uint) (map[uint][]BizOrderItem, error)
	listOwned(userID, offset, limit int, status, bizType string, from, to time.Time) ([]BizOrder, int64, error)
	listAll(offset, limit, userID int, status string) ([]BizOrder, int64, error)
}

type gormBizOrderRepository struct{ db *gorm.DB }

func newBizOrderRepository(db *gorm.DB) bizOrderRepository { return &gormBizOrderRepository{db: db} }

func (r *gormBizOrderRepository) create(o *BizOrder, items []BizOrderItem) error {
	if err := r.db.Transaction(func(tx *gorm.DB) error {
		// 订单号（主键）由 DB 原生自增原子分配——无 SELECT MAX+rand 竞态、无需撞键重试。
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
	}); err != nil {
		return err
	}
	// 下单成功后，把订单号自增序列额外随机跳 0..4 步，使下一张订单号出现不规则跳跃，
	// 避免订单号被顺序枚举遍历他人订单（CP-0037 / #36）。与 user 模块共用同一发号策略。
	framework.ScatterNextID(r.db, BizOrder{}.TableName(), o.ID)
	return nil
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

func (r *gormBizOrderRepository) markPaid(id int) (int64, error) {
	res := r.db.Model(&BizOrder{}).Where("id = ? AND status <> ?", id, BizOrderPaid).
		Update("status", BizOrderPaid)
	return res.RowsAffected, res.Error
}

// setGift 记录订单实际赠送的临时开机时长（履约时写入）。
func (r *gormBizOrderRepository) setGift(id, minutes int) error {
	return r.db.Model(&BizOrder{}).Where("id = ?", id).
		Update("gift_runtime_minutes", minutes).Error
}

// itemsByOrders 批量取多张订单的订单项，按 order_id 分组（订单历史列表随单返回明细）。
func (r *gormBizOrderRepository) itemsByOrders(orderIDs []uint) (map[uint][]BizOrderItem, error) {
	out := map[uint][]BizOrderItem{}
	if len(orderIDs) == 0 {
		return out, nil
	}
	var items []BizOrderItem
	if err := r.db.Where("order_id IN ?", orderIDs).Find(&items).Error; err != nil {
		return nil, err
	}
	for _, it := range items {
		out[it.OrderID] = append(out[it.OrderID], it)
	}
	return out, nil
}

func (r *gormBizOrderRepository) listOwned(userID, offset, limit int, status, bizType string, from, to time.Time) ([]BizOrder, int64, error) {
	q := r.db.Model(&BizOrder{}).Where("user_id = ?", userID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if bizType != "" {
		q = q.Where("biz_type = ?", bizType)
	}
	if !from.IsZero() {
		q = q.Where("created_at >= ?", from)
	}
	if !to.IsZero() {
		q = q.Where("created_at <= ?", to)
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
