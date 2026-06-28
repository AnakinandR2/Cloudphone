package automation

import (
	"strings"

	"manager-backend/framework/apperr"
	"manager-backend/framework/midplat"
	"manager-backend/modules/phone"
)

// RunNow 一次性立即运行：把脚本下发到指定（本人拥有的）云手机。
// params 为共用参数；perPhone 为逐台覆盖（cpId → 覆盖值），均按脚本 params_schema 校验。
func (s *serviceImpl) RunNow(userID int, scriptLocalID uint, cpIDs []string, taskName string, params map[string]any, perPhone map[string]map[string]any) ([]AutomationTask, error) {
	if err := s.requireOps(); err != nil {
		return nil, err
	}
	script, err := s.repo.usableScript(userID, scriptLocalID)
	if err != nil {
		return nil, apperr.NotFound("脚本不存在或不可用")
	}
	if script.ScriptID == 0 {
		return nil, apperr.Validation("脚本尚未就绪，请稍后重试")
	}
	valid, err := s.filterOwned(userID, cpIDs)
	if err != nil {
		return nil, err
	}
	if len(valid) == 0 {
		return nil, apperr.Validation("未选择有效的云手机（需本人拥有且已开通）")
	}
	if taskName == "" {
		taskName = script.Name
	}

	// 按 schema 校验并构造每台的 scriptParams：默认共用一份，逐台覆盖时单独构造。
	// schema 现 parse 自脚本顶部注释（唯一真源）。
	_, specs, err := deriveSchema(script.LuaContent, "")
	if err != nil {
		return nil, err
	}
	sharedEff, err := buildParams(specs, params)
	if err != nil {
		return nil, err
	}
	sharedJSON := serializeParams(specs, sharedEff)
	paramsByCp := make(map[string]string, len(valid))
	for _, cp := range valid {
		ov := perPhone[cp]
		if len(ov) == 0 {
			paramsByCp[cp] = sharedJSON
			continue
		}
		eff, berr := buildParams(specs, mergeParams(params, ov))
		if berr != nil {
			return nil, apperr.Validation("云手机 " + cp + "：" + berr.Error())
		}
		paramsByCp[cp] = serializeParams(specs, eff)
	}

	ctx, cancel := opCtx()
	defer cancel()
	created, err := s.ops.CreateTasks(ctx, script.ScriptID, taskName, beijingNow(), valid, paramsByCp)
	if err != nil {
		return nil, apperr.Internal("创建任务失败：" + err.Error())
	}
	rows := make([]AutomationTask, 0, len(created))
	for _, c := range created {
		rows = append(rows, AutomationTask{
			UserID:        uint(userID),
			MidTaskID:     c.ID,
			TaskNo:        c.TaskID,
			ScriptLocalID: script.ID,
			ScriptName:    script.Name,
			CpID:          c.CpID,
			TaskName:      taskName,
			Trigger:       TriggerManual,
			LastStatus:    "WAITING_PUBLISH",
		})
	}
	if err := s.repo.insertTasks(rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// filterOwned 过滤出本人拥有且已开通的 cpId（经 phone 门面校验归属）。
func (s *serviceImpl) filterOwned(userID int, cpIDs []string) ([]string, error) {
	owned, err := phone.OwnedCpIDs(userID)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(owned))
	for _, c := range owned {
		set[c] = true
	}
	out := make([]string, 0, len(cpIDs))
	for _, c := range cpIDs {
		if set[c] {
			out = append(out, c)
		}
	}
	return out, nil
}

// LogList 任务日志分页（本地索引，owner 隔离）。
func (s *serviceImpl) LogList(userID, page, size int, status string) ([]AutomationTask, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}
	return s.repo.listUserTasks(userID, (page-1)*size, size, status)
}

// TaskDetail 查任务状态 + 终态报告（状态/报告走中台实时）。
func (s *serviceImpl) TaskDetail(userID int, midTaskID int64) (*TaskReport, error) {
	if err := s.requireOps(); err != nil {
		return nil, err
	}
	if _, err := s.repo.taskByMidID(userID, midTaskID); err != nil {
		return nil, apperr.NotFound("任务不存在或不属于你")
	}
	ctx, cancel := opCtx()
	defer cancel()

	vos, err := s.ops.TaskStatuses(ctx, []int64{midTaskID})
	if err != nil {
		return nil, err
	}
	out := &TaskReport{MidTaskID: midTaskID}
	if len(vos) > 0 {
		vo := vos[0]
		out.TaskNo = vo.TaskID
		out.Status = vo.TaskStatus
		out.StatusDesc = vo.TaskStatusDesc
		out.Terminal = terminalTaskStatus[vo.TaskStatus]
		out.RunDurationMs = vo.RunDuration
		// 顺手回写本地状态。
		_ = s.repo.updateTaskStatus(midTaskID, vo.TaskStatus, vo.RunStartTime, vo.RunEndTime)
	}
	if out.Terminal {
		if rep, rerr := s.ops.TaskReport(ctx, midTaskID); rerr == nil && rep != nil {
			out.ScreenshotURL = rep.FirstScreenshot()
			out.RunLog = renderScriptLog(rep)
			out.Result = extractScriptResult(rep.RunLog)
			if rep.RunDurationMs > 0 {
				out.RunDurationMs = rep.RunDurationMs
			}
		}
	}
	return out, nil
}

// renderScriptLog 渲染可读日志：优先 runLogList，回退原始 runLog。
func renderScriptLog(rep *midplat.ScriptTaskReport) string {
	if len(rep.RunLogList) > 0 {
		var b strings.Builder
		for _, l := range rep.RunLogList {
			if l.Timestamp != "" {
				b.WriteString(l.Timestamp)
				b.WriteString(" ")
			}
			if l.Level != "" {
				b.WriteString("[" + l.Level + "] ")
			}
			b.WriteString(l.Content)
			b.WriteString("\n")
		}
		return b.String()
	}
	return rep.RunLog
}

// extractScriptResult 抽取 report_result 上报内容（包在 #RESULT#...#RESULT# 之间）。
func extractScriptResult(runLog string) string {
	const marker = "#RESULT#"
	start := strings.Index(runLog, marker)
	if start < 0 {
		return ""
	}
	rest := runLog[start+len(marker):]
	end := strings.Index(rest, marker)
	if end < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:end])
}
