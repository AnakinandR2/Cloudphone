package automation

import (
	"manager-backend/framework/apperr"
	"manager-backend/framework/midplat"
)

// PlanInput 是新建周期计划的入参。
type PlanInput struct {
	ScriptLocalID uint
	Name          string
	Frequency     string // INTERVAL / DAILY
	IntervalValue int    // INTERVAL：间隔分钟
	ExecutionTime string // DAILY：HH:mm:ss
	StartTime     string
	EndTime       string
	CpIDs         []string
}

// ListPlans 我的周期计划。
func (s *serviceImpl) ListPlans(userID int) ([]AutomationPlan, error) {
	return s.repo.listUserPlans(userID)
}

// CreatePlan 创建周期计划：校验脚本可用 + cpId 归属 → 调中台 → 落本地镜像。
func (s *serviceImpl) CreatePlan(userID int, in PlanInput) (*AutomationPlan, error) {
	if err := s.requireOps(); err != nil {
		return nil, err
	}
	if in.Frequency != "INTERVAL" && in.Frequency != "DAILY" {
		return nil, apperr.BadRequest("频率只能是 INTERVAL / DAILY")
	}
	if in.Frequency == "INTERVAL" && in.IntervalValue <= 0 {
		return nil, apperr.BadRequest("间隔分钟须大于 0")
	}
	if in.Frequency == "DAILY" && in.ExecutionTime == "" {
		return nil, apperr.BadRequest("每日计划须指定执行时间")
	}
	script, err := s.repo.usableScript(userID, in.ScriptLocalID)
	if err != nil {
		return nil, apperr.NotFound("脚本不存在或不可用")
	}
	if script.ScriptID == 0 {
		return nil, apperr.Validation("脚本尚未就绪，请稍后重试")
	}
	valid, err := s.filterOwned(userID, in.CpIDs)
	if err != nil {
		return nil, err
	}
	if len(valid) == 0 {
		return nil, apperr.Validation("未选择有效的云手机（需本人拥有且已开通）")
	}

	// 中台 §7.3 实测 startTime/endTime 为【必填】（文档标可选，实测缺失报「开始/结束时间不能为空」）。
	// 不在后端兜默认值（用户不可见、易误解）；由前端显式提供（带默认填充、可改），缺失则明确报错。
	if in.StartTime == "" || in.EndTime == "" {
		return nil, apperr.BadRequest("请填写计划的开始与结束时间")
	}

	ctx, cancel := opCtx()
	defer cancel()
	created, err := s.ops.CreatePlan(ctx, midplat.CreateScriptPlanRequest{
		ScriptID:           script.ScriptID,
		PlanName:           in.Name,
		ExecutionFrequency: in.Frequency,
		IntervalValue:      in.IntervalValue,
		ExecutionTime:      in.ExecutionTime,
		StartTime:          in.StartTime,
		EndTime:            in.EndTime,
		CpIDList:           valid,
	})
	if err != nil {
		return nil, apperr.Internal("创建计划失败：" + err.Error())
	}
	status := created.PlanStatus
	if status == "" {
		status = PlanNotStarted
	}
	plan := &AutomationPlan{
		UserID:        uint(userID),
		PlanID:        created.ID,
		PlanUID:       created.PlanUID,
		ScriptLocalID: script.ID,
		ScriptName:    script.Name,
		Name:          in.Name,
		Frequency:     in.Frequency,
		IntervalValue: in.IntervalValue,
		ExecutionTime: in.ExecutionTime,
		StartTime:     in.StartTime,
		EndTime:       in.EndTime,
		CpIDsJSON:     marshalStrList(valid),
		Status:        status,
	}
	if err := s.repo.createPlan(plan); err != nil {
		return nil, err
	}
	plan.CpIDs = valid
	return plan, nil
}

// planAction 启动/暂停/删除：校验归属 → 调中台 → 改本地。
func (s *serviceImpl) planAction(userID int, id uint, action string) error {
	if err := s.requireOps(); err != nil {
		return err
	}
	plan, err := s.repo.ownedPlan(userID, id)
	if err != nil {
		return apperr.NotFound("计划不存在或不属于你")
	}
	ctx, cancel := opCtx()
	defer cancel()
	switch action {
	case "start":
		if err := s.ops.StartPlan(ctx, plan.PlanID); err != nil {
			return apperr.Internal("启动计划失败：" + err.Error())
		}
		return s.repo.updatePlanStatus(id, PlanEnabling)
	case "pause":
		if err := s.ops.PausePlan(ctx, plan.PlanID); err != nil {
			return apperr.Internal("暂停计划失败：" + err.Error())
		}
		return s.repo.updatePlanStatus(id, PlanPaused)
	case "delete":
		if err := s.ops.DeletePlan(ctx, plan.PlanID); err != nil {
			return apperr.Internal("删除计划失败：" + err.Error())
		}
		return s.repo.deletePlan(id)
	}
	return apperr.BadRequest("未知操作")
}

func (s *serviceImpl) StartPlan(userID int, id uint) error { return s.planAction(userID, id, "start") }
func (s *serviceImpl) PausePlan(userID int, id uint) error { return s.planAction(userID, id, "pause") }
func (s *serviceImpl) DeletePlan(userID int, id uint) error {
	return s.planAction(userID, id, "delete")
}
