package automation

import (
	"context"
	"os"
	"testing"

	"manager-backend/framework"
	"manager-backend/framework/midplat"

	// 触发 phone 模块注册，使 phone 门面 OwnedCpIDs 可用。
	_ "manager-backend/modules/phone"
)

func TestMain(m *testing.M) {
	tdb, _ := framework.SetupTestDB(m)
	// 建所有已注册模块的表（含 automation 三表 + cloud_phones）。
	if err := framework.RunSetup(framework.DB); err != nil {
		panic(err)
	}
	// 装配所有已注册模块（phone 门面依赖 phoneModule.Init 注入 PhoneService）。
	for _, mod := range framework.GlobalModule.GetAll() {
		if err := mod.Init(framework.DB); err != nil {
			panic(err)
		}
	}
	code := m.Run()
	tdb.Teardown()
	os.Exit(code)
}

// withFakeOps 临时把 Service.ops 换成假实现，结束自动还原并清理本测试造的数据
// （多测试共用一个 sqlite 库，需各自清理避免 mid_task_id 唯一键冲突）。
func withFakeOps(t *testing.T, f midplatPort) {
	t.Helper()
	prev := Service.ops
	Service.ops = f
	t.Cleanup(func() {
		Service.ops = prev
		for _, tbl := range []string{"automation_tasks", "automation_plans", "automation_scripts", "cloud_phones"} {
			framework.DB.Exec("DELETE FROM " + tbl)
		}
	})
}

// seedPhone 在 cloud_phones 表造一台「本人拥有且已开通」的机器，供归属校验通过。
func seedPhone(t *testing.T, userID int, cpID string) {
	t.Helper()
	err := framework.DB.Exec(
		"INSERT INTO cloud_phones (user_id, cp_id, name, status, proxy_id) VALUES (?,?,?,?,0)",
		userID, cpID, "m", "RUNNING",
	).Error
	if err != nil {
		t.Fatalf("seed phone: %v", err)
	}
}

// fakeOps 是 midplatPort 的假实现：记录调用，便于断言。
type fakeOps struct {
	uploaded    bool
	scriptID    int64 // TemplateScriptID 返回（0 → 上传后返回 100）
	deletedTpl  []int64
	lastTaskCps []string
	lastPlan    midplat.CreateScriptPlanRequest
	planActions []string
}

func (f *fakeOps) UploadTemplate(_ context.Context, _, _, _ string, _ []byte) error {
	f.uploaded = true
	return nil
}
func (f *fakeOps) TemplateScriptID(_ context.Context, _ string) (int64, error) {
	if f.scriptID == 0 && f.uploaded {
		return 100, nil
	}
	return f.scriptID, nil
}
func (f *fakeOps) ToggleTemplate(_ context.Context, _ int64, _ bool) error { return nil }
func (f *fakeOps) DeleteTemplate(_ context.Context, id int64) error {
	f.deletedTpl = append(f.deletedTpl, id)
	return nil
}
func (f *fakeOps) CreateTasks(_ context.Context, _ int64, taskName, _ string, cpIDs []string) ([]midplat.ScriptTaskCreated, error) {
	f.lastTaskCps = cpIDs
	out := make([]midplat.ScriptTaskCreated, 0, len(cpIDs))
	for i, cp := range cpIDs {
		out = append(out, midplat.ScriptTaskCreated{ID: int64(9000 + i), TaskID: "T-" + cp, CpID: cp})
	}
	return out, nil
}
func (f *fakeOps) TaskStatuses(_ context.Context, ids []int64) ([]midplat.ScriptTaskVO, error) {
	out := make([]midplat.ScriptTaskVO, 0, len(ids))
	for _, id := range ids {
		out = append(out, midplat.ScriptTaskVO{ID: id, TaskStatus: "COMPLETED", TaskStatusDesc: "任务完成"})
	}
	return out, nil
}
func (f *fakeOps) TaskReport(_ context.Context, _ int64) (*midplat.ScriptTaskReport, error) {
	return &midplat.ScriptTaskReport{
		ScreenshotURL: []string{"https://oss/x.png"},
		RunLog:        "log\n#RESULT#{\"ok\":true}#RESULT#",
		RunDurationMs: 1234,
	}, nil
}
func (f *fakeOps) TasksByPlan(_ context.Context, _ string) ([]midplat.ScriptTaskVO, error) {
	return nil, nil
}
func (f *fakeOps) CreatePlan(_ context.Context, req midplat.CreateScriptPlanRequest) (*midplat.ScriptPlanCreated, error) {
	f.lastPlan = req
	return &midplat.ScriptPlanCreated{ID: 555, PlanUID: "plan-uid-x", PlanStatus: "NOT_STARTED"}, nil
}
func (f *fakeOps) StartPlan(_ context.Context, _ int64) error {
	f.planActions = append(f.planActions, "start")
	return nil
}
func (f *fakeOps) PausePlan(_ context.Context, _ int64) error {
	f.planActions = append(f.planActions, "pause")
	return nil
}
func (f *fakeOps) DeletePlan(_ context.Context, _ int64) error {
	f.planActions = append(f.planActions, "delete")
	return nil
}
