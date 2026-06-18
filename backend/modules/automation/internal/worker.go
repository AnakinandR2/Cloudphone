package automation

import "context"

// syncTasks 是同步 worker 的一轮（仿 billing run-session）：
//  1. 对每个活跃 plan 按 planUid 拉派生任务，upsert 进 automation_tasks；
//  2. 对所有非终态任务批量刷新状态 / 运行时间。
//
// 全程 best-effort：单个 plan / 单批查询失败只跳过，不中断整轮。
func (s *serviceImpl) syncTasks(ctx context.Context) {
	if s.ops == nil {
		return
	}
	s.discoverPlanTasks(ctx)
	s.refreshTaskStatuses(ctx)
}

// discoverPlanTasks 按 planUid 拉中台派生任务，upsert 进本地索引（带上 owner/script/plan 关联）。
func (s *serviceImpl) discoverPlanTasks(ctx context.Context) {
	plans, err := s.repo.activePlans()
	if err != nil {
		return
	}
	for i := range plans {
		p := plans[i]
		if p.PlanUID == "" {
			continue
		}
		vos, err := s.ops.TasksByPlan(ctx, p.PlanUID)
		if err != nil {
			continue
		}
		for _, vo := range vos {
			_ = s.repo.upsertTaskByMidID(&AutomationTask{
				UserID:        p.UserID,
				MidTaskID:     vo.ID,
				TaskNo:        vo.TaskID,
				ScriptLocalID: p.ScriptLocalID,
				ScriptName:    p.ScriptName,
				PlanLocalID:   p.ID,
				CpID:          vo.CpID,
				TaskName:      vo.TaskName,
				Trigger:       TriggerPlan,
				LastStatus:    vo.TaskStatus,
				RunStart:      vo.RunStartTime,
				RunEnd:        vo.RunEndTime,
			})
		}
	}
}

// refreshTaskStatuses 批量刷新非终态任务的状态。
func (s *serviceImpl) refreshTaskStatuses(ctx context.Context) {
	tasks, err := s.repo.nonTerminalTasks()
	if err != nil || len(tasks) == 0 {
		return
	}
	ids := make([]int64, 0, len(tasks))
	for _, t := range tasks {
		ids = append(ids, t.MidTaskID)
	}
	vos, err := s.ops.TaskStatuses(ctx, ids)
	if err != nil {
		return
	}
	for _, vo := range vos {
		_ = s.repo.updateTaskStatus(vo.ID, vo.TaskStatus, vo.RunStartTime, vo.RunEndTime)
	}
}
