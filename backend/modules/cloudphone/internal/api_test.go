package cloudphone

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"manager-backend/framework"
	"manager-backend/framework/midplat"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// ----------------------------------------------------------------------------
// fake http.RoundTripper：按请求路径路由，返回中台统一包络 {code,message,data}。
// 通过 midplat.Client.SetHTTPClient 注入，绝不真打中台。
// ----------------------------------------------------------------------------

type fakeRT struct {
	// handler 按请求路径返回 (status, body)。返回 status<=0 表示该路径未注册 → 404。
	handler func(r *http.Request) (int, string)
}

func (f *fakeRT) RoundTrip(r *http.Request) (*http.Response, error) {
	status, body := f.handler(r)
	if status == 0 {
		status, body = http.StatusNotFound, `{"code":"404","message":"not found"}`
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
		Request:    r,
	}, nil
}

// errRT 模拟传输层错误（连接失败），用于触发 client 方法返回 error → handler 502。
type errRT struct{}

func (errRT) RoundTrip(r *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("dial tcp: connection refused")
}

// env 构造一个成功包络（code 空串视为成功），data 为传入对象的 JSON。
func env(data any) string {
	raw, _ := json.Marshal(data)
	b, _ := json.Marshal(midplat.Envelope{Code: "0", Message: "ok", Data: raw})
	return string(b)
}

// newClient 用注入的 RoundTripper 构造一个 *midplat.Client。
func newClient(rt http.RoundTripper) *midplat.Client {
	c, err := midplat.New(midplat.Config{
		BaseURL:     "http://midplat.test",
		AccessKey:   "ak",
		SecretKey:   "sk",
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		panic(err)
	}
	c.SetHTTPClient(&http.Client{Transport: rt, Timeout: 5 * time.Second})
	return c
}

// withClient 临时替换包级 client，测试结束后恢复（默认 nil）。
func withClient(c *midplat.Client, fn func()) {
	prev := client
	client = c
	defer func() { client = prev }()
	fn()
}

// pathRT 把多个路径前缀映射到固定响应，便于聚合 handler 多端点造数据。
func pathRT(routes map[string]func() (int, string)) *fakeRT {
	return &fakeRT{handler: func(r *http.Request) (int, string) {
		for prefix, fn := range routes {
			if strings.Contains(r.URL.Path, prefix) {
				return fn()
			}
		}
		return 0, ""
	}}
}

// doGET 走指定 handler，返回 recorder。query 形如 "kind=vm&zoneId=1"。
func doGET(handler gin.HandlerFunc, query string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	u := "/x"
	if query != "" {
		u += "?" + query
	}
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.URL.RawQuery = query
	c.Request = req
	handler(c)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) framework.Response {
	t.Helper()
	var resp framework.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode body %q: %v", w.Body.String(), err)
	}
	return resp
}

// dataList 把 resp.Data 还原成 []map（便于断言聚合行数/字段）。
func dataList(t *testing.T, resp framework.Response) []map[string]any {
	t.Helper()
	raw, _ := json.Marshal(resp.Data)
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("data not a list: %v (%s)", err, string(raw))
	}
	return out
}

// ----------------------------------------------------------------------------
// 三态测试辅助：client==nil → 503；client 传输错误 → 502；OK → 200。
// ----------------------------------------------------------------------------

func assertNotConfigured(t *testing.T, handler gin.HandlerFunc, query string) {
	t.Helper()
	withClient(nil, func() {
		w := doGET(handler, query)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("client==nil 应 503，得 %d body=%s", w.Code, w.Body.String())
		}
	})
}

func assertGateway(t *testing.T, handler gin.HandlerFunc, query string) {
	t.Helper()
	withClient(newClient(errRT{}), func() {
		w := doGET(handler, query)
		if w.Code != http.StatusBadGateway {
			t.Fatalf("中台错误应 502，得 %d body=%s", w.Code, w.Body.String())
		}
	})
}

// ----------------------------------------------------------------------------
// ListZones
// ----------------------------------------------------------------------------

func TestListZones_NotConfigured(t *testing.T) { assertNotConfigured(t, ListZones, "") }
func TestListZones_Gateway(t *testing.T)       { assertGateway(t, ListZones, "") }

func TestListZones_OK(t *testing.T) {
	rt := pathRT(map[string]func() (int, string){
		"/avail-zone/query/all": func() (int, string) {
			return 200, env([]midplat.AvailZone{{ID: 1, Name: "zoneA"}, {ID: 2, Name: "zoneB"}})
		},
	})
	withClient(newClient(rt), func() {
		w := doGET(ListZones, "")
		if w.Code != 200 {
			t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
		}
		rows := dataList(t, decode(t, w))
		if len(rows) != 2 || rows[0]["name"] != "zoneA" {
			t.Fatalf("unexpected zones: %+v", rows)
		}
	})
}

// ----------------------------------------------------------------------------
// ListSpecs：默认 phone，多 zone 聚合；kind=vm 分支；单 zone 失败跳过；zoneId 过滤。
// ----------------------------------------------------------------------------

func TestListSpecs_NotConfigured(t *testing.T) { assertNotConfigured(t, ListSpecs, "") }

func TestListSpecs_ZonesError_Gateway(t *testing.T) {
	// ListSpecs 先取 allZones，若该步出错应 502。
	assertGateway(t, ListSpecs, "")
}

func TestListSpecs_PhoneAggregateAllZones(t *testing.T) {
	rt := phoneSpecsRT(
		map[int64]string{1: "zoneA", 2: "zoneB"},
		map[int64][]midplat.PhoneSpec{
			1: {{}},
			2: {{}, {}},
		}, nil)
	withClient(newClient(rt), func() {
		w := doGET(ListSpecs, "")
		if w.Code != 200 {
			t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
		}
		rows := dataList(t, decode(t, w))
		// zone1 贡献 1 条，zone2 贡献 2 条 → 共 3 条，每条带 zoneId/zoneName。
		if len(rows) != 3 {
			t.Fatalf("聚合应 3 条，得 %d: %+v", len(rows), rows)
		}
		// 校验补列 zoneName 存在。
		found := false
		for _, r := range rows {
			if r["zoneName"] == "zoneB" {
				found = true
			}
		}
		if !found {
			t.Fatalf("缺 zoneName=zoneB 的补列: %+v", rows)
		}
	})
}

func TestListSpecs_PhoneSkipFailingZone(t *testing.T) {
	// zone2 的规格查询返回错误（500）→ 应跳过，仅返回 zone1 的规格。
	rt := phoneSpecsRT(
		map[int64]string{1: "zoneA", 2: "zoneB"},
		map[int64][]midplat.PhoneSpec{1: {{}, {}}},
		map[int64]bool{2: true}, // zone2 失败
	)
	withClient(newClient(rt), func() {
		w := doGET(ListSpecs, "")
		if w.Code != 200 {
			t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
		}
		rows := dataList(t, decode(t, w))
		if len(rows) != 2 {
			t.Fatalf("坏区应跳过，仅 zone1 的 2 条，得 %d: %+v", len(rows), rows)
		}
	})
}

func TestListSpecs_ZoneIdFilter(t *testing.T) {
	rt := phoneSpecsRT(
		map[int64]string{1: "zoneA", 2: "zoneB"},
		map[int64][]midplat.PhoneSpec{1: {{}, {}, {}}, 2: {{}}},
		nil,
	)
	withClient(newClient(rt), func() {
		w := doGET(ListSpecs, "zoneId=1")
		rows := dataList(t, decode(t, w))
		if len(rows) != 3 {
			t.Fatalf("zoneId=1 应只返回该区 3 条，得 %d", len(rows))
		}
	})
}

func TestListSpecs_ZoneIdNotFound_StillQueries(t *testing.T) {
	// zoneId 不在 allZones 列表里：代码会构造 {ID: z} 仍去查（zoneName 为空）。
	rt := phoneSpecsRT(
		map[int64]string{1: "zoneA"},
		map[int64][]midplat.PhoneSpec{99: {{}}},
		nil,
	)
	withClient(newClient(rt), func() {
		w := doGET(ListSpecs, "zoneId=99")
		if w.Code != 200 {
			t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
		}
		rows := dataList(t, decode(t, w))
		if len(rows) != 1 {
			t.Fatalf("未知 zoneId 仍应去查到 1 条，得 %d: %+v", len(rows), rows)
		}
	})
}

func TestListSpecs_VMKind(t *testing.T) {
	rt := vmSpecsRT(
		map[int64]string{1: "zoneA", 2: "zoneB"},
		map[int64][]midplat.VirtualSpec{1: {{}}, 2: {{}, {}}},
		nil,
	)
	withClient(newClient(rt), func() {
		w := doGET(ListSpecs, "kind=vm")
		if w.Code != 200 {
			t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
		}
		rows := dataList(t, decode(t, w))
		if len(rows) != 3 {
			t.Fatalf("vm 聚合应 3 条，得 %d", len(rows))
		}
	})
}

func TestListSpecs_VMSkipFailingZone(t *testing.T) {
	rt := vmSpecsRT(
		map[int64]string{1: "zoneA", 2: "zoneB"},
		map[int64][]midplat.VirtualSpec{1: {{}}},
		map[int64]bool{2: true},
	)
	withClient(newClient(rt), func() {
		w := doGET(ListSpecs, "kind=vm")
		rows := dataList(t, decode(t, w))
		if len(rows) != 1 {
			t.Fatalf("vm 坏区应跳过，得 %d", len(rows))
		}
	})
}

// ----------------------------------------------------------------------------
// ListVMs / ListVMEnums / ListImageList
// ----------------------------------------------------------------------------

func TestListVMs_NotConfigured(t *testing.T) { assertNotConfigured(t, ListVMs, "") }
func TestListVMs_Gateway(t *testing.T)       { assertGateway(t, ListVMs, "") }

func TestListVMs_OK(t *testing.T) {
	var gotBody []byte
	rt := &fakeRT{handler: func(r *http.Request) (int, string) {
		if strings.Contains(r.URL.Path, "/server/page") {
			gotBody, _ = io.ReadAll(r.Body)
			return 200, env(map[string]any{
				"data":      []midplat.Server{{}, {}},
				"totalSize": 2,
			})
		}
		return 0, ""
	}}
	withClient(newClient(rt), func() {
		w := doGET(ListVMs, "status=RUNNING&vmUid=abc&vmIp=1.2.3.4")
		if w.Code != 200 {
			t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
		}
		rows := dataList(t, decode(t, w))
		if len(rows) != 2 {
			t.Fatalf("want 2 servers got %d", len(rows))
		}
		// 校验过滤参数透传到请求体。
		if !bytes.Contains(gotBody, []byte("RUNNING")) || !bytes.Contains(gotBody, []byte("abc")) {
			t.Fatalf("过滤参数未透传: %s", string(gotBody))
		}
	})
}

func TestListVMEnums_NotConfigured(t *testing.T) { assertNotConfigured(t, ListVMEnums, "") }
func TestListVMEnums_Gateway(t *testing.T)       { assertGateway(t, ListVMEnums, "") }

func TestListVMEnums_OK(t *testing.T) {
	rt := &fakeRT{handler: func(r *http.Request) (int, string) {
		return 200, env([]map[string]string{{"code": "RUNNING", "name": "运行中"}})
	}}
	withClient(newClient(rt), func() {
		w := doGET(ListVMEnums, "")
		if w.Code != 200 {
			t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestListImageList_NotConfigured(t *testing.T) { assertNotConfigured(t, ListImageList, "") }
func TestListImageList_Gateway(t *testing.T)       { assertGateway(t, ListImageList, "") }

func TestListImageList_OK(t *testing.T) {
	rt := &fakeRT{handler: func(r *http.Request) (int, string) {
		return 200, env([]midplat.ImageInfo{{}, {}, {}})
	}}
	withClient(newClient(rt), func() {
		w := doGET(ListImageList, "")
		if w.Code != 200 {
			t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
		}
		rows := dataList(t, decode(t, w))
		if len(rows) != 3 {
			t.Fatalf("want 3 images got %d", len(rows))
		}
	})
}

// ----------------------------------------------------------------------------
// ListBootPlans：specId<=0 → 400；OK；502。
// ----------------------------------------------------------------------------

func TestListBootPlans_NotConfigured(t *testing.T) { assertNotConfigured(t, ListBootPlans, "specId=1") }

func TestListBootPlans_BadSpecId(t *testing.T) {
	rt := pathRT(map[string]func() (int, string){})
	withClient(newClient(rt), func() {
		for _, q := range []string{"", "specId=0", "specId=-3", "specId=abc"} {
			w := doGET(ListBootPlans, q)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("specId=%q 应 400，得 %d", q, w.Code)
			}
		}
	})
}

func TestListBootPlans_Gateway(t *testing.T) {
	assertGateway(t, ListBootPlans, "specId=5")
}

func TestListBootPlans_OK(t *testing.T) {
	rt := &fakeRT{handler: func(r *http.Request) (int, string) {
		if strings.Contains(r.URL.Path, "/plan-mng/list/by-spec-and-scenario") {
			return 200, env([]midplat.BootPlan{{ID: 10}, {ID: 11}})
		}
		return 0, ""
	}}
	withClient(newClient(rt), func() {
		w := doGET(ListBootPlans, "specId=5")
		if w.Code != 200 {
			t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
		}
		rows := dataList(t, decode(t, w))
		if len(rows) != 2 {
			t.Fatalf("want 2 plans got %d", len(rows))
		}
	})
}

// ----------------------------------------------------------------------------
// ListSpecImages：specId<=0 → 400；去重；单 plan 失败跳过；OK；502。
// ----------------------------------------------------------------------------

func TestListSpecImages_NotConfigured(t *testing.T) {
	assertNotConfigured(t, ListSpecImages, "specId=1")
}

func TestListSpecImages_BadSpecId(t *testing.T) {
	withClient(newClient(pathRT(nil)), func() {
		w := doGET(ListSpecImages, "specId=0")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("specId=0 应 400，得 %d", w.Code)
		}
	})
}

func TestListSpecImages_PlansError_Gateway(t *testing.T) {
	// by-spec 查询出错 → 502。
	assertGateway(t, ListSpecImages, "specId=5")
}

func TestListSpecImages_DedupAndSkip(t *testing.T) {
	// plan 10 → imgA,imgB；plan 11 查询失败（跳过）；plan 12 → imgB(重复),imgC。
	// 期望去重后：imgA,imgB,imgC 共 3 条。
	rt := &fakeRT{handler: func(r *http.Request) (int, string) {
		p := r.URL.Path
		switch {
		case strings.Contains(p, "/plan-mng/list/by-spec-and-scenario"):
			return 200, env([]midplat.BootPlan{{ID: 10}, {ID: 11}, {ID: 12}})
		case strings.Contains(p, "/img/query/by-plan/10"):
			return 200, env([]midplat.PlanImage{
				{ImageID: "imgA", ImageName: "A"},
				{ImageID: "imgB", ImageName: "B"},
			})
		case strings.Contains(p, "/img/query/by-plan/11"):
			return 500, `{"code":"500","message":"boom"}` // 失败 → 跳过
		case strings.Contains(p, "/img/query/by-plan/12"):
			return 200, env([]midplat.PlanImage{
				{ImageID: "imgB", ImageName: "B"}, // 重复
				{ImageID: "imgC", ImageName: "C"},
			})
		}
		return 0, ""
	}}
	withClient(newClient(rt), func() {
		w := doGET(ListSpecImages, "specId=5")
		if w.Code != 200 {
			t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
		}
		rows := dataList(t, decode(t, w))
		if len(rows) != 3 {
			t.Fatalf("去重后应 3 条(imgA/imgB/imgC)，得 %d: %+v", len(rows), rows)
		}
		ids := map[string]bool{}
		for _, r := range rows {
			ids[fmt.Sprint(r["imageId"])] = true
		}
		if !ids["imgA"] || !ids["imgB"] || !ids["imgC"] {
			t.Fatalf("去重结果缺项: %+v", ids)
		}
	})
}

// ----------------------------------------------------------------------------
// ListAppList：分页 + appName 过滤；三态。
// ----------------------------------------------------------------------------

func TestListAppList_NotConfigured(t *testing.T) { assertNotConfigured(t, ListAppList, "") }
func TestListAppList_Gateway(t *testing.T)       { assertGateway(t, ListAppList, "") }

func TestListAppList_OK(t *testing.T) {
	var gotBody []byte
	rt := &fakeRT{handler: func(r *http.Request) (int, string) {
		if strings.Contains(r.URL.Path, "/app/page") {
			gotBody, _ = io.ReadAll(r.Body)
			return 200, env(map[string]any{
				"data":      []midplat.AppInfo{{}, {}},
				"totalSize": 7,
			})
		}
		return 0, ""
	}}
	withClient(newClient(rt), func() {
		w := doGET(ListAppList, "page=2&size=50&appName=wechat")
		if w.Code != 200 {
			t.Fatalf("want 200 got %d: %s", w.Code, w.Body.String())
		}
		resp := decode(t, w)
		// OKWithPage → data = {list, total}
		dataMap, ok := resp.Data.(map[string]any)
		if !ok {
			t.Fatalf("data 非分页对象: %+v", resp.Data)
		}
		if total, _ := dataMap["total"].(float64); int(total) != 7 {
			t.Fatalf("total 应为 7，得 %v", dataMap["total"])
		}
		// 校验 appName / page / pageSize 透传。
		if !bytes.Contains(gotBody, []byte("wechat")) {
			t.Fatalf("appName 未透传: %s", string(gotBody))
		}
	})
}

func TestListAppList_DefaultPaging(t *testing.T) {
	var gotBody []byte
	rt := &fakeRT{handler: func(r *http.Request) (int, string) {
		gotBody, _ = io.ReadAll(r.Body)
		return 200, env(map[string]any{"data": []midplat.AppInfo{}, "totalSize": 0})
	}}
	withClient(newClient(rt), func() {
		w := doGET(ListAppList, "")
		if w.Code != 200 {
			t.Fatalf("want 200 got %d", w.Code)
		}
		// 默认 page=1 size=200。
		if !bytes.Contains(gotBody, []byte(`"page":1`)) || !bytes.Contains(gotBody, []byte(`"pageSize":200`)) {
			t.Fatalf("默认分页未生效: %s", string(gotBody))
		}
	})
}

// ----------------------------------------------------------------------------
// RoundTripper 构造器：根据 zone→specs 映射，按路径返回 avail-zone / phone-spec / vm-spec。
// fail 标记某 zone 的 spec 查询返回 500（用于「单 zone 失败跳过」）。
// ----------------------------------------------------------------------------

// phone/vm 规格按 zoneId 查询，zoneId 走 query string（见 midplat.ListPhoneSpecsByZone）。
func isPhoneSpecPath(p string) bool { return strings.Contains(p, "/phone-spec/query/by-zone") }
func isVMSpecPath(p string) bool    { return strings.Contains(p, "/virtual-spec/query/by-zone") }

func availZonesBody(names map[int64]string) string {
	var zs []midplat.AvailZone
	for id, name := range names {
		zs = append(zs, midplat.AvailZone{ID: id, Name: name})
	}
	return env(zs)
}

func phoneSpecsRT(zoneNames map[int64]string, specs map[int64][]midplat.PhoneSpec, fail map[int64]bool) *fakeRT {
	return &fakeRT{handler: func(r *http.Request) (int, string) {
		p := r.URL.Path
		if strings.Contains(p, "/avail-zone/query/all") {
			return 200, availZonesBody(zoneNames)
		}
		if isPhoneSpecPath(p) {
			z := zoneIDFromQuery(r)
			if fail[z] {
				return 500, `{"code":"500","message":"zone down"}`
			}
			return 200, env(specs[z])
		}
		return 0, ""
	}}
}

func vmSpecsRT(zoneNames map[int64]string, specs map[int64][]midplat.VirtualSpec, fail map[int64]bool) *fakeRT {
	return &fakeRT{handler: func(r *http.Request) (int, string) {
		p := r.URL.Path
		if strings.Contains(p, "/avail-zone/query/all") {
			return 200, availZonesBody(zoneNames)
		}
		if isVMSpecPath(p) {
			z := zoneIDFromQuery(r)
			if fail[z] {
				return 500, `{"code":"500","message":"zone down"}`
			}
			return 200, env(specs[z])
		}
		return 0, ""
	}}
}

func zoneIDFromQuery(r *http.Request) int64 {
	q, _ := url.ParseQuery(r.URL.RawQuery)
	for _, k := range []string{"zoneId", "availZoneId"} {
		if v := q.Get(k); v != "" {
			var n int64
			fmt.Sscan(v, &n)
			return n
		}
	}
	return 0
}
