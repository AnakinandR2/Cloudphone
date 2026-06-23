package phone

import (
	"bytes"
	"encoding/json"
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

// ctxFor 构造一台带认证 userID + 路径参数的 gin.Context（不经路由，直接喂 handler，计入覆盖率）。
// body 非空则以 JSON 写入请求体；params 是路由参数键值对（如 "id","5"）。
func ctxFor(t *testing.T, method string, uid int, body any, params ...string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, "/", reader)
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	if uid > 0 {
		c.Set("userID", uid)
	}
	for i := 0; i+1 < len(params); i += 2 {
		c.Params = append(c.Params, gin.Param{Key: params[i], Value: params[i+1]})
	}
	return c, w
}

// decodeResp 解析统一响应体，返回 code 与 data。
func decodeResp(t *testing.T, w *httptest.ResponseRecorder) (int, json.RawMessage) {
	t.Helper()
	var resp struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp.Code, resp.Data
}

func apiCleanup(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("cloud_phones", "cp_tasks",
			"billing_license_units", "billing_ledger_entries",
			"billing_seat_usages", "billing_dunning_states",
			"billing_entitlement_batches", "billing_runtime_minute_wallets")
	})
}

// --- 认证缺失：所有需要 userID 的 handler 都应 401 ---

func TestHandlersRejectUnauthenticated(t *testing.T) {
	cases := map[string]gin.HandlerFunc{
		"list":          GetCloudPhoneList,
		"get":           GetCloudPhone,
		"create":        CreateCloudPhone,
		"update":        UpdateCloudPhone,
		"delete":        DeleteCloudPhone,
		"recycleList":   RecycleBinList,
		"recycleRestor": RecycleBinRestore,
		"listTags":      ListPhoneTags,
		"setTags":       SetPhoneTags,
		"power":         PowerCloudPhone,
		"restart":       RestartCloudPhone,
		"destroy":       DestroyCloudPhone,
	}
	for name, h := range cases {
		c, w := ctxFor(t, http.MethodGet, 0, nil, "id", "1")
		h(c)
		assert.Equal(t, http.StatusUnauthorized, w.Code, name+" 应 401")
	}
}

// --- parseID：非数字 id → 400 ---

func TestHandlersRejectBadID(t *testing.T) {
	c, w := ctxFor(t, http.MethodGet, userA, nil, "id", "abc")
	GetCloudPhone(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	c, w = ctxFor(t, http.MethodDelete, userA, nil, "id", "xyz")
	DeleteCloudPhone(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	c, w = ctxFor(t, http.MethodPost, userA, nil, "id", "no")
	RestartCloudPhone(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- 前台 CRUD handler 成功路径 ---

func TestCreateGetUpdateDeleteHandlers(t *testing.T) {
	apiCleanup(t)
	require.NoError(t, billing.GrantSeatLicensesForTest(userA, 5))
	withFakeOps(t, nil) // 无中台降级：Create 直接落 CREATED

	// Create
	c, w := ctxFor(t, http.MethodPost, userA, CloudPhoneCreate{Name: "H机", ProxyID: 3})
	CreateCloudPhone(c)
	require.Equal(t, http.StatusOK, w.Code)
	code, data := decodeResp(t, w)
	assert.Equal(t, 0, code)
	var created CloudPhone
	require.NoError(t, json.Unmarshal(data, &created))
	assert.NotZero(t, created.ID)
	idStr := strconv.Itoa(int(created.ID))

	// Get
	c, w = ctxFor(t, http.MethodGet, userA, nil, "id", idStr)
	GetCloudPhone(c)
	require.Equal(t, http.StatusOK, w.Code)
	_, data = decodeResp(t, w)
	var got CloudPhone
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, "H机", got.Name)

	// Update
	c, w = ctxFor(t, http.MethodPut, userA, CloudPhoneUpdate{Name: "H机2", Status: StatusStopped}, "id", idStr)
	UpdateCloudPhone(c)
	require.Equal(t, http.StatusOK, w.Code)
	_, data = decodeResp(t, w)
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, "H机2", got.Name)

	// List
	c, w = ctxFor(t, http.MethodGet, userA, nil)
	c.Request = httptest.NewRequest(http.MethodGet, "/?page=1&size=10", nil)
	c.Set("userID", userA)
	GetCloudPhoneList(c)
	require.Equal(t, http.StatusOK, w.Code)

	// Delete（STOPPED 可删）
	c, w = ctxFor(t, http.MethodDelete, userA, nil, "id", idStr)
	DeleteCloudPhone(c)
	require.Equal(t, http.StatusOK, w.Code)
}

// Create：缺席位 → service 错误经 FailErr 映射非 200。
func TestCreateHandlerSeatError(t *testing.T) {
	apiCleanup(t)
	withFakeOps(t, nil)
	c, w := ctxFor(t, http.MethodPost, 95011, CloudPhoneCreate{Name: "x"})
	CreateCloudPhone(c)
	assert.NotEqual(t, http.StatusOK, w.Code, "无席位创建应失败")
}

// Create：请求体非法 JSON → 400。
func TestCreateHandlerBadJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("{bad"))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("userID", userA)
	CreateCloudPhone(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Get：不存在 → 404。
func TestGetHandlerNotFound(t *testing.T) {
	apiCleanup(t)
	withFakeOps(t, &fakePort{})
	c, w := ctxFor(t, http.MethodGet, userA, nil, "id", "999999")
	GetCloudPhone(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- 回收站 handler ---

func TestRecycleBinHandlers(t *testing.T) {
	apiCleanup(t)
	withFakeOps(t, &fakeOps{})

	c, w := ctxFor(t, http.MethodGet, userA, nil)
	RecycleBinList(c)
	assert.Equal(t, http.StatusOK, w.Code)

	// Restore 不存在的实例 → 非 200。
	c, w = ctxFor(t, http.MethodPost, userA, nil, "id", "888888")
	RecycleBinRestore(c)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

// --- 标签 handler ---

func TestTagHandlers(t *testing.T) {
	apiCleanup(t)
	require.NoError(t, billing.GrantSeatLicensesForTest(userA, 5))
	withFakeOps(t, nil)

	p := CloudPhone{UserID: userA, Name: "tag机", Status: StatusCreated}
	require.NoError(t, framework.DB.Create(&p).Error)

	// SetTags
	body := map[string]any{"ids": []int{int(p.ID)}, "tags": []Tag{{Name: "电商"}}}
	c, w := ctxFor(t, http.MethodPost, userA, body)
	SetPhoneTags(c)
	require.Equal(t, http.StatusOK, w.Code)

	// ListTags
	c, w = ctxFor(t, http.MethodGet, userA, nil)
	ListPhoneTags(c)
	require.Equal(t, http.StatusOK, w.Code)
	_, data := decodeResp(t, w)
	var tags []Tag
	require.NoError(t, json.Unmarshal(data, &tags))
	require.Len(t, tags, 1)
	assert.Equal(t, "电商", tags[0].Name)

	// SetTags 空 ids → 校验错误。
	c, w = ctxFor(t, http.MethodPost, userA, map[string]any{"ids": []int{}, "tags": []Tag{}})
	SetPhoneTags(c)
	assert.NotEqual(t, http.StatusOK, w.Code)
}

// --- 管理侧 handler（直接喂 context，绕过权限中间件；权限由 module 路由层保障） ---

func TestAdminHandlers(t *testing.T) {
	apiCleanup(t)
	require.NoError(t, billing.GrantSeatLicensesForTest(userA, 5))
	withFakeOps(t, nil)

	a := CloudPhone{UserID: userA, Name: "A实例", Status: StatusCreated}
	require.NoError(t, framework.DB.Create(&a).Error)
	b := CloudPhone{UserID: userB, Name: "B实例", Status: StatusCreated}
	require.NoError(t, framework.DB.Create(&b).Error)

	// AdminList 全量
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?page=1&size=20", nil)
	AdminListCloudPhones(c)
	require.Equal(t, http.StatusOK, w.Code)

	// AdminList 按属主过滤
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?userId="+strconv.Itoa(userA), nil)
	AdminListCloudPhones(c)
	require.Equal(t, http.StatusOK, w.Code)

	// AdminGet
	c, w = ctxFor(t, http.MethodGet, 0, nil, "id", strconv.Itoa(int(a.ID)))
	AdminGetCloudPhone(c)
	require.Equal(t, http.StatusOK, w.Code)

	// AdminGet 不存在 → 404
	c, w = ctxFor(t, http.MethodGet, 0, nil, "id", "777777")
	AdminGetCloudPhone(c)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// AdminGet 非法 id → 400
	c, w = ctxFor(t, http.MethodGet, 0, nil, "id", "nope")
	AdminGetCloudPhone(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// AdminListTags
	c, w = ctxFor(t, http.MethodGet, 0, nil)
	AdminListPhoneTags(c)
	require.Equal(t, http.StatusOK, w.Code)

	// AdminDelete
	c, w = ctxFor(t, http.MethodDelete, 0, nil, "id", strconv.Itoa(int(a.ID)))
	AdminDeleteCloudPhone(c)
	require.Equal(t, http.StatusOK, w.Code)

	// AdminDelete 非法 id → 400
	c, w = ctxFor(t, http.MethodDelete, 0, nil, "id", "bad")
	AdminDeleteCloudPhone(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
