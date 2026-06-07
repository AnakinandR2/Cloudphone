package phone

import "gorm.io/gorm"

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
}

type gormRepository struct{ db *gorm.DB }

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
