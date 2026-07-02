package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProber 是 proberPort 的假实现：返回预设结果/错误，单测不打真网。
type fakeProber struct {
	res ProbeResult
	err error
}

func (f *fakeProber) Probe(_ context.Context, _ Proxy) (ProbeResult, error) {
	return f.res, f.err
}

// withFakeProber 临时把 ProxyService.prober 换成假实现，测试结束自动还原。
func withFakeProber(t *testing.T, f proberPort) {
	t.Helper()
	prev := ProxyService.prober
	ProxyService.prober = f
	t.Cleanup(func() { ProxyService.prober = prev })
}

const (
	userA = 2001
	userB = 2002
)

func sampleCreate(name, host string) *ProxyCreate {
	return &ProxyCreate{Name: name, Host: host, Port: 1080, Username: "u", Password: "p", Region: "US"}
}

func TestProxyCRUDOwnedByUser(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("proxies") })

	created, err := ProxyService.Create(userA, sampleCreate("px-a", "1.1.1.1"))
	require.NoError(t, err)
	assert.NotZero(t, created.ID)
	assert.Equal(t, uint(userA), created.UserID)
	assert.Equal(t, "socks5", created.Protocol)    // 默认协议
	assert.Equal(t, StatusUnknown, created.Status) // 默认未检测
	id := int(created.ID)

	got, err := ProxyService.GetByID(userA, id)
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1", got.Host)

	updated, err := ProxyService.Update(userA, id, &ProxyUpdate{Name: "px-a2", Port: 7890})
	require.NoError(t, err)
	assert.Equal(t, "px-a2", updated.Name)
	assert.Equal(t, 7890, updated.Port)

	require.NoError(t, ProxyService.Delete(userA, id))
	_, err = ProxyService.GetByID(userA, id)
	assert.Error(t, err)
}

// 核心：用户只能看/改/删自己的代理，访问他人代理一律「不存在」。
func TestProxyIsolationBetweenUsers(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("proxies") })

	a, err := ProxyService.Create(userA, sampleCreate("A代理", "10.0.0.1"))
	require.NoError(t, err)
	_, err = ProxyService.Create(userB, sampleCreate("B代理", "10.0.0.2"))
	require.NoError(t, err)

	listA, totalA, err := ProxyService.GetList(userA, 1, 10, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), totalA)
	require.Len(t, listA, 1)
	assert.Equal(t, "A代理", listA[0].Name)

	// B 看不到 / 改不动 / 删不掉 A 的代理
	_, err = ProxyService.GetByID(userB, int(a.ID))
	assert.Error(t, err)
	_, err = ProxyService.Update(userB, int(a.ID), &ProxyUpdate{Name: "篡改"})
	assert.Error(t, err)
	assert.Error(t, ProxyService.Delete(userB, int(a.ID)))

	still, err := ProxyService.GetByID(userA, int(a.ID))
	require.NoError(t, err)
	assert.Equal(t, "A代理", still.Name)
}

// 管理侧：看到全量代理池（跨用户），并能按运维删除任意代理。
func TestProxyAdminListAllAndDelete(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("proxies") })

	a, err := ProxyService.Create(userA, sampleCreate("A代理", "10.0.0.1"))
	require.NoError(t, err)
	_, err = ProxyService.Create(userB, sampleCreate("B代理", "10.0.0.2"))
	require.NoError(t, err)

	list, total, err := ProxyService.AdminList(1, 10, "", "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(2), total, "管理侧应看到全部用户的代理")
	assert.Len(t, list, 2)

	// 关键字过滤
	filtered, ftotal, err := ProxyService.AdminList(1, 10, "A代理", "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), ftotal)
	require.Len(t, filtered, 1)

	// 运维删除（不限属主）
	require.NoError(t, ProxyService.AdminDelete(int(a.ID)))
	_, total2, err := ProxyService.AdminList(1, 10, "", "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), total2)
}

// 可用代理下拉：不分页、仅本人、排除探测失败的。
func TestProxyListOptions(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("proxies") })

	ok1, err := ProxyService.Create(userA, sampleCreate("ok1", "1.1.1.1"))
	require.NoError(t, err)
	_, err = ProxyService.Create(userA, sampleCreate("ok2", "1.1.1.2"))
	require.NoError(t, err)
	bad, err := ProxyService.Create(userA, sampleCreate("bad", "1.1.1.3"))
	require.NoError(t, err)
	_, err = ProxyService.Create(userB, sampleCreate("b", "2.2.2.2")) // 他人代理不应混入
	require.NoError(t, err)

	// 把 bad 标记为 fail —— 应被 ListOptions 过滤掉
	_, err = ProxyService.Update(userA, int(bad.ID), &ProxyUpdate{}) // 占位（更新无副作用）
	require.NoError(t, err)
	require.NoError(t, framework.DB.Model(&Proxy{}).Where("id = ?", ok1.ID).Update("status", StatusFail).Error)

	opts, err := ProxyService.ListOptions(userA)
	require.NoError(t, err)
	assert.Len(t, opts, 2, "应只返回本人且非 fail 的代理")
	for _, p := range opts {
		assert.NotEqual(t, StatusFail, p.Status)
		assert.Equal(t, uint(userA), p.UserID)
	}
}

// 即时探测（表单测试用）：成功返回 ok + 各归属字段；失败返回 fail + message；不落库。
func TestProxyProbeAdHoc(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("proxies") })

	withFakeProber(t, &fakeProber{res: ProbeResult{
		EgressIP: "156.59.87.47", LatencyMs: 99, Country: "CN", City: "Hong Kong",
		ASN: "AS21859", ASNName: "Zenlayer Inc", Company: "Zenlayer", ConnType: "Corporate",
	}})
	out, err := ProxyService.Probe(&ProxyProbeRequest{Host: "1.2.3.4", Port: 1080})
	require.NoError(t, err)
	assert.Equal(t, StatusOK, out.Status)
	assert.Equal(t, 99, out.Latency)
	assert.Equal(t, "156.59.87.47", out.EgressIP)
	assert.Equal(t, "AS21859", out.ASN)
	assert.Equal(t, "Zenlayer Inc", out.ASNName)

	// 探测不落库：库里仍为空
	_, total, err := ProxyService.GetList(userA, 1, 10, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)

	// 失败：fail + message
	withFakeProber(t, &fakeProber{err: errors.New("connect refused")})
	out2, err := ProxyService.Probe(&ProxyProbeRequest{Host: "9.9.9.9", Port: 1})
	require.NoError(t, err)
	assert.Equal(t, StatusFail, out2.Status)
	assert.Contains(t, out2.Message, "connect refused")
}

// 批量导入：有效条目入库、无效（缺 host/port）跳过、全部归属当前用户。
func TestProxyBatchCreate(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("proxies") })

	items := []ProxyCreate{
		{Name: "a", Host: "1.1.1.1", Port: 1080},
		{Name: "b", Host: "2.2.2.2", Port: 1081, Username: "u", Password: "p"},
		{Name: "bad-no-host", Port: 1082},      // 无 host → 跳过
		{Name: "bad-no-port", Host: "3.3.3.3"}, // 无 port → 跳过
	}
	created, err := ProxyService.BatchCreate(userA, items)
	require.NoError(t, err)
	assert.Equal(t, 2, created)

	list, total, err := ProxyService.GetList(userA, 1, 50, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	for _, p := range list {
		assert.Equal(t, uint(userA), p.UserID)
		assert.Equal(t, "socks5", p.Protocol) // 默认协议
	}
	// 不串户：B 看不到 A 批量导入的
	_, totalB, err := ProxyService.GetList(userB, 1, 50, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(0), totalB)
}

// 测试代理：成功写回连通性 + 归属；失败记 fail；越权拒绝。
func TestProxyTestWritesResultAndIsolated(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("proxies") })

	created, err := ProxyService.Create(userA, sampleCreate("p", "1.1.1.1"))
	require.NoError(t, err)
	id := int(created.ID)

	// 成功：出口 IP + 延迟 + 归属（国家/城市/ASN/公司）写回
	withFakeProber(t, &fakeProber{res: ProbeResult{
		EgressIP: "156.59.87.47", LatencyMs: 128,
		Country: "CN", City: "Hong Kong", ASN: "AS21859", ASNName: "Zenlayer Inc",
		Company: "Zenlayer IP Block @Hong Kong", ConnType: "Corporate",
	}})
	got, err := ProxyService.TestProxy(userA, id)
	require.NoError(t, err)
	assert.Equal(t, StatusOK, got.Status)
	assert.Equal(t, 128, got.Latency)
	assert.Equal(t, "156.59.87.47", got.EgressIP)
	assert.Equal(t, "AS21859", got.ASN)
	assert.Equal(t, "Zenlayer Inc", got.ASNName)
	assert.Equal(t, "Corporate", got.ConnType)
	assert.Equal(t, "CN Hong Kong", got.Region) // 自动识别回填
	require.NotNil(t, got.LastCheckedAt)

	// 失败：记 fail，出口 IP 清空
	withFakeProber(t, &fakeProber{err: errors.New("dial timeout")})
	got2, err := ProxyService.TestProxy(userA, id)
	require.NoError(t, err)
	assert.Equal(t, StatusFail, got2.Status)
	assert.Empty(t, got2.EgressIP)

	// 越权：B 测 A 的代理 → 报错
	_, err = ProxyService.TestProxy(userB, id)
	assert.Error(t, err)
}

// 密码字段绝不应出现在 JSON 输出里（json:"-"）。
func TestProxyPasswordNeverSerialized(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("proxies") })

	const secret = "S3cr3t-PW-zzz"
	created, err := ProxyService.Create(userA, &ProxyCreate{Name: "px", Host: "2.2.2.2", Port: 1080, Password: secret})
	require.NoError(t, err)

	b, err := json.Marshal(created)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "\"password\"", "响应里不应有 password 字段")
	assert.NotContains(t, string(b), secret, "响应里不应泄露密码明文")
}
