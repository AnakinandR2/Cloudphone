package automation

import (
	"context"
	"fmt"
	"time"

	"manager-backend/framework"
	"manager-backend/framework/midplat"
)

// midplatPort 是 automation 对中台自动化能力（手册 §7）的出站端口。
// 抽接口便于单测注入假实现，无需真打中台。返回值直接用 SDK 类型，避免重复 DTO。
type midplatPort interface {
	// 脚本模板（§7.5）。midName 为我们生成的唯一模板名，便于上传后可靠取回 scriptId。
	UploadTemplate(ctx context.Context, midName, version, desc string, lua []byte) error
	TemplateScriptID(ctx context.Context, midName string) (int64, error)
	ToggleTemplate(ctx context.Context, scriptID int64, enabled bool) error
	DeleteTemplate(ctx context.Context, scriptID int64) error

	// 任务（§7.2 创建 / §7.6 查询报告）。scriptParamsByCp：cpId → 该台 scriptParams JSON（无则空串）。
	CreateTasks(ctx context.Context, scriptID int64, taskName, publishTime string, cpIDs []string, scriptParamsByCp map[string]string) ([]midplat.ScriptTaskCreated, error)
	TaskStatuses(ctx context.Context, ids []int64) ([]midplat.ScriptTaskVO, error)
	TaskReport(ctx context.Context, id int64) (*midplat.ScriptTaskReport, error)
	TasksByPlan(ctx context.Context, planUID string) ([]midplat.ScriptTaskVO, error)

	// 周期计划（§7.3 创建 / §7.4 启停删）。
	CreatePlan(ctx context.Context, req midplat.CreateScriptPlanRequest) (*midplat.ScriptPlanCreated, error)
	StartPlan(ctx context.Context, planID int64) error
	PausePlan(ctx context.Context, planID int64) error
	DeletePlan(ctx context.Context, planID int64) error
}

type sdkAdapter struct{ c *midplat.Client }

func (a *sdkAdapter) UploadTemplate(ctx context.Context, midName, version, desc string, lua []byte) error {
	return a.c.UploadLuaTemplate(ctx, midName+".lua", midName, version, desc, lua)
}

func (a *sdkAdapter) TemplateScriptID(ctx context.Context, midName string) (int64, error) {
	list, err := a.c.ListScriptTemplatesByName(ctx, midName)
	if err != nil {
		return 0, err
	}
	for _, t := range list {
		if t.Name == midName {
			return t.ID, nil
		}
	}
	return 0, nil
}

func (a *sdkAdapter) ToggleTemplate(ctx context.Context, scriptID int64, enabled bool) error {
	status := 0
	if enabled {
		status = 1
	}
	return a.c.ToggleScriptTemplates(ctx, []int64{scriptID}, status)
}

func (a *sdkAdapter) DeleteTemplate(ctx context.Context, scriptID int64) error {
	return a.c.DeleteScriptTemplates(ctx, []int64{scriptID})
}

func (a *sdkAdapter) CreateTasks(ctx context.Context, scriptID int64, taskName, publishTime string, cpIDs []string, scriptParamsByCp map[string]string) ([]midplat.ScriptTaskCreated, error) {
	items := make([]midplat.CreateScriptTaskItem, 0, len(cpIDs))
	for _, cp := range cpIDs {
		items = append(items, midplat.CreateScriptTaskItem{CpID: cp, PublishTime: publishTime, ScriptParams: scriptParamsByCp[cp]})
	}
	return a.c.CreateScriptTasks(ctx, midplat.CreateScriptTaskRequest{
		ScriptID: scriptID, TaskName: taskName, TaskList: items,
	})
}

func (a *sdkAdapter) TaskStatuses(ctx context.Context, ids []int64) ([]midplat.ScriptTaskVO, error) {
	return a.c.QueryScriptTasksByIDs(ctx, ids)
}

func (a *sdkAdapter) TaskReport(ctx context.Context, id int64) (*midplat.ScriptTaskReport, error) {
	return a.c.GetScriptTaskReport(ctx, id)
}

func (a *sdkAdapter) TasksByPlan(ctx context.Context, planUID string) ([]midplat.ScriptTaskVO, error) {
	return a.c.TaskPage(ctx, midplat.TaskPageRequest{PlanUID: planUID, PageSize: 200})
}

func (a *sdkAdapter) CreatePlan(ctx context.Context, req midplat.CreateScriptPlanRequest) (*midplat.ScriptPlanCreated, error) {
	return a.c.CreateScriptPlan(ctx, req)
}

func (a *sdkAdapter) StartPlan(ctx context.Context, planID int64) error {
	return a.c.StartScriptPlan(ctx, planID)
}
func (a *sdkAdapter) PausePlan(ctx context.Context, planID int64) error {
	return a.c.PauseScriptPlan(ctx, planID)
}
func (a *sdkAdapter) DeletePlan(ctx context.Context, planID int64) error {
	return a.c.DeleteScriptPlan(ctx, planID)
}

// newMidplatPort 从 framework.AppConfig 构造真实适配器；中台未配置时返回 nil。
func newMidplatPort() midplatPort {
	cfg := framework.AppConfig
	if cfg == nil || cfg.MidplatBaseURL == "" || cfg.MidplatAccessKey == "" || cfg.MidplatSecretKey == "" {
		return nil
	}
	c, err := midplat.New(midplat.Config{
		BaseURL:      cfg.MidplatBaseURL,
		AccessKey:    cfg.MidplatAccessKey,
		SecretKey:    cfg.MidplatSecretKey,
		TenantUID:    cfg.MidplatTenantUID,
		OperatorName: cfg.MidplatOperatorName,
		HTTPTimeout:  60 * time.Second,
	})
	if err != nil {
		return nil
	}
	return &sdkAdapter{c: c}
}

// opCtx 给每次中台操作独立超时。
func opCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// beijingNow 返回中台无时区接口用的「北京时间当前」字符串（UTC+8）。
func beijingNow() string {
	return time.Now().UTC().Add(8 * time.Hour).Format("2006-01-02T15:04:05")
}

// uniqueMidName 为一条脚本生成中台唯一模板名（含本地 id + 纳秒，编辑重传也唯一）。
func uniqueMidName(localID uint) string {
	return fmt.Sprintf("glory-%d-%d", localID, time.Now().UnixNano())
}
