package billing

import (
	"time"

	"manager-backend/framework/query"

	"gorm.io/gorm"
)

// runtimeChargeRepository runtime_charges + 会话结算进度的数据访问。
type runtimeChargeRepository interface {
	sessionSettled(ref string) (int, error)
	bumpSessionSettled(userID int, instanceID, ref string, settledMinutes int) error
	insertCharge(c *RuntimeCharge) error
	listCharges(userID, offset, limit int) ([]RuntimeCharge, error)
	// listSessions 返回该用户分页的开机会话标识（按最近 charge 时间倒序）+ 总会话数。
	// from/to 非空时按时间段筛选：只取与 [from, to] 有重叠的会话（其 charge 窗口与区间相交）。
	listSessions(userID, offset, limit int, from, to *time.Time) ([]string, int64, error)
	chargesForSessions(userID int, refs []string) ([]RuntimeCharge, error)
}

type gormRuntimeChargeRepository struct{ db *gorm.DB }

func newRuntimeChargeRepository(db *gorm.DB) runtimeChargeRepository {
	return &gormRuntimeChargeRepository{db: db}
}

func (r *gormRuntimeChargeRepository) sessionSettled(ref string) (int, error) {
	var p RuntimeSessionProgress
	err := r.db.Where("run_session_ref = ?", ref).First(&p).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	return p.SettledMinutes, err
}

func (r *gormRuntimeChargeRepository) bumpSessionSettled(userID int, instanceID, ref string, settledMinutes int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var p RuntimeSessionProgress
		err := tx.Where(RuntimeSessionProgress{RunSessionRef: ref}).
			Attrs(RuntimeSessionProgress{UserID: uint(userID), InstanceID: instanceID}).
			FirstOrCreate(&p).Error
		if err != nil {
			return err
		}
		return tx.Model(&RuntimeSessionProgress{}).Where("run_session_ref = ?", ref).
			Update("settled_minutes", settledMinutes).Error
	})
}

func (r *gormRuntimeChargeRepository) insertCharge(c *RuntimeCharge) error {
	return r.db.Create(c).Error
}

func (r *gormRuntimeChargeRepository) listCharges(userID, offset, limit int) ([]RuntimeCharge, error) {
	var out []RuntimeCharge
	err := r.db.Where("user_id = ?", userID).Order("id DESC").Offset(offset).Limit(limit).Find(&out).Error
	return out, err
}

func (r *gormRuntimeChargeRepository) listSessions(userID, offset, limit int, from, to *time.Time) ([]string, int64, error) {
	// 每个会话取一行（最近 charge 时间）用于排序与分页。
	type row struct {
		RunSessionRef string
		MaxID         int64
	}
	// 时间段筛选用 HAVING 作用在会话聚合上：会话窗口 [MIN(window_start), MAX(window_end)]
	// 与 [from, to] 相交 ⇔ MAX(window_end) > from 且 MIN(window_start) < to。
	having := ""
	hargs := []interface{}{}
	if from != nil {
		having = "MAX(window_end) > ?"
		hargs = append(hargs, *from)
	}
	if to != nil {
		if having != "" {
			having += " AND "
		}
		having += "MIN(window_start) < ?"
		hargs = append(hargs, *to)
	}
	// 分组查询构造器（count 与分页各取一份，避免链式副作用）。
	grouped := func() *gorm.DB {
		q := r.db.Model(&RuntimeCharge{}).
			Where("user_id = ?", userID).
			Group("run_session_ref")
		if having != "" {
			q = q.Having(having, hargs...)
		}
		return q
	}

	var total int64
	if err := r.db.Table("(?) as sub", grouped().Select("run_session_ref")).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []row
	ord := query.SafeOrder("max_id", "descending", map[string]bool{"max_id": true}, "max_id DESC")
	err := grouped().
		Select("run_session_ref, MAX(id) as max_id").
		Order(ord).
		Offset(offset).Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	refs := make([]string, 0, len(rows))
	for _, r := range rows {
		refs = append(refs, r.RunSessionRef)
	}
	return refs, total, nil
}

func (r *gormRuntimeChargeRepository) chargesForSessions(userID int, refs []string) ([]RuntimeCharge, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	var out []RuntimeCharge
	err := r.db.Where("user_id = ? AND run_session_ref IN ?", userID, refs).
		Order("id ASC").Find(&out).Error
	return out, err
}
