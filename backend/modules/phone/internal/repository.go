package phone

import (
	"time"

	"gorm.io/gorm"
)

// repository 云手机档案持久化。前台读写一律以 userID 约束（IDOR 防护）；
// admin* 方法供后台运营查看/删除全量实例（不限属主）。
type repository interface {
	count(userID int, kw, status, tag string) (int64, error)
	list(userID, offset, limit int, kw, status, tag, orderClause string) ([]CloudPhone, error)
	setTags(userID int, ids []int, tagsJSON string) error
	listTagsRaw(userID int) ([]string, error)
	findByID(userID, id int) (*CloudPhone, error)
	create(item *CloudPhone) error
	update(userID, id int, fields map[string]interface{}) error
	delete(userID, id int) error
	// setStatus 按业务主键无条件改状态（worker 收敛异步任务用，不带属主约束）。
	setStatus(phoneID uint, status string) error
	// listByUser 按用户 id 升序列出所有实例（enforcement worker 用）。
	listByUser(userID int) ([]CloudPhone, error)
	// deleteByID 按主键删除（enforcement worker 销毁超量实例用，不带属主约束）。
	deleteByID(id uint) error
	// listNonRecycledByUser 按 created_at 正序列出某用户「非回收态」实例（席位 reconcile 用）。
	listNonRecycledByUser(userID int) ([]CloudPhone, error)
	// listRecycledByUser 按 recycled_at 倒序列出某用户回收站实例（回收站列表用）。
	listRecycledByUser(userID int) ([]CloudPhone, error)
	// findRecycledByID 按属主取一台回收态实例（恢复用，IDOR 防护）。
	findRecycledByID(userID, id int) (*CloudPhone, error)
	// markRecycled 置某实例为回收态（recycled_at=now + reason）。
	markRecycled(phoneID uint, reason string, at time.Time) error
	// restoreRecycled 把回收态实例置回 STOPPED（清空 recycled_at/reason，限本人 + 限回收态）。
	restoreRecycled(userID, id int) error
	// listExpiredRecycled 列出 recycled_at 早于 before 的回收态实例（清理 cron 用，不限属主）。
	listExpiredRecycled(before time.Time) ([]CloudPhone, error)
	// recycledUserIDs 返回当前有回收态实例的去重用户 id（清理巡检范围）。
	recycledUserIDs() ([]int, error)
	// activeOwnerIDs 返回当前有「非回收态」实例的去重用户 id（席位 reconcile 巡检范围）。
	activeOwnerIDs() ([]int, error)
	// 管理侧（不限属主）
	adminCount(kw, status, tag string, userID int) (int64, error)
	adminList(offset, limit int, kw, status, tag string, userID int, orderClause string) ([]CloudPhone, error)
	listAllTagsRaw() ([]string, error)
	adminFindByID(id int) (*CloudPhone, error)
	adminDelete(id int) error
	// 异步任务追踪
	createTask(t *CpTask) error
	dueTasks() ([]CpTask, error)
	updateTask(id uint, status, lastErr string) error
	// ownedCpIDs 返回某用户名下已开通（cpId 非空）的全部 cpId（automation 跨模块归属校验用）。
	ownedCpIDs(userID int) ([]string, error)
	// 运行会话（计费同步/结算/护栏）
	ownersByCpIDs(cpIDs []string) (map[string]uint, error)
	// metaByCpIDs 批量取 cpId → 实例展示信息（名称/状态），供 billing 富化续费列表/费用日志。
	metaByCpIDs(cpIDs []string) (map[string]CpMeta, error)
	upsertRunSession(rs *RunSession) (isNew bool, err error)
	runningSessions() ([]RunSession, error)
	runningSessionCountByUser(userID int) (int64, error)
	sessionsOverlapping(since time.Time) ([]RunSession, error)
}

type gormRepository struct{ db *gorm.DB }

// ownedCpIDs 返回某用户名下已开通（cpId 非空）的全部 cpId。
func (r *gormRepository) ownedCpIDs(userID int) ([]string, error) {
	var rows []CloudPhone
	if err := r.db.Select("cp_id").
		Where("user_id = ? AND cp_id <> ''", userID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, p := range rows {
		if p.CpID != "" {
			out = append(out, p.CpID)
		}
	}
	return out, nil
}

// ownersByCpIDs 批量解析 cpId → 属主 userId（只含我方在册实例）。
func (r *gormRepository) ownersByCpIDs(cpIDs []string) (map[string]uint, error) {
	out := map[string]uint{}
	if len(cpIDs) == 0 {
		return out, nil
	}
	var rows []CloudPhone
	if err := r.db.Select("cp_id, user_id").Where("cp_id IN ?", cpIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, p := range rows {
		if p.CpID != "" {
			out[p.CpID] = p.UserID
		}
	}
	return out, nil
}

// CpMeta 实例展示信息投影（cpId → 名称/状态）。
type CpMeta struct {
	Name   string
	Status string
}

// metaByCpIDs 批量取 cpId → 名称/状态（含回收态，供费用日志/续费列表展示历史实例）。
func (r *gormRepository) metaByCpIDs(cpIDs []string) (map[string]CpMeta, error) {
	out := map[string]CpMeta{}
	if len(cpIDs) == 0 {
		return out, nil
	}
	var rows []CloudPhone
	if err := r.db.Select("cp_id, name, status").Where("cp_id IN ?", cpIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, p := range rows {
		if p.CpID != "" {
			out[p.CpID] = CpMeta{Name: p.Name, Status: p.Status}
		}
	}
	return out, nil
}

// upsertRunSession 按 LogNo upsert：不存在则插入（isNew=true）；已存在则更新可变字段（关机时间/状态等）。
func (r *gormRepository) upsertRunSession(rs *RunSession) (bool, error) {
	var existing RunSession
	err := r.db.Where("log_no = ?", rs.LogNo).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		if err := r.db.Create(rs).Error; err != nil {
			return false, err
		}
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return false, r.db.Model(&RunSession{}).Where("id = ?", existing.ID).Updates(map[string]interface{}{
		"power_off_at":          rs.PowerOffAt,
		"session_status":        rs.SessionStatus,
		"power_off_reason_code": rs.PowerOffReasonCode,
		"vm_uid":                rs.VmUID,
		"user_id":               rs.UserID,
		"synced_at":             rs.SyncedAt,
	}).Error
}

// runningSessions 返回所有运行中（未关机）会话，供护栏统计并发运行台。
func (r *gormRepository) runningSessions() ([]RunSession, error) {
	var out []RunSession
	err := r.db.Where("power_off_at IS NULL").Order("user_id, power_on_at").Find(&out).Error
	return out, err
}

// runningSessionCountByUser 返回某用户当前运行中（未关机）的会话数——开机门禁判并发席位余量用，
// 与护栏 runningSessions() 同源（power_off_at IS NULL）。
func (r *gormRepository) runningSessionCountByUser(userID int) (int64, error) {
	var n int64
	err := r.db.Model(&RunSession{}).Where("power_off_at IS NULL AND user_id = ?", userID).Count(&n).Error
	return n, err
}

// sessionsOverlapping 返回与 [since, now] 有交集的会话（结算窗口用）：
// 仍运行中，或关机时间晚于 since。
func (r *gormRepository) sessionsOverlapping(since time.Time) ([]RunSession, error) {
	var out []RunSession
	err := r.db.Where("power_off_at IS NULL OR power_off_at > ?", since).
		Order("user_id, power_on_at").Find(&out).Error
	return out, err
}

func newRepository(db *gorm.DB) repository { return &gormRepository{db: db} }

func (r *gormRepository) owned(userID int) *gorm.DB {
	return r.db.Model(&CloudPhone{}).Where("user_id = ?", userID)
}

func applyFilter(q *gorm.DB, kw, status, tag string) *gorm.DB {
	if kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR cp_id LIKE ?", like, like)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if tag != "" {
		// tags 以 JSON 数组 [{"name":..,"color":..}] 存储，按标签名做包含匹配。
		q = q.Where("tags LIKE ?", `%"name":"`+tag+`"%`)
	}
	return q
}

func (r *gormRepository) count(userID int, kw, status, tag string) (int64, error) {
	var total int64
	err := applyFilter(r.owned(userID), kw, status, tag).Count(&total).Error
	return total, err
}

func (r *gormRepository) list(userID, offset, limit int, kw, status, tag, orderClause string) ([]CloudPhone, error) {
	var items []CloudPhone
	err := applyFilter(r.owned(userID), kw, status, tag).Order(orderClause).Offset(offset).Limit(limit).Find(&items).Error
	return items, err
}

func (r *gormRepository) findByID(userID, id int) (*CloudPhone, error) {
	var item CloudPhone
	if err := r.owned(userID).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *gormRepository) create(item *CloudPhone) error { return r.db.Create(item).Error }

func (r *gormRepository) update(userID, id int, fields map[string]interface{}) error {
	return r.owned(userID).Where("id = ?", id).Updates(fields).Error
}

func (r *gormRepository) delete(userID, id int) error {
	return r.owned(userID).Where("id = ?", id).Delete(&CloudPhone{}).Error
}

func (r *gormRepository) setStatus(phoneID uint, status string) error {
	return r.db.Model(&CloudPhone{}).Where("id = ?", phoneID).Update("status", status).Error
}

func (r *gormRepository) listByUser(userID int) ([]CloudPhone, error) {
	var items []CloudPhone
	err := r.db.Where("user_id = ?", userID).Order("id ASC").Find(&items).Error
	return items, err
}

func (r *gormRepository) deleteByID(id uint) error {
	return r.db.Where("id = ?", id).Delete(&CloudPhone{}).Error
}

// --- 回收站 + 席位 reconcile ---

func (r *gormRepository) listNonRecycledByUser(userID int) ([]CloudPhone, error) {
	var items []CloudPhone
	err := r.db.Where("user_id = ? AND status <> ?", userID, StatusRecycled).
		Order("created_at ASC, id ASC").Find(&items).Error
	return items, err
}

func (r *gormRepository) listRecycledByUser(userID int) ([]CloudPhone, error) {
	var items []CloudPhone
	err := r.db.Where("user_id = ? AND status = ?", userID, StatusRecycled).
		Order("recycled_at DESC, id DESC").Find(&items).Error
	return items, err
}

func (r *gormRepository) findRecycledByID(userID, id int) (*CloudPhone, error) {
	var item CloudPhone
	if err := r.db.Where("user_id = ? AND id = ? AND status = ?", userID, id, StatusRecycled).
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *gormRepository) markRecycled(phoneID uint, reason string, at time.Time) error {
	return r.db.Model(&CloudPhone{}).Where("id = ?", phoneID).Updates(map[string]interface{}{
		"status":         StatusRecycled,
		"recycled_at":    at,
		"recycle_reason": reason,
	}).Error
}

func (r *gormRepository) restoreRecycled(userID, id int) error {
	return r.db.Model(&CloudPhone{}).
		Where("user_id = ? AND id = ? AND status = ?", userID, id, StatusRecycled).
		Updates(map[string]interface{}{
			"status":         StatusStopped,
			"recycled_at":    nil,
			"recycle_reason": "",
		}).Error
}

func (r *gormRepository) listExpiredRecycled(before time.Time) ([]CloudPhone, error) {
	var items []CloudPhone
	err := r.db.Where("status = ? AND recycled_at IS NOT NULL AND recycled_at < ?", StatusRecycled, before).
		Order("id ASC").Find(&items).Error
	return items, err
}

func (r *gormRepository) recycledUserIDs() ([]int, error) {
	var ids []int
	err := r.db.Model(&CloudPhone{}).Where("status = ?", StatusRecycled).
		Distinct().Pluck("user_id", &ids).Error
	return ids, err
}

func (r *gormRepository) activeOwnerIDs() ([]int, error) {
	var ids []int
	err := r.db.Model(&CloudPhone{}).Where("status <> ?", StatusRecycled).
		Distinct().Pluck("user_id", &ids).Error
	return ids, err
}

// --- 异步任务追踪 ---

func (r *gormRepository) createTask(t *CpTask) error { return r.db.Create(t).Error }

// dueTasks 取所有尚未收敛的任务（pending/running），供 worker 轮询处理。
func (r *gormRepository) dueTasks() ([]CpTask, error) {
	var tasks []CpTask
	err := r.db.Where("status IN ?", []string{TaskPending, TaskRunning}).Order("id ASC").Find(&tasks).Error
	return tasks, err
}

func (r *gormRepository) updateTask(id uint, status, lastErr string) error {
	return r.db.Model(&CpTask{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": status, "last_error": lastErr}).Error
}

// --- 管理侧（不限属主，可按 userID 过滤）---

func (r *gormRepository) adminQuery(kw, status, tag string, userID int) *gorm.DB {
	q := applyFilter(r.db.Model(&CloudPhone{}), kw, status, tag)
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	return q
}

func (r *gormRepository) adminCount(kw, status, tag string, userID int) (int64, error) {
	var total int64
	err := r.adminQuery(kw, status, tag, userID).Count(&total).Error
	return total, err
}

func (r *gormRepository) adminList(offset, limit int, kw, status, tag string, userID int, orderClause string) ([]CloudPhone, error) {
	var items []CloudPhone
	err := r.adminQuery(kw, status, tag, userID).Order(orderClause).Offset(offset).Limit(limit).Find(&items).Error
	return items, err
}

func (r *gormRepository) adminFindByID(id int) (*CloudPhone, error) {
	var item CloudPhone
	if err := r.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *gormRepository) adminDelete(id int) error {
	return r.db.Delete(&CloudPhone{}, id).Error
}

func (r *gormRepository) setTags(userID int, ids []int, tagsJSON string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Model(&CloudPhone{}).Where("user_id = ? AND id IN ?", userID, ids).Update("tags", tagsJSON).Error
}

func (r *gormRepository) listTagsRaw(userID int) ([]string, error) {
	var raws []string
	err := r.db.Model(&CloudPhone{}).Where("user_id = ? AND tags <> '' AND tags IS NOT NULL", userID).Pluck("tags", &raws).Error
	return raws, err
}

// listAllTagsRaw 全用户的 tags 原始串（运营侧标签选项去重用）。
func (r *gormRepository) listAllTagsRaw() ([]string, error) {
	var raws []string
	err := r.db.Model(&CloudPhone{}).Where("tags <> '' AND tags IS NOT NULL").Pluck("tags", &raws).Error
	return raws, err
}
