package phone

import (
	"context"
	"strings"
	"time"

	"manager-backend/framework/apperr"
	"manager-backend/framework/midplat"
)

// 自动化脚本「最小闭环」（手册 §7）：为某台云手机下发一个 hello-world Lua 任务并查结果，
// 用于打通「上传模板 → 创建任务 → 轮询状态 → 取报告」整条链路、验证能力与接口。

const (
	helloScriptName    = "glory_hello_world"     // §7.5.2 按名查 scriptId 用
	helloScriptFile    = "glory_hello_world.lua" // 上传时的文件名
	helloScriptVersion = "1.0.0"
	helloScriptDesc    = "Glory 平台连通性测试脚本（hello world）"
)

// helloScriptLua 是最基本的示例脚本：只用核心能力 log + report_result，
// 避免依赖具体设备/应用，确保任何在线云手机都能跑通。
const helloScriptLua = `log("hello world from glory cloud phone")
report_result('{"msg":"hello world from glory","ok":true}')
`

// 任务终态（手册 §7.6.2 taskStatus 枚举）。
var scriptTerminalStatus = map[string]bool{
	"COMPLETED": true,
	"FAILED":    true,
	"CANCELLED": true,
}

// ScriptRunResult 是「下发任务」的受理结果，前端据此轮询。
type ScriptRunResult struct {
	TaskID int64  `json:"task_id"`
	TaskNo string `json:"task_no"`
}

// ScriptTaskDetail 是任务状态 + 报告的合并视图（前端轮询展示）。
type ScriptTaskDetail struct {
	TaskID        int64  `json:"task_id"`
	TaskNo        string `json:"task_no"`
	Status        string `json:"status"`      // 英文枚举（§7.6.2）
	StatusDesc    string `json:"status_desc"` // 中文描述
	ExecResult    *int   `json:"exec_result"` // 0=失败 / 1=成功 / null=未跑
	Terminal      bool   `json:"terminal"`
	RunDurationMs int64  `json:"run_duration_ms"`
	RunLog        string `json:"run_log"`        // 可读日志（优先 runLogList，回退原始 runLog）
	Result        string `json:"result"`         // 脚本 report_result 上报的内容（从日志抽取）
	ScreenshotURL string `json:"screenshot_url"` // 任务截图
}

// ensureHelloTemplate 确保 hello-world 模板已存在，返回其 scriptId；不存在则自助上传后再取回。
func (s *serviceImpl) ensureHelloTemplate(ctx context.Context) (int64, error) {
	id, err := s.ops.ScriptTemplateID(ctx, helloScriptName)
	if err != nil {
		return 0, err
	}
	if id > 0 {
		return id, nil
	}
	// 模板不存在 → 自助上传（§7.5.1 multipart），再查回 scriptId。
	if err := s.ops.UploadScriptTemplate(ctx, helloScriptFile, helloScriptName, helloScriptVersion, helloScriptDesc, []byte(helloScriptLua)); err != nil {
		return 0, apperr.Internal("上传示例脚本模板失败：" + err.Error())
	}
	id, err = s.ops.ScriptTemplateID(ctx, helloScriptName)
	if err != nil {
		return 0, err
	}
	if id == 0 {
		return 0, apperr.Internal("示例脚本模板上传后仍查不到，请稍后重试")
	}
	return id, nil
}

// RunHelloScript 给某台运行中的云手机下发一个 hello-world 脚本任务。
func (s *serviceImpl) RunHelloScript(userID, id int) (*ScriptRunResult, error) {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()

	// 脚本在设备内运行，须已开机；门禁与展示同源，UNKNOWN 拒绝。
	live := s.liveStatus(ctx, p.CpID)
	if live == StatusUnknown {
		return nil, apperr.Validation("状态未知，请稍后重试")
	}
	if live != StatusRunning {
		return nil, apperr.Validation("请先开机后再运行脚本")
	}

	scriptID, err := s.ensureHelloTemplate(ctx)
	if err != nil {
		return nil, err
	}

	// publishTime：立即执行。中台无时区，按北京时间解析，故用 UTC+8 当前时刻。
	publishTime := time.Now().UTC().Add(8 * time.Hour).Format("2006-01-02T15:04:05")
	taskID, taskNo, err := s.ops.CreateScriptTask(ctx, scriptID, "Glory 连通性测试", p.CpID, publishTime)
	if err != nil {
		return nil, apperr.Internal("创建脚本任务失败：" + err.Error())
	}
	return &ScriptRunResult{TaskID: taskID, TaskNo: taskNo}, nil
}

// ScriptTaskStatus 查某个脚本任务的状态；到达终态时合并报告（日志/截图/结果）。
func (s *serviceImpl) ScriptTaskStatus(userID, id int, taskID int64) (*ScriptTaskDetail, error) {
	// 校验云手机归属（防止越权查别人的任务上下文）。
	if _, err := s.resolveCp(userID, id); err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()

	vo, err := s.ops.ScriptTaskStatus(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if vo == nil {
		return nil, apperr.NotFound("任务不存在")
	}
	out := &ScriptTaskDetail{
		TaskID:        taskID,
		TaskNo:        vo.TaskID,
		Status:        vo.TaskStatus,
		StatusDesc:    vo.TaskStatusDesc,
		ExecResult:    vo.ExecResult,
		Terminal:      scriptTerminalStatus[vo.TaskStatus],
		RunDurationMs: vo.RunDuration,
	}
	// 终态再取报告（未跑完报告多为空，避免无谓调用）。
	if out.Terminal {
		if rep, rerr := s.ops.ScriptTaskReport(ctx, taskID); rerr == nil && rep != nil {
			out.ScreenshotURL = rep.FirstScreenshot()
			out.RunLog = renderScriptLog(rep)
			out.Result = extractScriptResult(rep.RunLog)
		}
	}
	return out, nil
}

// renderScriptLog 把报告日志渲染为可读文本：优先用解析后的 runLogList，回退原始 runLog。
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

// extractScriptResult 从原始日志里抽取 report_result 上报的内容（包在 #RESULT#...#RESULT# 之间）。
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
