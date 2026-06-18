package automation

import "gorm.io/gorm"

type repository interface {
	// 脚本
	createScript(s *AutomationScript) error
	updateScript(s *AutomationScript) error
	scriptByID(id uint) (*AutomationScript, error)
	ownedScript(userID int, id uint) (*AutomationScript, error)  // 本人的（store=false）
	usableScript(userID int, id uint) (*AutomationScript, error) // 本人的 或 商店的（建任务用）
	listUserScripts(userID int) ([]AutomationScript, error)      // 我的脚本
	listStoreScripts() ([]AutomationScript, error)               // 商店脚本
	listAllUserScripts() ([]AutomationScript, error)             // 治理：全部用户脚本
	deleteScript(id uint) error

	// 计划
	createPlan(p *AutomationPlan) error
	planByID(id uint) (*AutomationPlan, error)
	ownedPlan(userID int, id uint) (*AutomationPlan, error)
	listUserPlans(userID int) ([]AutomationPlan, error)
	updatePlanStatus(id uint, status string) error
	deletePlan(id uint) error
	activePlans() ([]AutomationPlan, error)

	// 任务
	insertTasks(tasks []AutomationTask) error
	upsertTaskByMidID(t *AutomationTask) error
	taskByMidID(userID int, midID int64) (*AutomationTask, error)
	listUserTasks(userID, offset, limit int, status string) ([]AutomationTask, int64, error)
	nonTerminalTasks() ([]AutomationTask, error)
	updateTaskStatus(midID int64, status, runStart, runEnd string) error
}

type gormRepository struct{ db *gorm.DB }

func newRepository(db *gorm.DB) repository { return &gormRepository{db: db} }

// ---- 脚本 ----

func (r *gormRepository) createScript(s *AutomationScript) error { return r.db.Create(s).Error }
func (r *gormRepository) updateScript(s *AutomationScript) error { return r.db.Save(s).Error }

func (r *gormRepository) scriptByID(id uint) (*AutomationScript, error) {
	var s AutomationScript
	if err := r.db.First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *gormRepository) ownedScript(userID int, id uint) (*AutomationScript, error) {
	var s AutomationScript
	err := r.db.Where("id = ? AND user_id = ? AND store = ?", id, userID, false).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *gormRepository) usableScript(userID int, id uint) (*AutomationScript, error) {
	var s AutomationScript
	err := r.db.Where("id = ? AND (store = ? OR user_id = ?)", id, true, userID).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *gormRepository) listUserScripts(userID int) ([]AutomationScript, error) {
	var out []AutomationScript
	err := r.db.Where("user_id = ? AND store = ?", userID, false).Order("id DESC").Find(&out).Error
	return out, err
}

func (r *gormRepository) listStoreScripts() ([]AutomationScript, error) {
	var out []AutomationScript
	err := r.db.Where("store = ?", true).Order("id DESC").Find(&out).Error
	return out, err
}

func (r *gormRepository) listAllUserScripts() ([]AutomationScript, error) {
	var out []AutomationScript
	err := r.db.Where("store = ?", false).Order("id DESC").Find(&out).Error
	return out, err
}

func (r *gormRepository) deleteScript(id uint) error {
	return r.db.Delete(&AutomationScript{}, id).Error
}

// ---- 计划 ----

func (r *gormRepository) createPlan(p *AutomationPlan) error { return r.db.Create(p).Error }

func (r *gormRepository) planByID(id uint) (*AutomationPlan, error) {
	var p AutomationPlan
	if err := r.db.First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *gormRepository) ownedPlan(userID int, id uint) (*AutomationPlan, error) {
	var p AutomationPlan
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *gormRepository) listUserPlans(userID int) ([]AutomationPlan, error) {
	var out []AutomationPlan
	err := r.db.Where("user_id = ?", userID).Order("id DESC").Find(&out).Error
	return out, err
}

func (r *gormRepository) updatePlanStatus(id uint, status string) error {
	return r.db.Model(&AutomationPlan{}).Where("id = ?", id).Update("status", status).Error
}

func (r *gormRepository) deletePlan(id uint) error {
	return r.db.Delete(&AutomationPlan{}, id).Error
}

func (r *gormRepository) activePlans() ([]AutomationPlan, error) {
	var out []AutomationPlan
	err := r.db.Where("status <> ? AND plan_uid <> ''", PlanFinished).Find(&out).Error
	return out, err
}

// ---- 任务 ----

func (r *gormRepository) insertTasks(tasks []AutomationTask) error {
	if len(tasks) == 0 {
		return nil
	}
	return r.db.Create(&tasks).Error
}

// upsertTaskByMidID 按 mid_task_id upsert：不存在则插入，存在则刷新可变字段（worker 发现派生任务用）。
func (r *gormRepository) upsertTaskByMidID(t *AutomationTask) error {
	var existing AutomationTask
	err := r.db.Where("mid_task_id = ?", t.MidTaskID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.Create(t).Error
	}
	if err != nil {
		return err
	}
	return r.db.Model(&existing).Updates(map[string]any{
		"last_status": t.LastStatus,
		"run_start":   t.RunStart,
		"run_end":     t.RunEnd,
		"task_no":     t.TaskNo,
	}).Error
}

func (r *gormRepository) taskByMidID(userID int, midID int64) (*AutomationTask, error) {
	var t AutomationTask
	err := r.db.Where("mid_task_id = ? AND user_id = ?", midID, userID).First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *gormRepository) listUserTasks(userID, offset, limit int, status string) ([]AutomationTask, int64, error) {
	q := r.db.Model(&AutomationTask{}).Where("user_id = ?", userID)
	if status != "" {
		q = q.Where("last_status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []AutomationTask
	if err := q.Order("id DESC").Offset(offset).Limit(limit).Find(&out).Error; err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *gormRepository) nonTerminalTasks() ([]AutomationTask, error) {
	var out []AutomationTask
	err := r.db.Where("last_status NOT IN ?", []string{"COMPLETED", "FAILED", "CANCELLED"}).
		Limit(500).Find(&out).Error
	return out, err
}

func (r *gormRepository) updateTaskStatus(midID int64, status, runStart, runEnd string) error {
	fields := map[string]any{"last_status": status}
	if runStart != "" {
		fields["run_start"] = runStart
	}
	if runEnd != "" {
		fields["run_end"] = runEnd
	}
	return r.db.Model(&AutomationTask{}).Where("mid_task_id = ?", midID).Updates(fields).Error
}
