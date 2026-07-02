package apptest

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// billing_order_validation_test.go 覆盖 Plan B 任务 2：下单校验矩阵
// TC-10-007/011/012/013/014/015/016/018/032（docs/测试/用例/TC-10-计费与时长到期.md 一、三节）。
// 端点 POST /api/v1/billing/orders（CreateBizOrder）、POST /api/v1/billing/orders/:id/pay（PayBizOrder）。
//
// 承接关系：Plan A 已在 modules/billing/internal 用直调 service/handler 覆盖了这些校验分支的
// *存在性*（如 bizorder_service_test.go:TestBizOrder_RechargeRejectsBalancePay、
// api_handler_test.go:TestCreateBizOrder_BadRequest 等），但只断言 err!=nil / code!=200，
// 未在 HTTP 层校验具体状态码与文案。本文件在 apptest 补齐"具体状态码 + 精确错误文案"的接口层断言，
// 不重复 Plan A 已做的部分（如支付幂等 B1/B2、履约事务 B3 见 billing_pay_test.go）。
//
// 手机号：本文件全部用 13902 开头的唯一 11 位手机号，每个用例一个，不跨用例复用。

// TestBizOrder_SeatRenew_RequiresUnitIDs 覆盖 TC-10-007：续费需选单元。
// user 持有 seat 单元，POST /orders {biz_type:seat_renew, unit_ids:[], duration_value:1} → 422「请选择要续费的授权单元」。
func TestBizOrder_SeatRenew_RequiresUnitIDs(t *testing.T) {
	r := setupRouter()
	const phone = "13902000001"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	w := doJSON(r, "POST", "/api/v1/billing/orders", token, map[string]interface{}{
		"biz_type":       "seat_renew",
		"unit_ids":       []uint{},
		"duration_value": 1,
		"pay_method":     "balance",
	})
	require.Equal(t, http.StatusUnprocessableEntity, w.Code, "响应体: %s", w.Body.String())
	assert.Contains(t, decode(t, w).Message, "请选择要续费的授权单元")
}

// TestBizOrder_Recharge_RejectsBalancePay 覆盖 TC-10-011：充值禁用余额支付。
// Plan A 的 bizorder_service_test.go:TestBizOrder_RechargeRejectsBalancePay 只断言 err!=nil（直调 service），
// 本用例在 HTTP 层补齐具体状态码 422 + 精确文案「充值不支持余额支付」。
func TestBizOrder_Recharge_RejectsBalancePay(t *testing.T) {
	r := setupRouter()
	const phone = "13902000002"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	w := doJSON(r, "POST", "/api/v1/billing/orders", token, map[string]interface{}{
		"biz_type":     "recharge",
		"amount_cents": 10000,
		"pay_method":   "balance",
	})
	require.Equal(t, http.StatusUnprocessableEntity, w.Code, "响应体: %s", w.Body.String())
	assert.Contains(t, decode(t, w).Message, "充值不支持余额支付")
}

// TestBizOrder_Recharge_AmountMustBePositive 覆盖 TC-10-012：充值金额校验，amount_cents<=0 → 422。
func TestBizOrder_Recharge_AmountMustBePositive(t *testing.T) {
	r := setupRouter()
	const phone = "13902000003"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	for _, amt := range []int64{0, -1} {
		w := doJSON(r, "POST", "/api/v1/billing/orders", token, map[string]interface{}{
			"biz_type":     "recharge",
			"amount_cents": amt,
			"pay_method":   "wechat",
		})
		require.Equal(t, http.StatusUnprocessableEntity, w.Code, "amount_cents=%d 响应体: %s", amt, w.Body.String())
		assert.Contains(t, decode(t, w).Message, "充值金额必须大于0", "amount_cents=%d", amt)
	}
}

// TestBizOrder_SeatNew_QuantityOverMax 覆盖 TC-10-013：数量超上限拒绝，quantity=1001 → 422「数量超过单次上限（最多 1000）」。
func TestBizOrder_SeatNew_QuantityOverMax(t *testing.T) {
	r := setupRouter()
	const phone = "13902000004"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	w := doJSON(r, "POST", "/api/v1/billing/orders", token, map[string]interface{}{
		"biz_type":       "seat_new",
		"quantity":       1001,
		"duration_value": 1,
		"pay_method":     "balance",
	})
	require.Equal(t, http.StatusUnprocessableEntity, w.Code, "响应体: %s", w.Body.String())
	assert.Contains(t, decode(t, w).Message, "数量超过单次上限", "应干净拒绝且不泄漏底层 SQL: %s", w.Body.String())
	assert.Contains(t, decode(t, w).Message, "1000")
}

// TestBizOrder_SeatNew_QuantityBelowMin 覆盖 TC-10-014：数量下限拒绝，quantity=0 → 422「数量必须≥1」。
func TestBizOrder_SeatNew_QuantityBelowMin(t *testing.T) {
	r := setupRouter()
	const phone = "13902000005"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	w := doJSON(r, "POST", "/api/v1/billing/orders", token, map[string]interface{}{
		"biz_type":       "seat_new",
		"quantity":       0,
		"duration_value": 1,
		"pay_method":     "balance",
	})
	require.Equal(t, http.StatusUnprocessableEntity, w.Code, "响应体: %s", w.Body.String())
	assert.Contains(t, decode(t, w).Message, "数量必须", "响应体: %s", w.Body.String())
}

// TestBizOrder_InvalidBizType 覆盖 TC-10-015 前半：非法 biz_type → 422「非法的业务类型」。
func TestBizOrder_InvalidBizType(t *testing.T) {
	r := setupRouter()
	const phone = "13902000006"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	w := doJSON(r, "POST", "/api/v1/billing/orders", token, map[string]interface{}{
		"biz_type":       "no_such_biz_type",
		"quantity":       1,
		"duration_value": 1,
		"pay_method":     "balance",
	})
	require.Equal(t, http.StatusUnprocessableEntity, w.Code, "响应体: %s", w.Body.String())
	assert.Contains(t, decode(t, w).Message, "非法的业务类型")
}

// TestBizOrder_InvalidPayMethod 覆盖 TC-10-015 后半：非白名单 pay_method → 422「不支持的支付方式」。
func TestBizOrder_InvalidPayMethod(t *testing.T) {
	r := setupRouter()
	const phone = "13902000007"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	w := doJSON(r, "POST", "/api/v1/billing/orders", token, map[string]interface{}{
		"biz_type":       "seat_new",
		"quantity":       1,
		"duration_value": 1,
		"pay_method":     "bitcoin",
	})
	require.Equal(t, http.StatusUnprocessableEntity, w.Code, "响应体: %s", w.Body.String())
	assert.Contains(t, decode(t, w).Message, "不支持的支付方式")
}

// TestBizOrder_DisabledPayMethod_RejectedAtOrder 覆盖 TC-10-016：被禁用支付方式后端兜底拒绝。
// admin 把 wechat 停用 → 用户直发该方式下单 → 422「该支付方式已停用」（不依赖前端隐藏）。
// 用完后 admin 恢复启用，避免污染共享 pricing 配置影响其它并行用例。
func TestBizOrder_DisabledPayMethod_RejectedAtOrder(t *testing.T) {
	r := setupRouter()
	const phone = "13902000008"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	adminTok := adminToken(t, r)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	// 读当前配置，找到 wechat 项，改 enabled=false，保存。
	gw := doJSON(r, "GET", "/api/v1/admin/billing/payment-methods", adminTok, nil)
	require.Equal(t, http.StatusOK, gw.Code, "读支付方式失败: %s", gw.Body.String())
	methods := decode(t, gw).Data.(map[string]interface{})["payment_methods"].([]interface{})

	toggled := make([]interface{}, 0, len(methods))
	for _, m := range methods {
		mm := m.(map[string]interface{})
		if mm["code"] == "wechat" {
			mm["enabled"] = false
		}
		toggled = append(toggled, mm)
	}
	pw := doJSON(r, "PUT", "/api/v1/admin/billing/payment-methods", adminTok, map[string]interface{}{
		"payment_methods": toggled,
	})
	require.Equal(t, http.StatusOK, pw.Code, "停用 wechat 失败: %s", pw.Body.String())

	// 用完恢复，避免污染共享配置影响其它并行/后续用例。
	t.Cleanup(func() {
		restored := make([]interface{}, 0, len(methods))
		for _, m := range methods {
			mm := m.(map[string]interface{})
			if mm["code"] == "wechat" {
				mm["enabled"] = true
			}
			restored = append(restored, mm)
		}
		rw := doJSON(r, "PUT", "/api/v1/admin/billing/payment-methods", adminTok, map[string]interface{}{
			"payment_methods": restored,
		})
		require.Equal(t, http.StatusOK, rw.Code, "恢复 wechat 启用失败: %s", rw.Body.String())
	})

	w := doJSON(r, "POST", "/api/v1/billing/orders", token, map[string]interface{}{
		"biz_type":       "seat_new",
		"quantity":       1,
		"duration_value": 1,
		"pay_method":     "wechat",
	})
	require.Equal(t, http.StatusUnprocessableEntity, w.Code, "响应体: %s", w.Body.String())
	assert.Contains(t, decode(t, w).Message, "该支付方式已停用")
}

// TestBizOrder_BalanceInsufficient_LeavesOrderUnpaidAndNoFulfillment 覆盖 TC-10-018：
// 余额不足失败回滚。用户余额为 0，直接用余额支付下单 seat_new（seed 单价 3000 分/台/月）：
//   - HTTP 层返回 422「余额不足」（apperr.Validation 映射）；
//   - framework.DB 三查：订单落库为 unpaid（create 与 pay 分属两个事务，pay 失败回滚不影响已 create 的订单行）、
//     未生成席位单元、余额仍为 0（未被扣）。
func TestBizOrder_BalanceInsufficient_LeavesOrderUnpaidAndNoFulfillment(t *testing.T) {
	r := setupRouter()
	const phone = "13902000009"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	// 确认余额基线为 0（未充值）。
	ow := doJSON(r, "GET", "/api/v1/billing/overview", token, nil)
	require.Equal(t, http.StatusOK, ow.Code, "overview 失败: %s", ow.Body.String())
	balBefore := decode(t, ow).Data.(map[string]interface{})["balance_cents"].(float64)
	require.Equal(t, float64(0), balBefore, "新注册用户余额应为 0")

	w := doJSON(r, "POST", "/api/v1/billing/orders", token, map[string]interface{}{
		"biz_type":       "seat_new",
		"quantity":       1,
		"duration_value": 1,
		"pay_method":     "balance",
	})
	require.Equal(t, http.StatusUnprocessableEntity, w.Code, "响应体: %s", w.Body.String())
	assert.Contains(t, decode(t, w).Message, "余额不足")

	// 订单落库为 unpaid（create 独立事务先提交，pay 事务失败回滚不影响该行）。
	var orderCount int64
	require.NoError(t, framework.DB.Table("billing_biz_orders").
		Where("user_id = ? AND biz_type = ?", uid, "seat_new").Count(&orderCount).Error)
	require.Equal(t, int64(1), orderCount, "下单应落一行订单")
	var status string
	require.NoError(t, framework.DB.Table("billing_biz_orders").
		Where("user_id = ? AND biz_type = ?", uid, "seat_new").Select("status").Scan(&status).Error)
	assert.Equal(t, "unpaid", status, "余额不足支付失败后订单应保持 unpaid")

	// 未生成席位单元（履约随支付事务回滚）。
	var unitCount int64
	require.NoError(t, framework.DB.Table("billing_license_units").
		Where("user_id = ? AND kind = ?", uid, "seat").Count(&unitCount).Error)
	assert.Equal(t, int64(0), unitCount, "余额不足履约不应发生")

	// 余额不变（仍为 0，未被部分扣款）。
	aw := doJSON(r, "GET", "/api/v1/billing/overview", token, nil)
	require.Equal(t, http.StatusOK, aw.Code, "overview 失败: %s", aw.Body.String())
	balAfter := decode(t, aw).Data.(map[string]interface{})["balance_cents"].(float64)
	assert.Equal(t, float64(0), balAfter, "余额应保持不变")
}

// TestBizOrder_PayExpiredOrder_Rejected 覆盖 TC-10-032：支付过期订单拒绝。
// 用余额支付方式先建一笔 unpaid 订单不便（余额支付恒即时结算成 paid），故改用第三方方式下单
// 后直接经 framework.DB 把订单状态回填为 expired（模拟系统判定过期，不依赖未接线的到期任务），
// 再 POST /orders/:id/pay → 422「订单已过期」。
func TestBizOrder_PayExpiredOrder_Rejected(t *testing.T) {
	r := setupRouter()
	const phone = "13902000010"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	// wechat 桩网关当前实现为即时结算（下单即 paid），为构造 expired 前置态直接经 DB 回填状态，
	// 不依赖桩网关行为（该行为属另一测试点，见 bizorder_service_test.go:TestBizOrder_ThirdPartyAutoSettles）。
	w := doJSON(r, "POST", "/api/v1/billing/orders", token, map[string]interface{}{
		"biz_type":       "seat_new",
		"quantity":       1,
		"duration_value": 1,
		"pay_method":     "wechat",
	})
	require.Equal(t, http.StatusOK, w.Code, "下单失败: %s", w.Body.String())
	order := decode(t, w).Data.(map[string]interface{})["order"].(map[string]interface{})
	orderID := int(order["id"].(float64))
	require.NotZero(t, orderID)

	// 直接回填该订单状态为 expired（模拟到期判定，绕开尚未实现的到期任务）。
	res := framework.DB.Exec(
		"UPDATE billing_biz_orders SET status = ?, expired_at = ? WHERE id = ? AND user_id = ?",
		"expired", time.Now(), orderID, uid,
	)
	require.NoError(t, res.Error)
	require.EqualValues(t, 1, res.RowsAffected, "应恰好回填 1 行")

	pw := doJSON(r, "POST", fmt.Sprintf("/api/v1/billing/orders/%d/pay", orderID), token, nil)
	require.Equal(t, http.StatusUnprocessableEntity, pw.Code, "响应体: %s", pw.Body.String())
	assert.Contains(t, decode(t, pw).Message, "订单已过期")
}
