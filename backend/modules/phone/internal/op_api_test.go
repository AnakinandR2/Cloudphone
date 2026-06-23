package phone

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"manager-backend/framework"
	"manager-backend/modules/billing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// opCleanup 清理操作类 handler 测试涉及的表。
func opCleanup(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("cloud_phones", "cp_tasks", "run_sessions",
			"billing_license_units", "billing_ledger_entries",
			"billing_runtime_minute_wallets")
	})
}

// 大多数操作 handler 走 resolveCp（拥有 + 已开通 cpId）→ 透传 fakePort，期望 200。
// 这里一次性铺一台运行中的 provisioned 实例（多数操作不限状态），断言各 handler 成功。
func TestOpHandlersHappyPath(t *testing.T) {
	opCleanup(t)
	f := &fakePort{
		rooted:   map[string]bool{"cp-op": true},
		statuses: map[string]string{"cp-op": "NORMAL"},
	}
	withFakeOps(t, f)

	p := CloudPhone{UserID: userA, Name: "op机", Status: StatusRunning, CpID: "cp-op", ProxyID: 1}
	require.NoError(t, framework.DB.Create(&p).Error)
	idStr := strconv.Itoa(int(p.ID))

	run := func(h gin.HandlerFunc, method string, body any, extraParams ...string) *httptest.ResponseRecorder {
		params := append([]string{"id", idStr}, extraParams...)
		c, w := ctxFor(t, method, userA, body, params...)
		h(c)
		return w
	}

	assert.Equal(t, http.StatusOK, run(RestartCloudPhone, http.MethodPost, nil).Code)
	assert.Equal(t, http.StatusOK, run(ResetCloudPhone, http.MethodPost, map[string]any{"imageId": "img-1"}).Code)
	assert.Equal(t, http.StatusOK, run(NewDeviceCloudPhone, http.MethodPost, nil).Code)
	assert.Equal(t, http.StatusOK, run(WebRTCAuthCloudPhone, http.MethodPost, nil).Code)
	assert.Equal(t, http.StatusOK, run(WebRTCStateCloudPhone, http.MethodGet, nil).Code)
	assert.Equal(t, http.StatusOK, run(ScreenshotCloudPhone, http.MethodPost, map[string]any{"format": "png"}).Code)
	assert.Equal(t, http.StatusOK, run(VolumeCloudPhone, http.MethodPost, map[string]any{"volume": 50}).Code)
	assert.Equal(t, http.StatusOK, run(RotateCloudPhone, http.MethodPost, map[string]any{"orientation": "landscape"}).Code)
	assert.Equal(t, http.StatusOK, run(ShakeCloudPhone, http.MethodPost, nil).Code)
	assert.Equal(t, http.StatusOK, run(InstalledAppsCloudPhone, http.MethodGet, nil).Code)
	assert.Equal(t, http.StatusOK, run(InstallAppCloudPhone, http.MethodPost, map[string]any{"appIds": []int64{7}}).Code)
	assert.Equal(t, http.StatusOK, run(UninstallAppCloudPhone, http.MethodPost, map[string]any{"appIds": []int64{7}}).Code)
	assert.Equal(t, http.StatusOK, run(StartAppCloudPhone, http.MethodPost, map[string]any{"packageNames": []string{"com.demo"}}).Code)
	assert.Equal(t, http.StatusOK, run(StopAppCloudPhone, http.MethodPost, map[string]any{"packageNames": []string{"com.demo"}}).Code)
	assert.Equal(t, http.StatusOK, run(KillAllAppsCloudPhone, http.MethodPost, nil).Code)
	assert.Equal(t, http.StatusOK, run(AdbInfoCloudPhone, http.MethodGet, nil).Code)
	assert.Equal(t, http.StatusOK, run(EnableAdbCloudPhone, http.MethodPost, nil).Code)
	assert.Equal(t, http.StatusOK, run(DisableAdbCloudPhone, http.MethodPost, nil).Code)
	// rooted[cp-op]=true 且实时态 NORMAL → 关闭 root（enable=false）走中台成功。
	assert.Equal(t, http.StatusOK, run(RootCloudPhone, http.MethodPost, map[string]any{"enable": false}).Code)
	assert.Equal(t, http.StatusOK, run(RunLogsCloudPhone, http.MethodGet, nil).Code)
	assert.Equal(t, http.StatusOK, run(RuntimeCloudPhone, http.MethodGet, nil).Code)

	// 文件管理
	assert.Equal(t, http.StatusOK, run(FileListCloudPhone, http.MethodPost, map[string]any{"path": "/sdcard"}).Code)
	assert.Equal(t, http.StatusOK, run(FileDownloadCloudPhone, http.MethodPost, map[string]any{"path": "/sdcard/a.txt"}).Code)
	assert.Equal(t, http.StatusOK, run(FileDeleteCloudPhone, http.MethodPost, map[string]any{"paths": []string{"/sdcard/a.txt"}}).Code)

	// FileList 默认路径（空 path）。
	assert.Equal(t, http.StatusOK, run(FileListCloudPhone, http.MethodPost, map[string]any{}).Code)
}

// Power handler：需开机门禁放行（绑代理 + 有时长 + 实时态 STOPPED）。
func TestPowerHandler(t *testing.T) {
	opCleanup(t)
	require.NoError(t, billing.GrantRuntimeMinutesWalletForTest(userA, 1000))
	f := &fakePort{statuses: map[string]string{"cp-pw": "STOPPED"}}
	withFakeOps(t, f)
	p := CloudPhone{UserID: userA, Name: "pw机", Status: StatusStopped, CpID: "cp-pw", ProxyID: 1}
	require.NoError(t, framework.DB.Create(&p).Error)
	idStr := strconv.Itoa(int(p.ID))

	c, w := ctxFor(t, http.MethodPost, userA, map[string]any{"operation": "开机"}, "id", idStr)
	PowerCloudPhone(c)
	assert.Equal(t, http.StatusOK, w.Code)

	// 非法 operation → service 校验错误（400）。
	c, w = ctxFor(t, http.MethodPost, userA, map[string]any{"operation": "乱"}, "id", idStr)
	PowerCloudPhone(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Destroy handler：可销毁态走中台 destroy。
func TestDestroyHandler(t *testing.T) {
	opCleanup(t)
	f := &fakePort{statuses: map[string]string{"cp-de": "STOPPED"}}
	withFakeOps(t, f)
	p := CloudPhone{UserID: userA, Name: "de机", Status: StatusStopped, CpID: "cp-de", ProxyID: 1}
	require.NoError(t, framework.DB.Create(&p).Error)

	c, w := ctxFor(t, http.MethodPost, userA, nil, "id", strconv.Itoa(int(p.ID)))
	DestroyCloudPhone(c)
	assert.Equal(t, http.StatusOK, w.Code)
}

// 脚本 handler：运行中可发示例脚本 + 查任务状态。
func TestScriptHandlers(t *testing.T) {
	opCleanup(t)
	f := &fakePort{statuses: map[string]string{"cp-sc": MidplatReady}, scriptID: 42}
	withFakeOps(t, f)
	p := CloudPhone{UserID: userA, Name: "sc机", Status: StatusRunning, CpID: "cp-sc", ProxyID: 1}
	require.NoError(t, framework.DB.Create(&p).Error)
	idStr := strconv.Itoa(int(p.ID))

	c, w := ctxFor(t, http.MethodPost, userA, nil, "id", idStr)
	RunHelloScriptCloudPhone(c)
	require.Equal(t, http.StatusOK, w.Code)

	// 查状态
	c, w = ctxFor(t, http.MethodGet, userA, nil, "id", idStr, "taskId", "9001")
	ScriptTaskStatusCloudPhone(c)
	assert.Equal(t, http.StatusOK, w.Code)

	// taskId 非法 → 400
	c, w = ctxFor(t, http.MethodGet, userA, nil, "id", idStr, "taskId", "0")
	ScriptTaskStatusCloudPhone(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	c, w = ctxFor(t, http.MethodGet, userA, nil, "id", idStr, "taskId", "abc")
	ScriptTaskStatusCloudPhone(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// 未开通（无 cpId）实例操作 → service Validation 错误（422）。
func TestOpHandlerUnprovisioned(t *testing.T) {
	opCleanup(t)
	withFakeOps(t, &fakePort{})
	p := CloudPhone{UserID: userA, Name: "未开通", Status: StatusCreated}
	require.NoError(t, framework.DB.Create(&p).Error)

	c, w := ctxFor(t, http.MethodPost, userA, nil, "id", strconv.Itoa(int(p.ID)))
	RestartCloudPhone(c)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

// 越权：操作他人实例 → 404。
func TestOpHandlerForbidden(t *testing.T) {
	opCleanup(t)
	withFakeOps(t, &fakePort{})
	p := CloudPhone{UserID: userA, Name: "A机", Status: StatusRunning, CpID: "cp-own", ProxyID: 1}
	require.NoError(t, framework.DB.Create(&p).Error)

	c, w := ctxFor(t, http.MethodPost, userB, nil, "id", strconv.Itoa(int(p.ID)))
	RestartCloudPhone(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// Volume / Root：缺必填字段 → 400（ShouldBindJSON 失败）。
func TestOpHandlerBindError(t *testing.T) {
	opCleanup(t)
	withFakeOps(t, &fakePort{})
	p := CloudPhone{UserID: userA, Name: "x", Status: StatusRunning, CpID: "cp-bind", ProxyID: 1}
	require.NoError(t, framework.DB.Create(&p).Error)
	idStr := strconv.Itoa(int(p.ID))

	// 非法 JSON body
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("{bad"))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("userID", userA)
	c.Params = gin.Params{{Key: "id", Value: idStr}}
	VolumeCloudPhone(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("{bad"))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("userID", userA)
	c.Params = gin.Params{{Key: "id", Value: idStr}}
	RootCloudPhone(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// FileDownload：空 path → 400。
func TestFileDownloadEmptyPath(t *testing.T) {
	opCleanup(t)
	withFakeOps(t, &fakePort{})
	p := CloudPhone{UserID: userA, Name: "x", Status: StatusRunning, CpID: "cp-fd", ProxyID: 1}
	require.NoError(t, framework.DB.Create(&p).Error)

	c, w := ctxFor(t, http.MethodPost, userA, map[string]any{"path": ""}, "id", strconv.Itoa(int(p.ID)))
	FileDownloadCloudPhone(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// FileUpload：成功路径（multipart） + 非 multipart → 400 + 无文件 → 400。
func TestFileUploadHandler(t *testing.T) {
	opCleanup(t)
	withFakeOps(t, &fakePort{})
	p := CloudPhone{UserID: userA, Name: "x", Status: StatusRunning, CpID: "cp-up", ProxyID: 1}
	require.NoError(t, framework.DB.Create(&p).Error)
	idStr := strconv.Itoa(int(p.ID))

	// 成功：带一个文件。
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	mw.WriteField("folderPath", "/sdcard/Download")
	fw, err := mw.CreateFormFile("files", "a.txt")
	require.NoError(t, err)
	fw.Write([]byte("hello"))
	require.NoError(t, mw.Close())

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", body)
	c.Request.Header.Set("Content-Type", mw.FormDataContentType())
	c.Set("userID", userA)
	c.Params = gin.Params{{Key: "id", Value: idStr}}
	FileUploadCloudPhone(c)
	assert.Equal(t, http.StatusOK, w.Code)

	// 非 multipart → 400。
	c, w = ctxFor(t, http.MethodPost, userA, map[string]any{"x": 1}, "id", idStr)
	FileUploadCloudPhone(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// multipart 但无文件 → 400。
	body2 := &bytes.Buffer{}
	mw2 := multipart.NewWriter(body2)
	mw2.WriteField("folderPath", "/sdcard")
	require.NoError(t, mw2.Close())
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", body2)
	c.Request.Header.Set("Content-Type", mw2.FormDataContentType())
	c.Set("userID", userA)
	c.Params = gin.Params{{Key: "id", Value: idStr}}
	FileUploadCloudPhone(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
