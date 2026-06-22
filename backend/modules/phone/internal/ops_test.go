package phone

import (
	"context"
	"errors"
	"testing"
	"time"

	"manager-backend/framework"
	"manager-backend/framework/midplat"
	"manager-backend/modules/billing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePort 是 midplatPort 的假实现：记录最近一次调用，便于断言「透传了正确的 cpId」。
type fakePort struct {
	lastCpID   string
	lastOp     string
	calls      int
	err        error
	statuses   map[string]string // 供 Statuses 返回的 cpId→status 桩数据
	createCpID string            // Create 返回的 cpId（空则用 "cp-new"）

	adbToken *midplat.ADBTokenContainer // 非空则覆盖 AdbEnableToken 返回
	adbInfo  *midplat.CloudPhoneAdbInfo // 非空则覆盖 AdbInfo 返回（测过期补全 / DATA_NOT_EXIST 降级）
	rooted   map[string]bool            // 供 RootEnabledMap 返回的 cpId→isRooted 桩数据

	scriptID       int64                 // ScriptTemplateID 返回（0=未上传，触发 upload）
	scriptUploaded bool                  // 标记 UploadScriptTemplate 是否被调用
	scriptTask     *midplat.ScriptTaskVO // 非空则覆盖 ScriptTaskStatus 返回

	runLogPage *midplat.RunLogPage // 非空则覆盖 RunLogs 返回（远控真实开机时长用）
}

func (f *fakePort) Create(_ context.Context, args CreateArgs) (*CreateResult, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	cp := f.createCpID
	if cp == "" {
		cp = "cp-new"
	}
	return &CreateResult{CpID: cp, VmID: "vm-1", ImageID: "img-1"}, nil
}

func (f *fakePort) note(cpID, op string) error {
	f.calls++
	f.lastCpID = cpID
	f.lastOp = op
	return f.err
}

func (f *fakePort) StartOrShutdown(_ context.Context, cpID, operation string) error {
	return f.note(cpID, "power:"+operation)
}
func (f *fakePort) Restart(_ context.Context, cpID string) error { return f.note(cpID, "restart") }
func (f *fakePort) Reset(_ context.Context, cpID, imageID string) error {
	return f.note(cpID, "reset:"+imageID)
}
func (f *fakePort) RefreshPhone(_ context.Context, cpID string) error {
	return f.note(cpID, "refresh")
}
func (f *fakePort) Destroy(_ context.Context, cpID string) error { return f.note(cpID, "destroy") }
func (f *fakePort) WebRTCAuth(_ context.Context, cpID string) (*midplat.WebRTCAuthInfo, error) {
	if err := f.note(cpID, "webrtc-auth"); err != nil {
		return nil, err
	}
	return &midplat.WebRTCAuthInfo{CpID: cpID, SignalURL: "wss://sig", PushStreamURL: "rtmp://push", AuthToken: "tok"}, nil
}
func (f *fakePort) WebRTCState(_ context.Context, cpID string) (bool, error) {
	return true, f.note(cpID, "webrtc-state")
}
func (f *fakePort) Screenshot(_ context.Context, cpID, format string) error {
	return f.note(cpID, "screenshot:"+format)
}
func (f *fakePort) SetVolume(_ context.Context, cpID string, v int) error {
	return f.note(cpID, "volume")
}
func (f *fakePort) Rotate(_ context.Context, cpID, o string) error { return f.note(cpID, "rotate:"+o) }
func (f *fakePort) Shake(_ context.Context, cpID string) error     { return f.note(cpID, "shake") }
func (f *fakePort) InstalledApps(_ context.Context, cpID string) ([]midplat.InstalledApp, error) {
	if err := f.note(cpID, "apps"); err != nil {
		return nil, err
	}
	return []midplat.InstalledApp{{PackageName: "com.demo", AppName: "Demo"}}, nil
}
func (f *fakePort) InstallApp(_ context.Context, cpID string, _ []int64) error {
	return f.note(cpID, "install")
}
func (f *fakePort) UninstallApp(_ context.Context, cpID string, _ []int64, _ []string) error {
	return f.note(cpID, "uninstall")
}
func (f *fakePort) StartApp(_ context.Context, cpID string, _ []int64, _ []string) error {
	return f.note(cpID, "startApp")
}
func (f *fakePort) StopApp(_ context.Context, cpID string, _ []int64, _ []string) error {
	return f.note(cpID, "stopApp")
}
func (f *fakePort) KillAllApps(_ context.Context, cpID string) error { return f.note(cpID, "killall") }
func (f *fakePort) AdbEnableToken(_ context.Context, cpID string) (*midplat.ADBTokenContainer, error) {
	if err := f.note(cpID, "adbEnableToken"); err != nil {
		return nil, err
	}
	if f.adbToken != nil {
		return f.adbToken, nil
	}
	return &midplat.ADBTokenContainer{ContainerID: cpID, LoginCode: "tk_demo", AdbAddress: cpID + ".test-adb.cphone.cn:30002"}, nil
}
func (f *fakePort) AdbDisableToken(_ context.Context, cpID string) error {
	return f.note(cpID, "adbDisableToken")
}
func (f *fakePort) AdbEnabledMap(_ context.Context, _ []string) (map[string]bool, error) {
	return nil, nil
}
func (f *fakePort) Root(_ context.Context, _, cpID string, enable bool) error {
	op := "rootEnable"
	if !enable {
		op = "rootDisable"
	}
	return f.note(cpID, op)
}
func (f *fakePort) RootEnabledMap(_ context.Context, _ []string) (map[string]bool, error) {
	return f.rooted, nil
}
func (f *fakePort) ScriptTemplateID(_ context.Context, _ string) (int64, error) {
	if f.scriptID == 0 && f.scriptUploaded {
		return 42, nil // 上传后模板已存在
	}
	return f.scriptID, nil
}
func (f *fakePort) UploadScriptTemplate(_ context.Context, _, _, _, _ string, _ []byte) error {
	f.scriptUploaded = true
	return f.note("", "uploadScriptTemplate")
}
func (f *fakePort) CreateScriptTask(_ context.Context, _ int64, _, cpID, _ string) (int64, string, error) {
	if err := f.note(cpID, "createScriptTask"); err != nil {
		return 0, "", err
	}
	return 9001, "T-9001", nil
}
func (f *fakePort) ScriptTaskStatus(_ context.Context, _ int64) (*midplat.ScriptTaskVO, error) {
	if f.scriptTask != nil {
		return f.scriptTask, nil
	}
	return &midplat.ScriptTaskVO{ID: 9001, TaskID: "T-9001", TaskStatus: "COMPLETED"}, nil
}
func (f *fakePort) ScriptTaskReport(_ context.Context, _ int64) (*midplat.ScriptTaskReport, error) {
	return &midplat.ScriptTaskReport{TaskID: "T-9001", RunLog: "log\n#RESULT#{\"ok\":true}#RESULT#"}, nil
}
func (f *fakePort) RunLogs(_ context.Context, cpID string, _, _ int) (*midplat.RunLogPage, error) {
	if err := f.note(cpID, "runLogs"); err != nil {
		return nil, err
	}
	if f.runLogPage != nil {
		return f.runLogPage, nil
	}
	return &midplat.RunLogPage{PageNum: 1, PageSize: 20, TotalSize: 1, Data: []midplat.RunLogEntry{{LogNo: "LOG1", CpID: cpID, SessionStatus: "已关机"}}}, nil
}

// 远控真实开机时长：运行中日志(开机在过去、未关机) → running=true 且 uptime>0；已关机/无日志 → running=false。
func TestRuntimeRealUptime(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	on := time.Now().Add(-90 * time.Minute).Format("2006-01-02 15:04:05")
	f := &fakePort{
		statuses:   map[string]string{"cp-rt": "NORMAL"},
		runLogPage: &midplat.RunLogPage{PageNum: 1, PageSize: 1, TotalSize: 1, Data: []midplat.RunLogEntry{{LogNo: "L1", CpID: "cp-rt", PowerOnTime: on, PowerOffTime: "运行中", SessionStatus: "运行中"}}},
	}
	withFakeOps(t, f)
	p := insertPhone(t, userA, StatusRunning, "cp-rt")

	info, err := PhoneService.Runtime(userA, int(p.ID))
	require.NoError(t, err)
	assert.True(t, info.Running)
	assert.Greater(t, info.UptimeSeconds, int64(5300)) // ~90min=5400s
	assert.NotEmpty(t, info.PowerOnAt)

	// 最新日志已关机 → 不运行
	f.runLogPage = &midplat.RunLogPage{TotalSize: 1, Data: []midplat.RunLogEntry{{LogNo: "L2", CpID: "cp-rt", PowerOnTime: on, PowerOffTime: "2026-06-17 10:00:00", SessionStatus: "已关机"}}}
	info, err = PhoneService.Runtime(userA, int(p.ID))
	require.NoError(t, err)
	assert.False(t, info.Running)
	assert.Equal(t, int64(0), info.UptimeSeconds)

	// 无日志 → 不运行
	f.runLogPage = &midplat.RunLogPage{TotalSize: 0}
	info, err = PhoneService.Runtime(userA, int(p.ID))
	require.NoError(t, err)
	assert.False(t, info.Running)
}
func (f *fakePort) AdbInfo(_ context.Context, cpID string) (*midplat.CloudPhoneAdbInfo, error) {
	if err := f.note(cpID, "adbInfo"); err != nil {
		return nil, err
	}
	if f.adbInfo != nil {
		return f.adbInfo, nil
	}
	return &midplat.CloudPhoneAdbInfo{CpID: cpID, AdbAddress: "10.0.0.1:5555", AdbToken: "tok", Status: "NORMAL"}, nil
}
func (f *fakePort) FileList(_ context.Context, _, cpID, path string) ([]midplat.PhoneFile, error) {
	if err := f.note(cpID, "fileList:"+path); err != nil {
		return nil, err
	}
	return nil, nil
}

func (f *fakePort) FileDownload(_ context.Context, _, cpID, path string) ([]byte, error) {
	if err := f.note(cpID, "fileDownload:"+path); err != nil {
		return nil, err
	}
	return []byte("data"), nil
}
func (f *fakePort) FileUpload(_ context.Context, _, cpID, folderPath string, _ []midplat.UploadFile) error {
	return f.note(cpID, "fileUpload:"+folderPath)
}
func (f *fakePort) FileDelete(_ context.Context, _, cpID, path string) error {
	return f.note(cpID, "fileDelete:"+path)
}
func (f *fakePort) Statuses(_ context.Context, _ []string) (map[string]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.statuses, nil
}

// withFakeOps 临时把 PhoneService.ops 换成假实现，测试结束自动还原。
func withFakeOps(t *testing.T, f midplatPort) {
	t.Helper()
	prev := PhoneService.ops
	PhoneService.ops = f
	t.Cleanup(func() { PhoneService.ops = prev })
}

// provisionedPhone 造一台「本人拥有且已开通（cpId 非空）」的云手机，状态置 STOPPED（可开机）。
// 默认带 ProxyID=1（已绑代理），满足开机门禁「必须绑定代理」；直接落库，绕过 service.Create
// 的中台创建流程，避免污染 fakePort 的调用计数。
func provisionedPhone(t *testing.T, userID int, cpID string) int {
	t.Helper()
	p := CloudPhone{UserID: uint(userID), Name: "op机", Status: StatusStopped, CpID: cpID, ProxyID: 1}
	require.NoError(t, framework.DB.Create(&p).Error)
	return int(p.ID)
}

func TestOpPassesCpIDAndChecksOwnership(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("cloud_phones", "cp_tasks", "billing_runtime_minute_wallets", "billing_ledger_entries")
	})
	require.NoError(t, billing.GrantRuntimeMinutesWalletForTest(userA, 1000)) // 开机前置校验放行
	f := &fakePort{statuses: map[string]string{"cp-aaa": "STOPPED"}}          // 实时态：可开机
	withFakeOps(t, f)

	id := provisionedPhone(t, userA, "cp-aaa")

	// 开机：透传正确 cpId + 操作
	require.NoError(t, PhoneService.Power(userA, id, "开机"))
	assert.Equal(t, "cp-aaa", f.lastCpID)
	assert.Equal(t, "power:开机", f.lastOp)

	// 各操作都把同一台的 cpId 透传过去
	require.NoError(t, PhoneService.Restart(userA, id))
	assert.Equal(t, "restart", f.lastOp)
	require.NoError(t, PhoneService.Screenshot(userA, id, "png"))
	assert.Equal(t, "screenshot:png", f.lastOp)
	require.NoError(t, PhoneService.Rotate(userA, id, "landscape"))
	assert.Equal(t, "rotate:landscape", f.lastOp)

	info, err := PhoneService.WebRTCAuth(userA, id)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "cp-aaa", info.CpID)
	assert.NotEmpty(t, info.SignalURL)
}

// 自动化脚本最小闭环：模板缺失→自助上传→建任务，且仅运行中可发；终态合并报告抽取结果。
func TestRunHelloScriptUploadsAndCreates(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	f := &fakePort{statuses: map[string]string{"cp-s": MidplatReady}, scriptID: 0} // 模板不存在
	withFakeOps(t, f)
	id := provisionedPhone(t, userA, "cp-s")

	res, err := PhoneService.RunHelloScript(userA, id)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, f.scriptUploaded, "模板缺失时应自助上传")
	assert.Equal(t, int64(9001), res.TaskID)
	assert.Equal(t, "createScriptTask", f.lastOp)
	assert.Equal(t, "cp-s", f.lastCpID)

	// 查状态：终态合并报告，抽出 report_result 内容。
	detail, err := PhoneService.ScriptTaskStatus(userA, id, res.TaskID)
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", detail.Status)
	assert.True(t, detail.Terminal)
	assert.Equal(t, `{"ok":true}`, detail.Result)
}

// 非运行态不能发脚本。
func TestRunHelloScriptRejectsWhenNotRunning(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	withFakeOps(t, &fakePort{statuses: map[string]string{"cp-off": MidplatStopped}})
	id := provisionedPhone(t, userA, "cp-off")
	_, err := PhoneService.RunHelloScript(userA, id)
	require.Error(t, err)
}

// 列表用中台实时状态覆盖展示态（状态始终和中台一致）：中台 NORMAL → 业务 RUNNING。
func TestListEnrichesFromMidplat(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	withFakeOps(t, &fakePort{statuses: map[string]string{"cp-x": MidplatReady}})

	idProv := provisionedPhone(t, userA, "cp-x") // 本地 STOPPED，但中台 NORMAL

	list, _, err := PhoneService.GetList(userA, 1, 10, "", "", "", "", "")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, int(list[0].ID), idProv)
	assert.Equal(t, StatusRunning, list[0].Status, "应被中台实时状态(NORMAL→RUNNING)覆盖")
}

func TestOpRejectsOtherUsersPhone(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	f := &fakePort{}
	withFakeOps(t, f)

	id := provisionedPhone(t, userA, "cp-aaa")

	// B 操作 A 的云手机 → NotFound，且绝不触达中台
	assert.Error(t, PhoneService.Power(userB, id, "开机"))
	assert.Error(t, PhoneService.Restart(userB, id))
	_, err := PhoneService.WebRTCAuth(userB, id)
	assert.Error(t, err)
	assert.Equal(t, 0, f.calls, "越权操作不应调用中台")
}

func TestOpRejectsUnprovisioned(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	f := &fakePort{}
	withFakeOps(t, f)

	// 未开通（cpId 为空）的云手机不能执行操作
	p := CloudPhone{UserID: userA, Name: "未开通", Status: StatusCreated}
	require.NoError(t, framework.DB.Create(&p).Error)
	assert.Error(t, PhoneService.Power(userA, int(p.ID), "开机"))
	assert.Equal(t, 0, f.calls, "未开通不应调用中台")
}

func TestOpInvalidArgs(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	f := &fakePort{}
	withFakeOps(t, f)
	id := provisionedPhone(t, userA, "cp-aaa")

	assert.Error(t, PhoneService.Power(userA, id, "xx"))            // 非法 operation
	assert.Error(t, PhoneService.Rotate(userA, id, "diagonal"))     // 非法 orientation
	assert.Error(t, PhoneService.InstallApp(userA, id, nil))        // 空 appIds
	assert.Error(t, PhoneService.UninstallApp(userA, id, nil, nil)) // 都为空
}

func TestOpUnconfiguredMidplat(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	withFakeOps(t, nil) // 模拟中台未配置
	id := provisionedPhone(t, userA, "cp-aaa")
	assert.Error(t, PhoneService.Power(userA, id, "开机"))
}

func TestOpErrorPropagates(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	f := &fakePort{err: errors.New("中台炸了")}
	withFakeOps(t, f)
	id := provisionedPhone(t, userA, "cp-aaa")
	assert.Error(t, PhoneService.Restart(userA, id))
}

func TestOpDestroyRemovesLocalRecord(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	f := &fakePort{}
	withFakeOps(t, f)
	id := provisionedPhone(t, userA, "cp-aaa")

	require.NoError(t, PhoneService.Destroy(userA, id))
	assert.Equal(t, "destroy", f.lastOp)
	// 本地档案应已删除
	_, err := PhoneService.GetByID(userA, id)
	assert.Error(t, err)
}

// TestAdbEnableReturnsTokenAndAddress 校验 enable 走 token 接口，
// 以 enable 响应的 login_code / adb_address 为准，并 best-effort 从 §2.6 补过期时间。
func TestAdbEnableReturnsTokenAndAddress(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	f := &fakePort{
		adbToken: &midplat.ADBTokenContainer{ContainerID: "cp-aaa", LoginCode: "tk_zjdov8ohaevt", AdbAddress: "cp-aaa.test-adb.cphone.cn:30002"},
		adbInfo:  &midplat.CloudPhoneAdbInfo{CpID: "cp-aaa", AdbToken: "tk_zjdov8ohaevt", AdbTokenExpiredAt: "2026-08-10T15:46:16", Status: "NORMAL"},
	}
	withFakeOps(t, f)
	id := provisionedPhone(t, userA, "cp-aaa")

	info, err := PhoneService.AdbEnable(userA, id)
	require.NoError(t, err)
	assert.True(t, info.Enabled)
	assert.Equal(t, "tk_zjdov8ohaevt", info.AdbToken)
	assert.Equal(t, "cp-aaa.test-adb.cphone.cn:30002", info.AdbAddress)
	assert.Equal(t, "2026-08-10T15:46:16", info.AdbTokenExpiredAt)
}

// TestAdbEnableMissingLoginCodeErrors 校验中台没返回登录码时视为失败。
func TestAdbEnableMissingLoginCodeErrors(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	f := &fakePort{adbToken: &midplat.ADBTokenContainer{ContainerID: "cp-aaa"}} // 无 login_code
	withFakeOps(t, f)
	id := provisionedPhone(t, userA, "cp-aaa")

	_, err := PhoneService.AdbEnable(userA, id)
	require.Error(t, err)
}

// TestAdbDisableCallsTokenDisable 校验关闭走 token disable。
func TestAdbDisableCallsTokenDisable(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	f := &fakePort{}
	withFakeOps(t, f)
	id := provisionedPhone(t, userA, "cp-aaa")

	require.NoError(t, PhoneService.AdbDisable(userA, id))
	assert.Equal(t, "adbDisableToken", f.lastOp)
}

// TestAdbInfoDataNotExistDegradesToDisabled 覆盖 P1-A10：底层对不存在 cpId 降级返回空信息，
// service 应呈现为「未开启」而非报错。
func TestAdbInfoDataNotExistDegradesToDisabled(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	f := &fakePort{adbInfo: &midplat.CloudPhoneAdbInfo{CpID: "cp-aaa"}} // 无 token = 未开启
	withFakeOps(t, f)
	id := provisionedPhone(t, userA, "cp-aaa")

	info, err := PhoneService.AdbInfo(userA, id)
	require.NoError(t, err)
	assert.False(t, info.Enabled)
	assert.Empty(t, info.AdbToken)
}
