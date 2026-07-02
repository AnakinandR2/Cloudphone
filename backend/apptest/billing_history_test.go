package apptest

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"manager-backend/modules/billing"
)

// billing_history_test.go 覆盖计划 B 任务 4：订单/账本/单元列表与属主隔离（TC-10-080~086）。
//
// 承接关系：modules/billing/internal/api_handler_test.go 与 service_test.go（Plan A）已在
// handler/service 单元层覆盖：ListBizOrders 未授权 401、GetBizOrder 无效 ID 400/不存在（伪造大 ID）非 200、
// AdminListBizOrders 按 userId/phone 过滤(命中回填 phone/未命中空页)、AdminGetBizOrder 返回
// user_id/items/fee 三键、ListLedger 分页+类型过滤+跨 user 隔离（但用手写 int userID，不经真实 JWT）。
// 这些单元测试均用 injectUser(uid) 直接注入 gin.Context，绕开了真实鉴权/RBAC 中间件。
// 本文件只补 HTTP 边界的差集：
//   - 真实前台 JWT 下 A/B 两用户互相看不到对方的订单/账本（真实 401/404 路径，而非手写 int uid）；
//   - GetBizOrder 对「真实存在但属于他人」的订单 → 404「订单不存在」（Plan A 只测了不存在的伪造 ID）；
//   - ListBizOrders 的 status/biz_type/from/to 组合过滤 + 分页在真实 HTTP 层生效；
//   - GetMyLedger 按 subject/type 过滤、流水完整（含 delta/balance_after）；
//   - AdminListBizOrders 的 phone 查不到 → 空页（真实 HTTP，不同于 Plan A 直接查询未经 staff 权限校验的路径）；
//   - AdminGetBizOrder 返回完整字段（真实 HTTP + 真实 staff 权限中间件放行）；
//   - TC-10-086：真实 RBAC 门禁 —— 持 billing:view 的员工可读、无该权限的员工 403「权限不足」
//     （Plan A 的 billingRouter 完全不挂 PermissionMiddleware，此点是纯粹差集）。

// histCreateSeatOrder 经 HTTP 用余额支付创建一个 seat_new 订单，返回订单 ID 与响应体 Data。
// 调用前调用方需已给该用户充值足够余额（seed 单价 3000 分/台/月）。
func histCreateSeatOrder(t *testing.T, engine *gin.Engine, token string, quantity, durationValue int) (int, map[string]interface{}) {
	t.Helper()
	w := doJSON(engine, "POST", "/api/v1/billing/orders", token, map[string]interface{}{
		"biz_type":       "seat_new",
		"quantity":       quantity,
		"duration_value": durationValue,
		"pay_method":     "balance",
	})
	require.Equal(t, http.StatusOK, w.Code, "下单失败: %s", w.Body.String())
	data := decode(t, w).Data.(map[string]interface{})
	order := data["order"].(map[string]interface{})
	id := int(order["id"].(float64))
	require.NotZero(t, id, "下单应返回订单 ID")
	return id, order
}

// TestBillingOrderHistoryOwnerIsolationHTTP 覆盖 TC-10-080/081：
//   - 属主隔离：A 只能看到自己的订单，看不到 B 的（列表 total/内容）；
//   - status/biz_type/时间区间过滤：按 status=paid、biz_type=seat_new 过滤命中，按不存在的
//     status 过滤应为空；
//   - 分页：默认 size=20；显式 size 生效；
//   - 详情：本人订单 200 且含 items 与 fee_cents/fee_percent_bps/fee_fixed_cents；
//   - TC-10-081 非本人 id → 404「订单不存在」（真实存在但属于另一用户的订单，而非伪造 ID）。
func TestBillingOrderHistoryOwnerIsolationHTTP(t *testing.T) {
	r := setupRouter()

	const phoneA = "13904000001"
	const phoneB = "13904000002"
	tokenA := registerUser(t, r, phoneA)
	tokenB := registerUser(t, r, phoneB)
	uidA := userIDByPhone(t, phoneA)
	uidB := userIDByPhone(t, phoneB)
	t.Cleanup(func() {
		cleanupUserAndBillingByID(t, uidA)
		cleanupUserAndBillingByID(t, uidB)
	})

	// 充值，确保余额购买能成功（seed 单价 3000 分/台/月）。
	require.Equal(t, http.StatusOK,
		doJSON(r, "POST", "/api/v1/billing/topup", tokenA, map[string]interface{}{"amount_cents": 1000000}).Code)
	require.Equal(t, http.StatusOK,
		doJSON(r, "POST", "/api/v1/billing/topup", tokenB, map[string]interface{}{"amount_cents": 1000000}).Code)

	// A 下两笔订单（seat_new × 2），B 下一笔订单。
	orderA1, _ := histCreateSeatOrder(t, r, tokenA, 1, 1)
	orderA2, _ := histCreateSeatOrder(t, r, tokenA, 2, 1)
	orderB1, _ := histCreateSeatOrder(t, r, tokenB, 1, 1)

	// 1) 属主隔离：A 的列表只应看到 A 的 2 笔，看不到 B 的订单 ID。
	wListA := doJSON(r, "GET", "/api/v1/billing/orders", tokenA, nil)
	require.Equal(t, http.StatusOK, wListA.Code, "A 查订单列表失败: %s", wListA.Body.String())
	pageA := decode(t, wListA).Data.(map[string]interface{})
	assert.Equal(t, float64(2), pageA["total"], "A 应只看到自己的 2 笔订单")
	listA := pageA["list"].([]interface{})
	require.Len(t, listA, 2)
	seenA := map[int]bool{}
	for _, it := range listA {
		row := it.(map[string]interface{})
		seenA[int(row["id"].(float64))] = true
		// 随单返回订单项明细。
		items, ok := row["items"].([]interface{})
		require.True(t, ok, "订单历史应随单返回 items: %v", row)
		assert.NotEmpty(t, items, "seat_new 订单应至少 1 个订单项")
	}
	assert.True(t, seenA[orderA1] && seenA[orderA2], "A 列表应含自己的两笔订单")
	assert.False(t, seenA[orderB1], "A 列表不应含 B 的订单")

	// B 的列表只应看到自己的 1 笔。
	wListB := doJSON(r, "GET", "/api/v1/billing/orders", tokenB, nil)
	require.Equal(t, http.StatusOK, wListB.Code)
	pageB := decode(t, wListB).Data.(map[string]interface{})
	assert.Equal(t, float64(1), pageB["total"], "B 应只看到自己的 1 笔订单")

	// 2) status 过滤：全部下单均为余额支付即时 paid，status=paid 命中 2 条；
	//    status=unpaid（A 没有未支付订单）→ 0 条。
	wPaid := doJSON(r, "GET", "/api/v1/billing/orders?status=paid", tokenA, nil)
	require.Equal(t, http.StatusOK, wPaid.Code)
	pagePaid := decode(t, wPaid).Data.(map[string]interface{})
	assert.Equal(t, float64(2), pagePaid["total"], "A 的 2 笔订单均应为 paid")

	wUnpaid := doJSON(r, "GET", "/api/v1/billing/orders?status=unpaid", tokenA, nil)
	require.Equal(t, http.StatusOK, wUnpaid.Code)
	pageUnpaid := decode(t, wUnpaid).Data.(map[string]interface{})
	assert.Equal(t, float64(0), pageUnpaid["total"], "A 无 unpaid 订单，按 status 过滤应为空")

	// 3) biz_type 过滤：biz_type=seat_new 命中全部 2 条；biz_type=recharge（A 未下过）应为空。
	wSeat := doJSON(r, "GET", "/api/v1/billing/orders?biz_type=seat_new", tokenA, nil)
	require.Equal(t, http.StatusOK, wSeat.Code)
	pageSeat := decode(t, wSeat).Data.(map[string]interface{})
	assert.Equal(t, float64(2), pageSeat["total"], "按 biz_type=seat_new 过滤应命中 2 条")

	wRecharge := doJSON(r, "GET", "/api/v1/billing/orders?biz_type=recharge", tokenA, nil)
	require.Equal(t, http.StatusOK, wRecharge.Code)
	pageRecharge := decode(t, wRecharge).Data.(map[string]interface{})
	assert.Equal(t, float64(0), pageRecharge["total"], "A 未下过 recharge 订单，应为空")

	// 4) 时间区间过滤：from=未来 应过滤掉全部（因为订单创建时间早于 from）。
	// RFC3339 含时区偏移 '+08:00'，裸拼进 URL 会被 query 解析当成空格 → 过滤失效。必须转义。
	future := url.QueryEscape(time.Now().Add(24 * time.Hour).Format(time.RFC3339))
	wFrom := doJSON(r, "GET", "/api/v1/billing/orders?from="+future, tokenA, nil)
	require.Equal(t, http.StatusOK, wFrom.Code)
	pageFrom := decode(t, wFrom).Data.(map[string]interface{})
	assert.Equal(t, float64(0), pageFrom["total"], "from 设为未来应过滤掉全部历史订单")

	// to=未来 应仍命中全部（订单created_at必早于未来）。
	wTo := doJSON(r, "GET", "/api/v1/billing/orders?to="+future, tokenA, nil)
	require.Equal(t, http.StatusOK, wTo.Code)
	pageTo := decode(t, wTo).Data.(map[string]interface{})
	assert.Equal(t, float64(2), pageTo["total"], "to 设为未来应仍命中全部历史订单")

	// 5) 分页：size=1 应只返回 1 条，但 total 仍是全量 2。
	wSize1 := doJSON(r, "GET", "/api/v1/billing/orders?page=1&size=1", tokenA, nil)
	require.Equal(t, http.StatusOK, wSize1.Code)
	pageSize1 := decode(t, wSize1).Data.(map[string]interface{})
	assert.Equal(t, float64(2), pageSize1["total"])
	assert.Len(t, pageSize1["list"].([]interface{}), 1, "size=1 应只返回 1 条")

	// 6) 详情：A 查自己的订单 → 200，含 items 与 fee 三键。
	wDetail := doJSON(r, "GET", fmt.Sprintf("/api/v1/billing/orders/%d", orderA1), tokenA, nil)
	require.Equal(t, http.StatusOK, wDetail.Code, "A 查自己订单详情失败: %s", wDetail.Body.String())
	detail := decode(t, wDetail).Data.(map[string]interface{})
	assert.Equal(t, float64(orderA1), detail["id"])
	assert.Equal(t, "seat_new", detail["biz_type"])
	assert.Equal(t, "paid", detail["status"])
	for _, key := range []string{"fee_cents", "fee_percent_bps", "fee_fixed_cents", "total_cents", "items"} {
		_, ok := detail[key]
		assert.True(t, ok, "订单详情应含字段 %s: %v", key, detail)
	}

	// 7) TC-10-081 核心：非本人 id → 404「订单不存在」。B 用真实存在（属于 A）的订单 ID 查详情。
	wCross := doJSON(r, "GET", fmt.Sprintf("/api/v1/billing/orders/%d", orderA1), tokenB, nil)
	assert.Equal(t, http.StatusNotFound, wCross.Code, "B 查 A 的订单详情应 404: %s", wCross.Body.String())
	assert.Contains(t, decode(t, wCross).Message, "订单不存在", "越权查订单详情应提示订单不存在")

	// 反向：A 查 B 的订单同样 404。
	wCross2 := doJSON(r, "GET", fmt.Sprintf("/api/v1/billing/orders/%d", orderB1), tokenA, nil)
	assert.Equal(t, http.StatusNotFound, wCross2.Code, "A 查 B 的订单详情应 404: %s", wCross2.Body.String())
	assert.Contains(t, decode(t, wCross2).Message, "订单不存在")
}

// TestBillingLedgerFilterAndIntegrityHTTP 覆盖 TC-10-082：资源/余额账本按 subject/type 过滤、
// 流水完整（含 delta/balance_after）；并顺带验证 A/B 属主隔离（复用 ListLedger 已有隔离语义，
// 但走真实前台 JWT，而非 Plan A 单元测试手写 int userID）。
func TestBillingLedgerFilterAndIntegrityHTTP(t *testing.T) {
	r := setupRouter()

	const phoneA = "13904000003"
	const phoneB = "13904000004"
	tokenA := registerUser(t, r, phoneA)
	tokenB := registerUser(t, r, phoneB)
	uidA := userIDByPhone(t, phoneA)
	uidB := userIDByPhone(t, phoneB)
	t.Cleanup(func() {
		cleanupUserAndBillingByID(t, uidA)
		cleanupUserAndBillingByID(t, uidB)
	})

	// A：充值(topup/balance) + 授予 3 个席位(adjust_grant/seat，走种子门面 FulfillNew)
	// + 授予 500 分钟临时时长(adjust_grant/runtime_minute)。
	require.Equal(t, http.StatusOK,
		doJSON(r, "POST", "/api/v1/billing/topup", tokenA, map[string]interface{}{"amount_cents": 5000}).Code)
	require.NoError(t, billing.GrantSeatLicensesForTest(int(uidA), 3))
	require.NoError(t, billing.GrantRuntimeMinutesWalletForTest(int(uidA), 500))

	// B：仅充值，用于验证属主隔离（B 的流水不应出现在 A 的账本里）。
	require.Equal(t, http.StatusOK,
		doJSON(r, "POST", "/api/v1/billing/topup", tokenB, map[string]interface{}{"amount_cents": 9999}).Code)

	// 1) 不带过滤：A 应看到 3 条流水（topup + seat adjust_grant + runtime_minute adjust_grant）。
	wAll := doJSON(r, "GET", "/api/v1/billing/ledger", tokenA, nil)
	require.Equal(t, http.StatusOK, wAll.Code, "A 查账本失败: %s", wAll.Body.String())
	pageAll := decode(t, wAll).Data.(map[string]interface{})
	assert.Equal(t, float64(3), pageAll["total"], "A 应有 3 条流水")
	listAll := pageAll["list"].([]interface{})
	require.Len(t, listAll, 3)
	for _, it := range listAll {
		row := it.(map[string]interface{})
		for _, key := range []string{"delta", "balance_after", "subject", "type"} {
			_, ok := row[key]
			assert.True(t, ok, "流水行应含字段 %s: %v", key, row)
		}
	}

	// 2) 按 subject=balance 过滤：仅命中 topup 那 1 条。
	wBalance := doJSON(r, "GET", "/api/v1/billing/ledger?subject=balance", tokenA, nil)
	require.Equal(t, http.StatusOK, wBalance.Code)
	pageBalance := decode(t, wBalance).Data.(map[string]interface{})
	assert.Equal(t, float64(1), pageBalance["total"], "subject=balance 应只命中充值流水")
	balRow := pageBalance["list"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, "balance", balRow["subject"])
	assert.Equal(t, "topup", balRow["type"])
	assert.Equal(t, float64(5000), balRow["delta"], "充值流水 delta 应为 5000 分")
	assert.Equal(t, float64(5000), balRow["balance_after"], "首次充值后余额快照应为 5000 分")

	// 3) 按 subject=seat 过滤：仅命中席位授予那 1 条。
	wSeat := doJSON(r, "GET", "/api/v1/billing/ledger?subject=seat", tokenA, nil)
	require.Equal(t, http.StatusOK, wSeat.Code)
	pageSeat := decode(t, wSeat).Data.(map[string]interface{})
	assert.Equal(t, float64(1), pageSeat["total"], "subject=seat 应只命中席位授予流水")
	seatRow := pageSeat["list"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, "seat", seatRow["subject"])
	assert.Equal(t, "adjust_grant", seatRow["type"])
	assert.Equal(t, float64(3), seatRow["delta"], "授予 3 个席位 delta 应为 3")

	// 4) 按 type=adjust_grant 过滤：命中 seat + runtime_minute 两条授予流水。
	wGrant := doJSON(r, "GET", "/api/v1/billing/ledger?type=adjust_grant", tokenA, nil)
	require.Equal(t, http.StatusOK, wGrant.Code)
	pageGrant := decode(t, wGrant).Data.(map[string]interface{})
	assert.Equal(t, float64(2), pageGrant["total"], "type=adjust_grant 应命中 2 条授予流水")

	// 5) subject + type 组合过滤：runtime_minute + adjust_grant → 仅 1 条。
	wCombo := doJSON(r, "GET", "/api/v1/billing/ledger?subject=runtime_minute&type=adjust_grant", tokenA, nil)
	require.Equal(t, http.StatusOK, wCombo.Code)
	pageCombo := decode(t, wCombo).Data.(map[string]interface{})
	assert.Equal(t, float64(1), pageCombo["total"])
	comboRow := pageCombo["list"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, float64(500), comboRow["delta"], "临时时长授予 delta 应为 500 分钟")

	// 6) 属主隔离：B 的账本只应看到自己的 1 条充值流水，看不到 A 的任何流水。
	wB := doJSON(r, "GET", "/api/v1/billing/ledger", tokenB, nil)
	require.Equal(t, http.StatusOK, wB.Code)
	pageB := decode(t, wB).Data.(map[string]interface{})
	assert.Equal(t, float64(1), pageB["total"], "B 应只看到自己的 1 条充值流水")
	bRow := pageB["list"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, float64(9999), bRow["delta"])
}

// TestAdminBizOrdersListDetailAndPermissionGateHTTP 覆盖 TC-10-084/085/086：
//   - 后台订单列表按 userId/phone/status 过滤，phone 查不到 → 空页（真实 HTTP，staff 权限中间件真放行）；
//   - 后台订单详情返回 user_id/items/手续费快照；
//   - 权限门禁：无 billing:view/manage 的员工打后台 billing 接口 → 403「权限不足」；
//     持 billing:view 的员工可读列表/详情（只读权限即放行 GET）。
func TestAdminBizOrdersListDetailAndPermissionGateHTTP(t *testing.T) {
	r := setupRouter()
	admin := adminToken(t, r)

	const phoneA = "13904000005"
	tokenA := registerUser(t, r, phoneA)
	uidA := userIDByPhone(t, phoneA)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidA) })

	require.Equal(t, http.StatusOK,
		doJSON(r, "POST", "/api/v1/billing/topup", tokenA, map[string]interface{}{"amount_cents": 1000000}).Code)
	orderA, _ := histCreateSeatOrder(t, r, tokenA, 1, 1)

	// 造两个 staff：一个仅有 billing:view（只读通过）、一个无 billing 相关权限（403）。
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	viewerName := "hist_viewer_" + suffix
	noneName := "hist_none_" + suffix
	viewerRole := "hist_role_view_" + suffix
	noneRole := "hist_role_none_" + suffix

	var viewerRoleID, noneRoleID, viewerID, noneID int
	t.Cleanup(func() {
		db := framework.DB
		if viewerID != 0 {
			db.Exec("DELETE FROM staff_roles WHERE staff_id = ?", viewerID)
			db.Exec("DELETE FROM staff WHERE id = ?", viewerID)
		}
		if noneID != 0 {
			db.Exec("DELETE FROM staff_roles WHERE staff_id = ?", noneID)
			db.Exec("DELETE FROM staff WHERE id = ?", noneID)
		}
		if viewerRoleID != 0 {
			db.Exec("DELETE FROM role_permissions WHERE role_id = ?", viewerRoleID)
			db.Exec("DELETE FROM staff_roles WHERE role_id = ?", viewerRoleID)
			db.Exec("DELETE FROM roles WHERE id = ?", viewerRoleID)
		}
		if noneRoleID != 0 {
			db.Exec("DELETE FROM role_permissions WHERE role_id = ?", noneRoleID)
			db.Exec("DELETE FROM staff_roles WHERE role_id = ?", noneRoleID)
			db.Exec("DELETE FROM roles WHERE id = ?", noneRoleID)
		}
	})

	viewerRoleID = createRole(t, r, admin, viewerRole, []string{"billing:view"})
	noneRoleID = createRole(t, r, admin, noneRole, []string{"role:view"}) // 无 billing:view/manage

	createUser(t, r, admin, viewerName, "pass123", false)
	createUser(t, r, admin, noneName, "pass123", false)
	viewerID = staffIDByUsername(t, viewerName)
	noneID = staffIDByUsername(t, noneName)
	assignRoles(t, r, admin, viewerID, []int{viewerRoleID})
	assignRoles(t, r, admin, noneID, []int{noneRoleID})

	viewerTok := login(t, r, viewerName, "pass123")
	noneTok := login(t, r, noneName, "pass123")

	// --- TC-10-086：权限门禁 ---
	// 无 billing:view/manage 的员工打列表/详情 → 403「权限不足」。
	wDenyList := doJSON(r, "GET", "/api/v1/admin/billing/biz-orders", noneTok, nil)
	assert.Equal(t, http.StatusForbidden, wDenyList.Code, "无权限打后台订单列表应 403: %s", wDenyList.Body.String())
	assert.Contains(t, decode(t, wDenyList).Message, "权限不足")

	wDenyDetail := doJSON(r, "GET", fmt.Sprintf("/api/v1/admin/billing/biz-orders/%d", orderA), noneTok, nil)
	assert.Equal(t, http.StatusForbidden, wDenyDetail.Code, "无权限打后台订单详情应 403: %s", wDenyDetail.Body.String())
	assert.Contains(t, decode(t, wDenyDetail).Message, "权限不足")

	// --- TC-10-084：持 billing:view 的员工可读列表，按 userId 过滤命中。 ---
	wList := doJSON(r, "GET", fmt.Sprintf("/api/v1/admin/billing/biz-orders?userId=%d", uidA), viewerTok, nil)
	require.Equal(t, http.StatusOK, wList.Code, "持 billing:view 应可读订单列表: %s", wList.Body.String())
	page := decode(t, wList).Data.(map[string]interface{})
	assert.Equal(t, float64(1), page["total"], "按 userId 过滤应恰命中 A 的 1 笔订单")
	row := page["list"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, float64(uidA), row["user_id"])
	assert.Equal(t, phoneA, row["phone"], "订单行应回填下单用户手机号")

	// 按 status 过滤：status=paid 命中，status=unpaid 不命中（该订单余额支付即时 paid）。
	wPaid := doJSON(r, "GET", fmt.Sprintf("/api/v1/admin/billing/biz-orders?userId=%d&status=paid", uidA), viewerTok, nil)
	require.Equal(t, http.StatusOK, wPaid.Code)
	pagePaid := decode(t, wPaid).Data.(map[string]interface{})
	assert.Equal(t, float64(1), pagePaid["total"], "status=paid 应命中该订单")

	wUnpaid := doJSON(r, "GET", fmt.Sprintf("/api/v1/admin/billing/biz-orders?userId=%d&status=unpaid", uidA), viewerTok, nil)
	require.Equal(t, http.StatusOK, wUnpaid.Code)
	pageUnpaid := decode(t, wUnpaid).Data.(map[string]interface{})
	assert.Equal(t, float64(0), pageUnpaid["total"], "status=unpaid 不应命中该已付订单")

	// phone 精确过滤：命中。
	wPhoneHit := doJSON(r, "GET", "/api/v1/admin/billing/biz-orders?phone="+phoneA, viewerTok, nil)
	require.Equal(t, http.StatusOK, wPhoneHit.Code)
	pagePhoneHit := decode(t, wPhoneHit).Data.(map[string]interface{})
	assert.Equal(t, float64(1), pagePhoneHit["total"], "phone 精确过滤应命中")

	// phone 查不到 → 空页（TC-10-084 明确要求的场景）。
	wPhoneMiss := doJSON(r, "GET", "/api/v1/admin/billing/biz-orders?phone=13900000099", viewerTok, nil)
	require.Equal(t, http.StatusOK, wPhoneMiss.Code)
	pagePhoneMiss := decode(t, wPhoneMiss).Data.(map[string]interface{})
	assert.Equal(t, float64(0), pagePhoneMiss["total"], "phone 查不到应返回空页而非报错")
	assert.Len(t, pagePhoneMiss["list"].([]interface{}), 0)

	// --- TC-10-085：后台订单详情含 user_id/items/手续费快照。 ---
	wDetail := doJSON(r, "GET", fmt.Sprintf("/api/v1/admin/billing/biz-orders/%d", orderA), viewerTok, nil)
	require.Equal(t, http.StatusOK, wDetail.Code, "持 billing:view 应可读订单详情: %s", wDetail.Body.String())
	detail := decode(t, wDetail).Data.(map[string]interface{})
	assert.Equal(t, float64(orderA), detail["id"])
	assert.Equal(t, float64(uidA), detail["user_id"], "后台详情应返回 user_id")
	items, ok := detail["items"].([]interface{})
	require.True(t, ok, "后台详情应返回 items: %v", detail)
	assert.NotEmpty(t, items, "seat_new 订单详情应至少 1 个订单项")
	for _, key := range []string{"fee_cents", "fee_percent_bps", "fee_fixed_cents"} {
		_, ok := detail[key]
		assert.True(t, ok, "后台详情应含手续费快照字段 %s: %v", key, detail)
	}
}
