package phone

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"manager-backend/framework/midplat"

	"github.com/stretchr/testify/require"
)

// 回归测试：单台 start/stop/killAll 应用时，中台 HTTP 200 但目标 cp 落在失败列表，
// 适配器必须识别为业务失败并返回错误（C1，防"失败列表被丢弃、失败当成功"）。

func newTestAdapter(t *testing.T, respBody string) *sdkAdapter {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)
	cli, err := midplat.New(midplat.Config{BaseURL: srv.URL, AccessKey: "ak", SecretKey: "sk"})
	require.NoError(t, err)
	return &sdkAdapter{c: cli}
}

func TestStartApp_FailedListReportsError(t *testing.T) {
	a := newTestAdapter(t, `{"code":"0","data":{"startFailedCpIds":["cp1"]}}`)
	err := a.StartApp(context.Background(), "cp1", nil, []string{"pkg"})
	require.Error(t, err, "目标 cp 在启动失败列表中应返回错误")
}

func TestStartApp_SuccessNoError(t *testing.T) {
	a := newTestAdapter(t, `{"code":"0","data":{"startFailedCpIds":[]}}`)
	err := a.StartApp(context.Background(), "cp1", nil, []string{"pkg"})
	require.NoError(t, err, "目标 cp 不在失败列表应成功")
}

func TestStopApp_FailedListReportsError(t *testing.T) {
	a := newTestAdapter(t, `{"code":"0","data":{"stopFailedCpIds":["cp1"]}}`)
	err := a.StopApp(context.Background(), "cp1", nil, []string{"pkg"})
	require.Error(t, err, "目标 cp 在停止失败列表中应返回错误")
}

func TestKillAllApps_FailedListReportsError(t *testing.T) {
	a := newTestAdapter(t, `{"code":"0","data":{"containers":["cp1"]}}`)
	err := a.KillAllApps(context.Background(), "cp1")
	require.Error(t, err, "目标 cp 在 killall 失败列表中应返回错误")
}

func TestKillAllApps_SuccessNoError(t *testing.T) {
	a := newTestAdapter(t, `{"code":"0","data":{"containers":[]}}`)
	err := a.KillAllApps(context.Background(), "cp1")
	require.NoError(t, err, "目标 cp 不在失败列表应成功")
}
