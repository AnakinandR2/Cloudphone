// auto_script.go：自动化脚本任务（手册 §7）。
//
// 最小闭环用到的 4 类接口：
//   - §7.5.2 脚本模板分页   POST /api/automation/templates/page        —— 按名查 scriptId
//   - §7.5.1 上传 lua 模板   POST /api/automation/templates/lua/batch    —— multipart 自助上传
//   - §7.2   创建一次性任务  POST /autoscript/task/create-scheduled       —— 下发到某台 cp
//   - §7.6.2 按 ids 查任务   POST /autoscript/task/query-by-ids           —— 轮询状态
//   - §7.6.8 查任务报告      POST /autoscript/task/report                 —— 日志 / 截图 / 结果
//
// 注意：这些端点前缀是 /open/api/autoScript/...（与 vendor/v1 不同），doc §7 路径已含 /open。
package midplat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	neturl "net/url"
	"strconv"
)

// ScriptTemplate 是 §7.5.2 模板分页返回的关键字段（只取闭环所需）。
type ScriptTemplate struct {
	ID            int64  `json:"id"` // ⭐ scriptId（§7.2 创建任务入参）
	Name          string `json:"name"`
	Description   string `json:"description"`
	ScriptVersion string `json:"scriptVersion"`
	IsPublic      int    `json:"isPublic"`
	Status        int    `json:"status"` // 0=禁用 / 1=可用
}

// ListScriptTemplatesByName 按名称分页查询脚本模板（当前租户可见），返回首页列表。
func (c *Client) ListScriptTemplatesByName(ctx context.Context, name string) ([]ScriptTemplate, error) {
	const path = "/open/api/autoScript/api/automation/templates/page"
	body := map[string]any{"page": 1, "pageSize": 50, "name": name}
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, body)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var page struct {
		Data []ScriptTemplate `json:"data"`
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, err
	}
	return page.Data, nil
}

// scriptTemplateMeta 是批量上传 §7.5.1 的 meta 数组项。
type scriptTemplateMeta struct {
	Name          string `json:"name"`
	ScriptVersion string `json:"scriptVersion"`
	Description   string `json:"description"`
	IsPublic      int    `json:"isPublic"`
}

// UploadLuaTemplate 调用 §7.5.1 批量上传端点上传单个 Lua 脚本模板（multipart：files + meta）。
// 上传成功不直接返回 scriptId，需再调 ListScriptTemplatesByName 取回。
func (c *Client) UploadLuaTemplate(ctx context.Context, fileName, name, version, description string, lua []byte) error {
	const path = "/open/api/autoScript/api/automation/templates/lua/batch"

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("files", fileName)
	if err != nil {
		return fmt.Errorf("midplat: 创建 multipart 文件字段失败: %w", err)
	}
	if _, err := fw.Write(lua); err != nil {
		return fmt.Errorf("midplat: 写入 lua 内容失败: %w", err)
	}
	metaJSON, _ := json.Marshal([]scriptTemplateMeta{{
		Name: name, ScriptVersion: version, Description: description, IsPublic: 0,
	}})
	if err := w.WriteField("meta", string(metaJSON)); err != nil {
		return fmt.Errorf("midplat: 写入 meta 字段失败: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("midplat: 关闭 multipart writer 失败: %w", err)
	}
	_, err = c.doRawWithQuery(ctx, http.MethodPost, path, nil, &buf, w.FormDataContentType())
	return err
}

// CreateScriptTaskItem 是 §7.2 taskList 的单条（一台 cp 一条）。
type CreateScriptTaskItem struct {
	CpID         string `json:"cpId"`
	PublishTime  string `json:"publishTime"`            // LocalDateTime，无时区，北京时间
	ScriptParams string `json:"scriptParams,omitempty"` // 单条自定义参数（JSON 字符串）
}

// CreateScriptTaskRequest 是 §7.2 create-scheduled 的请求体。
type CreateScriptTaskRequest struct {
	ScriptID int64                  `json:"scriptId"`
	TaskName string                 `json:"taskName"`
	Remark   string                 `json:"remark,omitempty"`
	TaskList []CreateScriptTaskItem `json:"taskList"`
}

// ScriptTaskCreated 是 §7.2 返回（每个 cpId 一条）。
type ScriptTaskCreated struct {
	ID              int64  `json:"id"` // ⭐ 任务主键（Long），后续查询/取消/删除入参
	TaskID          string `json:"taskId"`
	CpID            string `json:"cpId"`
	PlanPublishTime string `json:"planPublishTime"`
}

// CreateScriptTasks 调用 §7.2 创建一次性脚本任务。
func (c *Client) CreateScriptTasks(ctx context.Context, req CreateScriptTaskRequest) ([]ScriptTaskCreated, error) {
	const path = "/open/api/autoScript/autoscript/task/create-scheduled"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []ScriptTaskCreated
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ScriptTaskVO 是 §7.6.2 query-by-ids 返回的任务视图（只取闭环所需）。
type ScriptTaskVO struct {
	ID             int64  `json:"id"`
	TaskID         string `json:"taskId"`
	TaskName       string `json:"taskName"`
	CpID           string `json:"cpId"`
	TaskStatus     string `json:"taskStatus"`     // WAITING_PUBLISH/.../COMPLETED/FAILED/CANCELLED
	TaskStatusDesc string `json:"taskStatusDesc"` // 中文描述
	ExecResult     *int   `json:"execResult"`     // 0=失败 / 1=成功 / null=未跑
	RunDuration    int64  `json:"runDuration"`    // 毫秒
	RunStartTime   string `json:"runStartTime"`
	RunEndTime     string `json:"runEndTime"`
}

// QueryScriptTasksByIDs 调用 §7.6.2 按 ids（Long 数组）批量查任务状态。
func (c *Client) QueryScriptTasksByIDs(ctx context.Context, ids []int64) ([]ScriptTaskVO, error) {
	const path = "/open/api/autoScript/autoscript/task/query-by-ids"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, map[string]any{"ids": ids})
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []ScriptTaskVO
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ScriptTaskReportLog 是 §7.6.8 报告里解析后的日志行（level 实测可能不存在）。
type ScriptTaskReportLog struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Content   string `json:"content"`
}

// ScriptTaskReport 是 §7.6.8 任务报告（截图 + 日志 + 结果）。
//
// ⚠️ 实测修正（真机 dump）：
//   - screenshotUrl 实测是【数组】（["https://…"]），不是 string——类型错会导致整个 report
//     反序列化失败、字段静默全空。这里用 []string。
//   - taskStatus 实测返回【中文】（"已完成"），与 §7.6.2 的英文枚举分裂——终态判定别用它，
//     只取本结构的日志/截图/结果，状态以 §7.6.2 query-by-ids 的英文为准。
//   - 实测含 runDurationMs（毫秒），比秒级 runDuration 稳，优先用。
//   - runLogList[].level 实测可能缺失，渲染需容错。
type ScriptTaskReport struct {
	TaskID           string                `json:"taskId"`
	TaskStatus       string                `json:"taskStatus"` // 实测中文，勿用于终态判定
	PlanName         string                `json:"planName"`
	RunDuration      int64                 `json:"runDuration"`   // 秒（不稳）
	RunDurationMs    int64                 `json:"runDurationMs"` // 毫秒（实测存在，优先）
	StartTime        string                `json:"startTime"`
	EndTime          string                `json:"endTime"`
	ScreenshotURL    []string              `json:"screenshotUrl"` // 实测数组
	ScreenshotBase64 string                `json:"screenshotBase64"`
	RunLog           string                `json:"runLog"`
	RunLogList       []ScriptTaskReportLog `json:"runLogList"`
}

// FirstScreenshot 返回首张截图 URL（无则空串）。
func (r *ScriptTaskReport) FirstScreenshot() string {
	if len(r.ScreenshotURL) > 0 {
		return r.ScreenshotURL[0]
	}
	return ""
}

// GetScriptTaskReport 调用 §7.6.8 查任务报告。任务未跑完时多数字段为空。
func (c *Client) GetScriptTaskReport(ctx context.Context, id int64) (*ScriptTaskReport, error) {
	const path = "/open/api/autoScript/autoscript/task/report"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, map[string]any{"id": id})
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return &ScriptTaskReport{}, nil
	}
	var out ScriptTaskReport
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ===== §7.5.2 模板分页（带过滤）/ 启停 / 删除 =====

// ListScriptTemplatesRequest 是 §7.5.2 模板分页入参。
type ListScriptTemplatesRequest struct {
	Page     int     `json:"page"`
	PageSize int     `json:"pageSize"`
	Name     string  `json:"name,omitempty"`
	IsPublic *int    `json:"isPublic,omitempty"` // 0=私有 / 1=公共
	Status   *int    `json:"status,omitempty"`   // 0=禁用 / 1=可用
	IDs      []int64 `json:"ids,omitempty"`
}

// ListScriptTemplates 调用 §7.5.2 分页查询脚本模板（返回首层 data 数组）。
func (c *Client) ListScriptTemplates(ctx context.Context, req ListScriptTemplatesRequest) ([]ScriptTemplate, error) {
	const path = "/open/api/autoScript/api/automation/templates/page"
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 100
	}
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var page struct {
		Data []ScriptTemplate `json:"data"`
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, err
	}
	return page.Data, nil
}

// ToggleScriptTemplates 批量启停脚本模板（§7.5.1 batch-toggle）。status：1=可用 / 0=禁用。
func (c *Client) ToggleScriptTemplates(ctx context.Context, ids []int64, status int) error {
	const path = "/open/api/autoScript/api/automation/templates/batch-toggle"
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, map[string]any{"ids": ids, "status": status})
	return err
}

// DeleteScriptTemplates 批量删除脚本模板（§7.5.1 batch-delete）。
func (c *Client) DeleteScriptTemplates(ctx context.Context, ids []int64) error {
	const path = "/open/api/autoScript/api/automation/templates/batch-delete"
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, map[string]any{"ids": ids})
	return err
}

// ===== §7.3 创建周期计划 / §7.4 计划启停删 =====

// CreateScriptPlanRequest 是 §7.3 scriptPlan/create 的请求体。
type CreateScriptPlanRequest struct {
	ScriptID           int64    `json:"scriptId"`
	PlanName           string   `json:"planName"`
	ScriptParams       string   `json:"scriptParams,omitempty"`
	ExecutionFrequency string   `json:"executionFrequency"`      // INTERVAL / DAILY
	IntervalValue      int      `json:"intervalValue,omitempty"` // INTERVAL：间隔分钟
	ExecutionTime      string   `json:"executionTime,omitempty"` // DAILY：HH:mm:ss
	StartTime          string   `json:"startTime,omitempty"`
	EndTime            string   `json:"endTime,omitempty"`
	Remark             string   `json:"remark,omitempty"`
	CpIDList           []string `json:"cpIdList"` // 推荐：cpId 字符串列表
}

// ScriptPlanCreated 是 §7.3 返回。
type ScriptPlanCreated struct {
	ID         int64  `json:"id"`
	PlanUID    string `json:"planUid"`
	ScriptID   int64  `json:"scriptId"`
	PlanName   string `json:"planName"`
	PlanStatus string `json:"planStatus"` // 初始 NOT_STARTED
}

// CreateScriptPlan 调用 §7.3 创建周期计划。
func (c *Client) CreateScriptPlan(ctx context.Context, req CreateScriptPlanRequest) (*ScriptPlanCreated, error) {
	const path = "/open/api/autoScript/scriptPlan/create"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return &ScriptPlanCreated{}, nil
	}
	var out ScriptPlanCreated
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// planAction 封装 §7.4 的特殊风格：POST + query ?id= + 无 body。
func (c *Client) planAction(ctx context.Context, action string, id int64) error {
	path := "/open/api/autoScript/scriptPlan/" + action
	q := neturl.Values{}
	q.Set("id", strconv.FormatInt(id, 10))
	_, err := c.doJSON(ctx, http.MethodPost, path, q, nil)
	return err
}

// StartScriptPlan 启动计划（§7.4，NOT_STARTED/PAUSED → ENABLING）。
func (c *Client) StartScriptPlan(ctx context.Context, id int64) error {
	return c.planAction(ctx, "start", id)
}

// PauseScriptPlan 暂停计划（§7.4，ENABLING → PAUSED）。
func (c *Client) PauseScriptPlan(ctx context.Context, id int64) error {
	return c.planAction(ctx, "pause", id)
}

// DeleteScriptPlan 删除计划（§7.4）。
func (c *Client) DeleteScriptPlan(ctx context.Context, id int64) error {
	return c.planAction(ctx, "delete", id)
}

// ===== §7.6.1 任务分页（供 worker 按 planUid 发现派生任务）=====

// TaskPageRequest 是 §7.6.1 task/page 入参（只放闭环所需过滤）。
type TaskPageRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	PlanUID  string `json:"planUid,omitempty"`
	CpID     string `json:"cpId,omitempty"`
	ScriptID int64  `json:"scriptId,omitempty"`
}

// TaskPage 调用 §7.6.1 任务分页，返回首层 data 数组（复用 ScriptTaskVO）。
func (c *Client) TaskPage(ctx context.Context, req TaskPageRequest) ([]ScriptTaskVO, error) {
	const path = "/open/api/autoScript/autoscript/task/page"
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 100
	}
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var page struct {
		Data []ScriptTaskVO `json:"data"`
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, err
	}
	return page.Data, nil
}
