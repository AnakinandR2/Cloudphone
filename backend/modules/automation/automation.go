// Package automation 是「自动化脚本任务」业务模块的公开入口。
//
// 对接云手机中台手册 §7：脚本模板（脚本商店 + 用户脚本）、一次性任务 / 周期计划、任务日志与报告。
// 本地镜像脚本/计划/任务，状态与报告走中台实时。模块实现位于 modules/automation/internal，
// 受 Go internal 机制保护；公开包用于在 main 中 blank-import 触发自注册，并对外导出少量门面。
//
// 依赖方向：automation → phone（经 phone 门面做 cpId 归属校验），phone 不反向依赖本模块。
package automation

import automationinternal "manager-backend/modules/automation/internal"

// 公开别名（供 openapi 等跨模块引用返回类型）。
type (
	AutomationTask   = automationinternal.AutomationTask
	TaskReport       = automationinternal.TaskReport
	AutomationScript = automationinternal.AutomationScript
)

// ListUsableScripts 列出某用户可运行的脚本（我的 ∪ 商店，仅启用）。
func ListUsableScripts(userID int) ([]AutomationScript, error) {
	return automationinternal.Service.ListUsableScripts(userID)
}

// RunScript 对指定 cp 下发一次性脚本任务（开放 API 用）。taskName 留空则用脚本名。
func RunScript(userID int, scriptLocalID uint, cpIDs []string) ([]AutomationTask, error) {
	return automationinternal.Service.RunNow(userID, scriptLocalID, cpIDs, "", nil, nil)
}

// TaskDetail 查某脚本任务的状态 + 终态报告（开放 API 用）。
func TaskDetail(userID int, midTaskID int64) (*TaskReport, error) {
	return automationinternal.Service.TaskDetail(userID, midTaskID)
}
