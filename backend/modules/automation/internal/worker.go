package automation

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

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

// discoverPlanTasks 并行按 planUid 拉中台派生任务，串行 upsert 进本地索引。
// goroutine 内构建好 AutomationTask 切片，用 mutex 汇总，主 goroutine 串行写库。
func (s *serviceImpl) discoverPlanTasks(ctx context.Context) {
	plans, err := s.repo.activePlans()
	if err != nil {
		return
	}

	var (
		mu       sync.Mutex
		allTasks []*AutomationTask
	)

	eg, ctx := errgroup.WithContext(ctx)
	for i := range plans {
		p := plans[i]
		if p.PlanUID == "" {
			continue
		}
		eg.Go(func() error {
			vos, err := s.ops.TasksByPlan(ctx, p.PlanUID)
			if err != nil {
				return nil // best-effort：单 plan 失败跳过
			}
			tasks := make([]*AutomationTask, 0, len(vos))
			for _, vo := range vos {
				tasks = append(tasks, &AutomationTask{
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
			mu.Lock()
			allTasks = append(allTasks, tasks...)
			mu.Unlock()
			return nil
		})
	}
	_ = eg.Wait()

	for _, t := range allTasks {
		_ = s.repo.upsertTaskByMidID(t)
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
