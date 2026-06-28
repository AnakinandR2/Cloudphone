package billing

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// injectUser 返回一个中间件，把 userID 注入 gin.Context（模拟 user/staff 鉴权中间件结果）。
// uid<=0 表示不注入（模拟未授权）。
func injectUser(uid int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if uid > 0 {
			c.Set("userID", uid)
		}
		c.Next()
	}
}

// billingRouter 构造一个把全部 billing handler 直接挂载（不经真实鉴权中间件）的引擎，
// 由 inject 控制是否注入 userID。直接调用 handler 函数同样计入包覆盖率（见设计文档 §3.1）。
func billingRouter(inject gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/billing")
	g.Use(inject)
	{
		g.GET("/account", GetMyAccount)
		g.GET("/ledger", GetMyLedger)
		g.POST("/topup", Topup)
		g.GET("/trials", ListMyTrials)
		g.POST("/trials/:code/claim", ClaimTrial)
		g.GET("/overview", GetBillingOverview)
		g.GET("/purchase-config", GetPurchaseConfig)
		g.POST("/quote", BizQuote)
		g.GET("/license-units", ListLicenseUnits)
		g.POST("/orders", CreateBizOrder)
		g.GET("/orders", ListBizOrders)
		g.GET("/orders/:id", GetBizOrder)
		g.POST("/orders/:id/pay", PayBizOrder)
		g.GET("/runtime/log", GetRuntimeLog)
	}
	admin := r.Group("/admin/billing")
	admin.Use(inject)
	{
		admin.GET("/accounts/:userId", AdminGetAccount)
		admin.POST("/accounts/:userId/adjust", AdminAdjustBalance)
		admin.POST("/accounts/:userId/adjust-resource", AdminAdjustResourceV2)
		admin.GET("/trials", AdminListTrialPolicies)
		admin.POST("/trials", AdminCreateTrialPolicy)
		admin.PUT("/trials/:id", AdminUpdateTrialPolicy)
		admin.DELETE("/trials/:id", AdminDeleteTrialPolicy)
		admin.POST("/trials/:id/eligibility", AdminGrantTrialEligibility)
		admin.POST("/trials/:id/feature", AdminFeatureTrialPolicy)
		admin.GET("/trials/:id/grants", AdminListTrialGrants)
		admin.GET("/runtime-config", AdminGetRuntimePricing)
		admin.PUT("/runtime-config", AdminSaveRuntimePricing)
		admin.GET("/pricing", AdminGetPricing)
		admin.PUT("/pricing", AdminSavePricing)
		admin.GET("/payment-methods", AdminGetPaymentMethods)
		admin.PUT("/payment-methods", AdminSavePaymentMethods)
		admin.POST("/payment-methods/upload", UploadPayMethodLogo)
		admin.GET("/recharge-presets", AdminGetRechargePresets)
		admin.PUT("/recharge-presets", AdminSaveRechargePresets)
		admin.GET("/notices", AdminGetNotices)
		admin.PUT("/notices", AdminSaveNotices)
		admin.GET("/biz-orders", AdminListBizOrders)
		admin.GET("/biz-orders/:id", AdminGetBizOrder)
		admin.POST("/biz-orders/:id/mark-paid", AdminMarkBizOrderPaid)
	}
	return r
}

// do 发起请求并解出 envelope。
func do(t *testing.T, r *gin.Engine, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var rdr *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewBuffer(b)
	} else {
		rdr = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var env map[string]any
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &env)
	}
	return w.Code, env
}

// ---- 前台 handler ----

func TestGetMyAccount_Unauthorized(t *testing.T) {
	r := billingRouter(injectUser(0))
	code, _ := do(t, r, http.MethodGet, "/billing/account", nil)
	assert.Equal(t, http.StatusUnauthorized, code)
}

func TestGetMyAccount_OK(t *testing.T) {
	uid := 970101
	require.NoError(t, WalletService.TopUp(uid, 12345, "seed", "test"))
	r := billingRouter(injectUser(uid))
	code, env := do(t, r, http.MethodGet, "/billing/account", nil)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(0), env["code"])
	data := env["data"].(map[string]any)
	assert.Equal(t, float64(12345), data["balance_cents"])
}

func TestGetMyLedger_OK(t *testing.T) {
	uid := 970102
	require.NoError(t, WalletService.TopUp(uid, 500, "seed", "test"))
	r := billingRouter(injectUser(uid))
	code, env := do(t, r, http.MethodGet, "/billing/ledger?page=1&size=10", nil)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(0), env["code"])
}

func TestGetMyLedger_Unauthorized(t *testing.T) {
	r := billingRouter(injectUser(0))
	code, _ := do(t, r, http.MethodGet, "/billing/ledger", nil)
	assert.Equal(t, http.StatusUnauthorized, code)
}

func TestTopup_OK(t *testing.T) {
	uid := 970103
	r := billingRouter(injectUser(uid))
	code, env := do(t, r, http.MethodPost, "/billing/topup", TopupRequest{AmountCents: 999})
	assert.Equal(t, http.StatusOK, code)
	data := env["data"].(map[string]any)
	assert.Equal(t, float64(999), data["balance_cents"])
}

func TestTopup_BadRequest(t *testing.T) {
	r := billingRouter(injectUser(970104))
	code, _ := do(t, r, http.MethodPost, "/billing/topup", map[string]any{"amount_cents": 0})
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestTopup_Unauthorized(t *testing.T) {
	r := billingRouter(injectUser(0))
	code, _ := do(t, r, http.MethodPost, "/billing/topup", TopupRequest{AmountCents: 100})
	assert.Equal(t, http.StatusUnauthorized, code)
}

func TestGetBillingOverview_Unauthorized(t *testing.T) {
	r := billingRouter(injectUser(0))
	code, _ := do(t, r, http.MethodGet, "/billing/overview", nil)
	assert.Equal(t, http.StatusUnauthorized, code)
}

func TestGetBillingOverview_OK_WithFakePhonePort(t *testing.T) {
	t.Cleanup(func() {
		SetRunningInstanceCountProvider(nil)
		framework.CleanTable("billing_license_units", "billing_biz_orders", "billing_biz_order_items")
	})
	uid := 970105
	require.NoError(t, WalletService.TopUp(uid, 8888, "seed", "test"))
	// 给用户买 2 个 boot_slot（履约出名额），fake phone 端口返回运行中 5 台 → in_use 封顶为 2。
	require.NoError(t, FulfillService.FulfillNew(uid, KindBootSlot, 2, 1, SourceGrant, "test"))
	SetRunningInstanceCountProvider(func(userID int) int {
		if userID == uid {
			return 5
		}
		return 0
	})

	r := billingRouter(injectUser(uid))
	code, env := do(t, r, http.MethodGet, "/billing/overview", nil)
	assert.Equal(t, http.StatusOK, code)
	data := env["data"].(map[string]any)
	assert.Equal(t, float64(8888), data["balance_cents"])
	boot := data["boot_slot"].(map[string]any)
	assert.Equal(t, float64(2), boot["total"])
	assert.Equal(t, float64(2), boot["in_use"]) // 封顶到持有名额数
}

func TestGetPurchaseConfig_OK(t *testing.T) {
	r := billingRouter(injectUser(970106))
	code, env := do(t, r, http.MethodGet, "/billing/purchase-config", nil)
	assert.Equal(t, http.StatusOK, code)
	data := env["data"].(map[string]any)
	require.Contains(t, data, "kinds")
	require.Contains(t, data, "runtime_pack")
}

func TestBizQuote_OK(t *testing.T) {
	r := billingRouter(injectUser(970107))
	code, env := do(t, r, http.MethodPost, "/billing/quote", BizQuoteRequest{
		BizType: BizSeatNew, Quantity: 2, DurationValue: 3,
	})
	assert.Equal(t, http.StatusOK, code)
	data := env["data"].(map[string]any)
	assert.Equal(t, float64(2), data["quantity"])
	require.Contains(t, data, "payable_cents")
}

func TestBizQuote_BadRequest(t *testing.T) {
	r := billingRouter(injectUser(970108))
	// 缺少必填字段 → 绑定失败 400。
	code, _ := do(t, r, http.MethodPost, "/billing/quote", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestListLicenseUnits_OK(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_license_units") })
	uid := 970109
	require.NoError(t, FulfillService.FulfillNew(uid, KindSeat, 1, 1, SourceGrant, "test"))
	r := billingRouter(injectUser(uid))
	code, env := do(t, r, http.MethodGet, "/billing/license-units?kind=seat", nil)
	assert.Equal(t, http.StatusOK, code)
	data := env["data"].(map[string]any)
	items := data["items"].([]any)
	assert.Len(t, items, 1)
}

func TestListLicenseUnits_KeywordFilters(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_license_units") })
	uid := 970110
	require.NoError(t, FulfillService.FulfillNew(uid, KindSeat, 1, 1, SourceGrant, "test"))
	r := billingRouter(injectUser(uid))
	// 该单元未占用实例，关键词过滤后应为空。
	code, env := do(t, r, http.MethodGet, "/billing/license-units?kind=seat&keyword=nomatch", nil)
	assert.Equal(t, http.StatusOK, code)
	data := env["data"].(map[string]any)
	items := data["items"].([]any)
	assert.Len(t, items, 0)
}

func TestListLicenseUnits_Unauthorized(t *testing.T) {
	r := billingRouter(injectUser(0))
	code, _ := do(t, r, http.MethodGet, "/billing/license-units", nil)
	assert.Equal(t, http.StatusUnauthorized, code)
}

func TestCreateAndGetAndPayBizOrder_Lifecycle(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_license_units", "billing_biz_orders", "billing_biz_order_items")
	})
	uid := 970111
	require.NoError(t, WalletService.TopUp(uid, 1000000, "seed", "test"))
	r := billingRouter(injectUser(uid))

	// 创建：余额支付 seat_new，下单即履约并 paid。
	code, env := do(t, r, http.MethodPost, "/billing/orders", BizOrderCreate{
		BizType: BizSeatNew, Quantity: 1, DurationValue: 1, PayMethod: PayBalance,
	})
	assert.Equal(t, http.StatusOK, code)
	data := env["data"].(map[string]any)
	order := data["order"].(map[string]any)
	orderID := int(order["id"].(float64))
	require.NotZero(t, orderID)

	// 列表。
	code, env = do(t, r, http.MethodGet, "/billing/orders?page=1&size=10", nil)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(0), env["code"])

	// 详情。
	code, env = do(t, r, http.MethodGet, "/billing/orders/"+itoa(orderID), nil)
	assert.Equal(t, http.StatusOK, code)
	d := env["data"].(map[string]any)
	assert.Equal(t, float64(orderID), d["id"])

	// 已 paid 的订单再 pay → 幂等，返回 200 且仍为 paid。
	code, env = do(t, r, http.MethodPost, "/billing/orders/"+itoa(orderID)+"/pay", nil)
	assert.Equal(t, http.StatusOK, code)
	pd := env["data"].(map[string]any)
	pay := pd["pay"].(map[string]any)
	assert.Equal(t, BizOrderPaid, pay["status"])
}

func TestGetBizOrder_BadID(t *testing.T) {
	r := billingRouter(injectUser(970112))
	code, _ := do(t, r, http.MethodGet, "/billing/orders/abc", nil)
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestGetBizOrder_NotFound(t *testing.T) {
	r := billingRouter(injectUser(970113))
	code, _ := do(t, r, http.MethodGet, "/billing/orders/99999999", nil)
	assert.NotEqual(t, http.StatusOK, code)
}

func TestPayBizOrder_BadID(t *testing.T) {
	r := billingRouter(injectUser(970114))
	code, _ := do(t, r, http.MethodPost, "/billing/orders/xyz/pay", nil)
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestCreateBizOrder_Unauthorized(t *testing.T) {
	r := billingRouter(injectUser(0))
	code, _ := do(t, r, http.MethodPost, "/billing/orders", BizOrderCreate{BizType: BizSeatNew})
	assert.Equal(t, http.StatusUnauthorized, code)
}

func TestCreateBizOrder_BadRequest(t *testing.T) {
	r := billingRouter(injectUser(970115))
	// 非法 JSON 体（类型不符）→ 400。
	req := httptest.NewRequest(http.MethodPost, "/billing/orders", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetRuntimeLog_OK(t *testing.T) {
	uid := 970116
	r := billingRouter(injectUser(uid))
	code, env := do(t, r, http.MethodGet, "/billing/runtime/log?page=1&size=10&from=2026-01-01&to=2026-12-31", nil)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(0), env["code"])
}

func TestGetRuntimeLog_Unauthorized(t *testing.T) {
	r := billingRouter(injectUser(0))
	code, _ := do(t, r, http.MethodGet, "/billing/runtime/log", nil)
	assert.Equal(t, http.StatusUnauthorized, code)
}

func TestListBizOrders_Unauthorized(t *testing.T) {
	r := billingRouter(injectUser(0))
	code, _ := do(t, r, http.MethodGet, "/billing/orders", nil)
	assert.Equal(t, http.StatusUnauthorized, code)
}

// ---- 前台试用 handler ----

func TestListMyTrials_OK(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable(trialTables...) })
	repo := newTrialRepository(framework.DB)
	require.NoError(t, repo.createPolicy(&TrialPolicy{Code: "mytrial-1", Name: "T1", Enabled: true, PerUserLimit: 1, Items: oneItem(KindSeat, 1, 7)}))
	r := billingRouter(injectUser(970117))
	code, env := do(t, r, http.MethodGet, "/billing/trials", nil)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(0), env["code"])
}

func TestListMyTrials_Unauthorized(t *testing.T) {
	r := billingRouter(injectUser(0))
	code, _ := do(t, r, http.MethodGet, "/billing/trials", nil)
	assert.Equal(t, http.StatusUnauthorized, code)
}

func TestClaimTrial_OK(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable(trialTables...) })
	repo := newTrialRepository(framework.DB)
	require.NoError(t, repo.createPolicy(&TrialPolicy{Code: "claim-ok", Name: "C", Enabled: true, PerUserLimit: 1, AllowNewUser: true, Items: oneItem(SubjectRuntimeMinute, 600, 0)}))
	r := billingRouter(injectUser(970118))
	code, env := do(t, r, http.MethodPost, "/billing/trials/claim-ok/claim", ClaimRequest{})
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(0), env["code"])
}

func TestClaimTrial_Unauthorized(t *testing.T) {
	r := billingRouter(injectUser(0))
	code, _ := do(t, r, http.MethodPost, "/billing/trials/x/claim", ClaimRequest{})
	assert.Equal(t, http.StatusUnauthorized, code)
}

func TestClaimTrial_NotFound(t *testing.T) {
	r := billingRouter(injectUser(970119))
	code, _ := do(t, r, http.MethodPost, "/billing/trials/no-such-code/claim", ClaimRequest{})
	assert.NotEqual(t, http.StatusOK, code)
}

// ---- 后台 handler ----

func TestAdminGetAccount_BadUserID(t *testing.T) {
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodGet, "/admin/billing/accounts/abc", nil)
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestAdminGetAccount_OK(t *testing.T) {
	uid := 970120
	require.NoError(t, WalletService.TopUp(uid, 7000, "seed", "test"))
	r := billingRouter(injectUser(1))
	code, env := do(t, r, http.MethodGet, "/admin/billing/accounts/"+itoa(uid)+"?page=1&size=10", nil)
	assert.Equal(t, http.StatusOK, code)
	data := env["data"].(map[string]any)
	require.Contains(t, data, "account")
	require.Contains(t, data, "capacities_v2")
}

func TestAdminAdjustBalance_OK(t *testing.T) {
	uid := 970121
	r := billingRouter(injectUser(1))
	code, env := do(t, r, http.MethodPost, "/admin/billing/accounts/"+itoa(uid)+"/adjust",
		AdjustRequest{DeltaCents: 500, Reason: "compensation"})
	assert.Equal(t, http.StatusOK, code)
	data := env["data"].(map[string]any)
	assert.Equal(t, float64(500), data["balance_cents"])
}

func TestAdminAdjustBalance_BadRequest(t *testing.T) {
	r := billingRouter(injectUser(1))
	// 缺 reason（required）。
	code, _ := do(t, r, http.MethodPost, "/admin/billing/accounts/970122/adjust",
		map[string]any{"delta_cents": 100})
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestAdminAdjustBalance_BadUserID(t *testing.T) {
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodPost, "/admin/billing/accounts/bad/adjust",
		AdjustRequest{DeltaCents: 1, Reason: "x"})
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestAdminAdjustResourceV2_Seat(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_license_units") })
	uid := 970123
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodPost, "/admin/billing/accounts/"+itoa(uid)+"/adjust-resource",
		map[string]any{"subject": KindSeat, "quantity": 2, "duration_value": 1, "reason": "grant"})
	assert.Equal(t, http.StatusOK, code)
	cap, err := LicenseService.Capacity(uid, KindSeat)
	require.NoError(t, err)
	assert.Equal(t, 2, cap)
}

func TestAdminAdjustResourceV2_RuntimeMinute(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_runtime_minute_wallets") })
	uid := 970124
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodPost, "/admin/billing/accounts/"+itoa(uid)+"/adjust-resource",
		map[string]any{"subject": SubjectRuntimeMinute, "minutes": 300, "reason": "grant"})
	assert.Equal(t, http.StatusOK, code)
	rem, _ := RuntimeWalletService.Remaining(uid)
	assert.Equal(t, int64(300), rem)
}

func TestAdminAdjustResourceV2_BadSubject(t *testing.T) {
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodPost, "/admin/billing/accounts/970125/adjust-resource",
		map[string]any{"subject": "bogus", "reason": "x"})
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestAdminAdjustResourceV2_BadUserID(t *testing.T) {
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodPost, "/admin/billing/accounts/zzz/adjust-resource",
		map[string]any{"subject": KindSeat})
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestAdminAdjustResourceV2_MissingSubject(t *testing.T) {
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodPost, "/admin/billing/accounts/970126/adjust-resource",
		map[string]any{"quantity": 1})
	assert.Equal(t, http.StatusBadRequest, code)
}

// ---- 后台试用策略 handler ----

func TestAdminTrialPolicy_CRUD(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable(trialTables...) })
	r := billingRouter(injectUser(1))

	// 列表（空也 200）。
	code, _ := do(t, r, http.MethodGet, "/admin/billing/trials", nil)
	assert.Equal(t, http.StatusOK, code)

	// 创建。
	code, env := do(t, r, http.MethodPost, "/admin/billing/trials", TrialPolicyCreate{
		Code: "adm-pol-1", Name: "AP1", PerUserLimit: 1,
		Items: []TrialPolicyItemInput{{Subject: KindSeat, Quantity: 1, ExpireDays: 7}},
	})
	require.Equal(t, http.StatusOK, code)
	pid := int(env["data"].(map[string]any)["id"].(float64))

	// 更新。
	newName := "AP1-renamed"
	code, _ = do(t, r, http.MethodPut, "/admin/billing/trials/"+itoa(pid), TrialPolicyUpdate{Name: newName})
	assert.Equal(t, http.StatusOK, code)

	// 资格授予。
	code, _ = do(t, r, http.MethodPost, "/admin/billing/trials/"+itoa(pid)+"/eligibility",
		EligibilityGrantRequest{UserID: 970130})
	assert.Equal(t, http.StatusOK, code)

	// 营销展示标记。
	code, _ = do(t, r, http.MethodPost, "/admin/billing/trials/"+itoa(pid)+"/feature",
		TrialFeatureRequest{Featured: true})
	assert.Equal(t, http.StatusOK, code)

	// 发放记录。
	code, _ = do(t, r, http.MethodGet, "/admin/billing/trials/"+itoa(pid)+"/grants", nil)
	assert.Equal(t, http.StatusOK, code)

	// 删除。
	code, _ = do(t, r, http.MethodDelete, "/admin/billing/trials/"+itoa(pid), nil)
	assert.Equal(t, http.StatusOK, code)
}

func TestAdminCreateTrialPolicy_BadRequest(t *testing.T) {
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodPost, "/admin/billing/trials", map[string]any{"name": "x"})
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestAdminUpdateTrialPolicy_BadID(t *testing.T) {
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodPut, "/admin/billing/trials/abc", TrialPolicyUpdate{Name: "x"})
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestAdminDeleteTrialPolicy_BadID(t *testing.T) {
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodDelete, "/admin/billing/trials/xyz", nil)
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestAdminGrantTrialEligibility_BadID(t *testing.T) {
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodPost, "/admin/billing/trials/abc/eligibility",
		EligibilityGrantRequest{UserID: 1})
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestAdminGrantTrialEligibility_Unauthorized(t *testing.T) {
	r := billingRouter(injectUser(0))
	code, _ := do(t, r, http.MethodPost, "/admin/billing/trials/1/eligibility",
		EligibilityGrantRequest{UserID: 1})
	assert.Equal(t, http.StatusUnauthorized, code)
}

func TestAdminGrantTrialEligibility_BadRequest(t *testing.T) {
	r := billingRouter(injectUser(1))
	// 缺 user_id（required）。
	code, _ := do(t, r, http.MethodPost, "/admin/billing/trials/1/eligibility", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestAdminFeatureTrialPolicy_BadID(t *testing.T) {
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodPost, "/admin/billing/trials/abc/feature", TrialFeatureRequest{Featured: true})
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestAdminListTrialGrants_BadID(t *testing.T) {
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodGet, "/admin/billing/trials/abc/grants", nil)
	assert.Equal(t, http.StatusBadRequest, code)
}

// ---- 后台配置 GET/PUT handler ----

func TestAdminPricingConfig_GetPut(t *testing.T) {
	r := billingRouter(injectUser(1))

	for _, p := range []string{"/admin/billing/pricing", "/admin/billing/runtime-config",
		"/admin/billing/payment-methods", "/admin/billing/recharge-presets", "/admin/billing/notices"} {
		code, env := do(t, r, http.MethodGet, p, nil)
		assert.Equalf(t, http.StatusOK, code, "GET %s", p)
		assert.Equalf(t, float64(0), env["code"], "GET %s code", p)
	}
}

// restorePricing 在每个会改写全局定价配置的用例后还原默认值，避免污染共享单例（与其他用例的默认值断言冲突）。
func restorePricing(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { _ = PricingConfigService.Save(defaultPricingConfig()) })
}

func TestAdminSavePricing_OK(t *testing.T) {
	restorePricing(t)
	r := billingRouter(injectUser(1))
	body := map[string]any{"kinds": map[string]any{KindSeat: map[string]any{"unit_price_cents": 3500}}}
	code, env := do(t, r, http.MethodPut, "/admin/billing/pricing", body)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(0), env["code"])
}

func TestAdminSavePricing_BadRequest(t *testing.T) {
	r := billingRouter(injectUser(1))
	req := httptest.NewRequest(http.MethodPut, "/admin/billing/pricing", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminSaveRuntimePricing_OK(t *testing.T) {
	restorePricing(t)
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodPut, "/admin/billing/runtime-config", RuntimePackCfg{})
	assert.Equal(t, http.StatusOK, code)
}

func TestAdminSavePaymentMethods_OK(t *testing.T) {
	restorePricing(t)
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodPut, "/admin/billing/payment-methods",
		map[string]any{"payment_methods": []map[string]any{{"code": "wechat", "name": "微信", "enabled": true, "sort": 1}}})
	assert.Equal(t, http.StatusOK, code)
}

func TestAdminSaveRechargePresets_OK(t *testing.T) {
	restorePricing(t)
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodPut, "/admin/billing/recharge-presets",
		map[string]any{"presets_cents": []int64{1000, 5000, 10000}})
	assert.Equal(t, http.StatusOK, code)
}

func TestAdminSaveNotices_OK(t *testing.T) {
	restorePricing(t)
	r := billingRouter(injectUser(1))
	body := map[string]any{
		KindSeat:       map[string]any{"notice": "seat notice", "billing_note": "note"},
		"runtime_pack": map[string]any{"notice": "rt notice"},
	}
	code, _ := do(t, r, http.MethodPut, "/admin/billing/notices", body)
	assert.Equal(t, http.StatusOK, code)
}

func TestAdminSavePaymentMethods_BadRequest(t *testing.T) {
	r := billingRouter(injectUser(1))
	req := httptest.NewRequest(http.MethodPut, "/admin/billing/payment-methods", bytes.NewBufferString("nope"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// 合法手续费配置（wechat 2%+¥1、balance 全 0）保存通过并回显 fee 字段。
func TestAdminSavePaymentMethods_FeeValid(t *testing.T) {
	restorePricing(t)
	r := billingRouter(injectUser(1))
	code, env := do(t, r, http.MethodPut, "/admin/billing/payment-methods",
		map[string]any{"payment_methods": []map[string]any{
			{"code": "balance", "name": "余额", "enabled": true, "sort": 0, "fee_percent_bps": 0, "fee_fixed_cents": 0},
			{"code": "wechat", "name": "微信", "enabled": true, "sort": 1, "fee_percent_bps": 200, "fee_fixed_cents": 100},
		}})
	require.Equal(t, http.StatusOK, code)
	data := env["data"].(map[string]any)
	methods := data["payment_methods"].([]any)
	wechat := methods[1].(map[string]any)
	assert.Equal(t, float64(200), wechat["fee_percent_bps"])
	assert.Equal(t, float64(100), wechat["fee_fixed_cents"])
}

// balance 配置非 0 手续费 → 400。
func TestAdminSavePaymentMethods_BalanceFeeRejected(t *testing.T) {
	restorePricing(t)
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodPut, "/admin/billing/payment-methods",
		map[string]any{"payment_methods": []map[string]any{
			{"code": "balance", "name": "余额", "enabled": true, "sort": 0, "fee_percent_bps": 200, "fee_fixed_cents": 0},
		}})
	assert.Equal(t, http.StatusBadRequest, code)
}

// 负值固定手续费 → 400。
func TestAdminSavePaymentMethods_NegativeFixedRejected(t *testing.T) {
	restorePricing(t)
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodPut, "/admin/billing/payment-methods",
		map[string]any{"payment_methods": []map[string]any{
			{"code": "wechat", "name": "微信", "enabled": true, "sort": 1, "fee_percent_bps": 200, "fee_fixed_cents": -1},
		}})
	assert.Equal(t, http.StatusBadRequest, code)
}

// 比例手续费超上限(>10000) → 400。
func TestAdminSavePaymentMethods_PercentOverCapRejected(t *testing.T) {
	restorePricing(t)
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodPut, "/admin/billing/payment-methods",
		map[string]any{"payment_methods": []map[string]any{
			{"code": "wechat", "name": "微信", "enabled": true, "sort": 1, "fee_percent_bps": 10001, "fee_fixed_cents": 0},
		}})
	assert.Equal(t, http.StatusBadRequest, code)
}

// 满额免阈值合法（wechat 满 ¥1000 免 + logo_url）保存通过并回显新字段。
func TestAdminSavePaymentMethods_ThresholdAndLogoValid(t *testing.T) {
	restorePricing(t)
	r := billingRouter(injectUser(1))
	code, env := do(t, r, http.MethodPut, "/admin/billing/payment-methods",
		map[string]any{"payment_methods": []map[string]any{
			{"code": "wechat", "name": "微信", "enabled": true, "sort": 1, "fee_percent_bps": 200, "fee_fixed_cents": 100, "fee_free_threshold_cents": 100000, "logo_url": "https://x.com/w.svg"},
		}})
	require.Equal(t, http.StatusOK, code)
	data := env["data"].(map[string]any)
	methods := data["payment_methods"].([]any)
	wechat := methods[0].(map[string]any)
	assert.Equal(t, float64(100000), wechat["fee_free_threshold_cents"])
	assert.Equal(t, "https://x.com/w.svg", wechat["logo_url"])
}

// 满额免阈值为负 → 400。
func TestAdminSavePaymentMethods_NegativeThresholdRejected(t *testing.T) {
	restorePricing(t)
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodPut, "/admin/billing/payment-methods",
		map[string]any{"payment_methods": []map[string]any{
			{"code": "wechat", "name": "微信", "enabled": true, "sort": 1, "fee_free_threshold_cents": -1},
		}})
	assert.Equal(t, http.StatusBadRequest, code)
}

// balance 配置非 0 满额免阈值 → 400。
func TestAdminSavePaymentMethods_BalanceThresholdRejected(t *testing.T) {
	restorePricing(t)
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodPut, "/admin/billing/payment-methods",
		map[string]any{"payment_methods": []map[string]any{
			{"code": "balance", "name": "余额", "enabled": true, "sort": 0, "fee_free_threshold_cents": 100000},
		}})
	assert.Equal(t, http.StatusBadRequest, code)
}

// ---- UploadPayMethodLogo（镜像 partner 上传校验；S3 为全局变量，按既有方式跳过深覆盖）----

// payLogoMultipart 构造一个 multipart 请求体（field=file）。
func payLogoMultipart(t *testing.T, filename string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, _ = fw.Write(data)
	require.NoError(t, mw.Close())
	return &buf, mw.FormDataContentType()
}

// S3 未配置 → 503。
func TestUploadPayMethodLogo_NoS3(t *testing.T) {
	if framework.S3 != nil {
		t.Skip("S3 已配置，跳过未配置分支")
	}
	r := billingRouter(injectUser(1))
	req := httptest.NewRequest(http.MethodPost, "/admin/billing/payment-methods/upload", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// S3 已配置但缺文件 → 400。
func TestUploadPayMethodLogo_MissingFile(t *testing.T) {
	if framework.S3 == nil {
		t.Skip("S3 未配置，缺文件分支被 503 短路")
	}
	r := billingRouter(injectUser(1))
	req := httptest.NewRequest(http.MethodPost, "/admin/billing/payment-methods/upload", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// 非法扩展名（.exe）→ 400。
func TestUploadPayMethodLogo_BadExt(t *testing.T) {
	if framework.S3 == nil {
		t.Skip("S3 未配置，扩展名校验分支被 503 短路")
	}
	r := billingRouter(injectUser(1))
	buf, ct := payLogoMultipart(t, "evil.exe", []byte("x"))
	req := httptest.NewRequest(http.MethodPost, "/admin/billing/payment-methods/upload", buf)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---- 后台订单 handler ----

func TestAdminBizOrders_ListAndDetailAndMarkPaid(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_biz_orders", "billing_biz_order_items", "billing_license_units")
	})
	uid := 970140
	// 用第三方下单生成一笔订单（自动 paid）。
	res, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: BizSeatNew, Quantity: 1, DurationValue: 1, PayMethod: PayWechat,
	})
	require.NoError(t, err)
	oid := int(res.Order.ID)

	r := billingRouter(injectUser(1))

	// 列表（带 userId 过滤）。
	code, env := do(t, r, http.MethodGet, "/admin/billing/biz-orders?userId="+itoa(uid)+"&page=1&size=10", nil)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(0), env["code"])
	// 列表每行须含 phone 字段（无对应前台用户时为空串）。
	page := env["data"].(map[string]any)
	list := page["list"].([]any)
	require.GreaterOrEqual(t, len(list), 1)
	row := list[0].(map[string]any)
	_, hasPhone := row["phone"]
	assert.True(t, hasPhone, "订单列表行应含 phone 字段")

	// 详情。
	code, env = do(t, r, http.MethodGet, "/admin/billing/biz-orders/"+itoa(oid), nil)
	assert.Equal(t, http.StatusOK, code)
	d := env["data"].(map[string]any)
	assert.Equal(t, float64(uid), d["user_id"])

	// mark-paid（已 paid，幂等/报错都接受非 panic；只验证 handler 不 500-空）。
	code, _ = do(t, r, http.MethodPost, "/admin/billing/biz-orders/"+itoa(oid)+"/mark-paid", nil)
	assert.Contains(t, []int{http.StatusOK, http.StatusConflict, http.StatusUnprocessableEntity, http.StatusBadRequest}, code)
}

// 按手机号精确过滤：命中（回填正确 phone）与未命中（空页）。
func TestAdminBizOrders_FilterByPhone(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_biz_orders", "billing_biz_order_items", "billing_license_units")
		framework.CleanTable("users")
	})

	// 造一个前台用户（直接写 users 表，避开 user internal）：id 与手机号确定。
	const phone = "13955550001"
	uid := 970160
	require.NoError(t, framework.DB.Exec(
		"INSERT INTO users (id, phone, hashed_password, is_active, token_version) VALUES (?, ?, ?, ?, ?)",
		uid, phone, "x", true, 0).Error)

	// 为该用户下一笔订单。
	_, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: BizSeatNew, Quantity: 1, DurationValue: 1, PayMethod: PayWechat,
	})
	require.NoError(t, err)

	r := billingRouter(injectUser(1))

	// 命中：?phone= 精确手机号 → 列表非空且行 phone 正确回填。
	code, env := do(t, r, http.MethodGet, "/admin/billing/biz-orders?phone="+phone+"&page=1&size=10", nil)
	require.Equal(t, http.StatusOK, code)
	page := env["data"].(map[string]any)
	assert.Equal(t, float64(1), page["total"])
	list := page["list"].([]any)
	require.Len(t, list, 1)
	row := list[0].(map[string]any)
	assert.Equal(t, float64(uid), row["user_id"])
	assert.Equal(t, phone, row["phone"])

	// 未命中：手机号不存在 → 空页。
	code, env = do(t, r, http.MethodGet, "/admin/billing/biz-orders?phone=13900000000&page=1&size=10", nil)
	require.Equal(t, http.StatusOK, code)
	page = env["data"].(map[string]any)
	assert.Equal(t, float64(0), page["total"])
}

func TestAdminMarkBizOrderPaid_BadID(t *testing.T) {
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodPost, "/admin/billing/biz-orders/abc/mark-paid", nil)
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestAdminGetBizOrder_BadID(t *testing.T) {
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodGet, "/admin/billing/biz-orders/xyz", nil)
	assert.Equal(t, http.StatusBadRequest, code)
}

func TestAdminGetBizOrder_NotFound(t *testing.T) {
	r := billingRouter(injectUser(1))
	code, _ := do(t, r, http.MethodGet, "/admin/billing/biz-orders/88888888", nil)
	assert.NotEqual(t, http.StatusOK, code)
}

// 用户详情 GetBizOrder 与后台详情 AdminGetBizOrder 均返回 fee 三键且数值正确。
func TestBizOrderDetail_ReturnsFeeKeys(t *testing.T) {
	t.Cleanup(func() {
		_ = PricingConfigService.Save(defaultPricingConfig())
		framework.CleanTable("billing_biz_orders", "billing_biz_order_items", "billing_license_units")
	})
	_, err := PricingConfigService.SavePartial(func(d *PricingConfigData) {
		d.PaymentMethods = []PaymentMethod{
			{Code: PayBalance, Name: "余额", Enabled: true, Sort: 0},
			{Code: PayWechat, Name: "微信", Enabled: true, Sort: 1, FeePercentBps: 200, FeeFixedCents: 100},
		}
	})
	require.NoError(t, err)

	uid := 970150
	res, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: BizSeatNew, Quantity: 1, DurationValue: 1, PayMethod: PayWechat,
	})
	require.NoError(t, err)
	oid := int(res.Order.ID)
	// total = 3000；fee = 60 + 100 = 160。
	require.Equal(t, int64(160), res.Order.FeeCents)

	// 用户侧详情。
	ru := billingRouter(injectUser(uid))
	code, env := do(t, ru, http.MethodGet, "/billing/orders/"+itoa(oid), nil)
	require.Equal(t, http.StatusOK, code)
	d := env["data"].(map[string]any)
	assert.Equal(t, float64(160), d["fee_cents"])
	assert.Equal(t, float64(200), d["fee_percent_bps"])
	assert.Equal(t, float64(100), d["fee_fixed_cents"])

	// 后台侧详情。
	ra := billingRouter(injectUser(1))
	code, env = do(t, ra, http.MethodGet, "/admin/billing/biz-orders/"+itoa(oid), nil)
	require.Equal(t, http.StatusOK, code)
	ad := env["data"].(map[string]any)
	assert.Equal(t, float64(160), ad["fee_cents"])
	assert.Equal(t, float64(200), ad["fee_percent_bps"])
	assert.Equal(t, float64(100), ad["fee_fixed_cents"])
}
