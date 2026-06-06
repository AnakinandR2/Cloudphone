package phone

import (
	"context"
	"errors"
	"testing"

	"manager-backend/framework"
	"manager-backend/framework/midplat"

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
	return &CreateResult{CpID: cp, VmID: "vm-1", ImageID: "img-1", Region: args.Region}, nil
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
func (f *fakePort) AdbOperate(_ context.Context, cpID, operation string, _ []string, _ int) (*midplat.AdbOperateResult, error) {
	if err := f.note(cpID, "adbOperate:"+operation); err != nil {
		return nil, err
	}
	return &midplat.AdbOperateResult{AllSuccess: true, SuccessContainers: []string{cpID}}, nil
}
func (f *fakePort) AdbWhitelist(_ context.Context, cpID string) ([]midplat.AdbWhitelistEntry, error) {
	if err := f.note(cpID, "adbWhitelist"); err != nil {
		return nil, err
	}
	return nil, nil
}
func (f *fakePort) AdbInfo(_ context.Context, cpID string) (*midplat.CloudPhoneAdbInfo, error) {
	if err := f.note(cpID, "adbInfo"); err != nil {
		return nil, err
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
// 直接落库，绕过 service.Create 的中台创建流程，避免污染 fakePort 的调用计数。
func provisionedPhone(t *testing.T, userID int, cpID string) int {
	t.Helper()
	p := CloudPhone{UserID: uint(userID), Name: "op机", Status: StatusStopped, CpID: cpID}
	require.NoError(t, framework.DB.Create(&p).Error)
	return int(p.ID)
}

func TestOpPassesCpIDAndChecksOwnership(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones", "cp_tasks") })
	f := &fakePort{}
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
