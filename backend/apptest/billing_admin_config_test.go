package apptest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// billing_admin_config_test.go 对应 Plan B 任务 6：admin 配置写入 → 前台用户端立即生效的跨子系统
// E2E，以及后台统一资源赠送。覆盖 TC-ID：
//   - TC-10-026 admin 定价读写：PUT /admin/billing/pricing 改价后，用户端 GET /billing/purchase-config
//     与 POST /billing/quote 立即读到新价。
//   - TC-10-025 admin 支付方式手续费校验：fee_percent_bps 超界 / fee_fixed_cents 为负 / balance 方式
//     配非 0 手续费，均应 400（AdminSavePaymentMethods 用 framework.Fail 直给状态码，非 apperr）。
//   - TC-10-023 手续费下单快照固定：下单后 admin 改该支付方式手续费，历史订单 fee_* 字段不变。
//   - TC-10-024 手续费-0 元订单不收费：total_cents=0 的订单 fee_cents=0（含固定部分）。
//   - TC-10-087 后台资源赠送（统一履约）：POST /admin/billing/accounts/:userId/adjust-resource 对
//     seat/boot_slot/runtime_minute 三科目均走 FulfillNew/FulfillRuntimePack（source=grant），
//     履约生效 + adjust_grant 流水落账。
//   - TC-10-092 赠送关闭：gift_minutes_per_seat_month=0 时，seat_new/seat_renew 均不赠送
//     （gift_runtime_minutes=0），仅本体履约照常发生。
//
// 定价/支付方式/时长包配置（PricingConfigData）是进程级单例（单行 JSON，id=1），apptest 与其他
// 测试文件共享同一张表；本文件所有改配置的用例均遵循「GET 现状 → 改 → t.Cleanup 原样 PUT 回」的
// 惯例（与 billing_order_validation_test.go:TestBizOrder_DisabledPayMethod_RejectedAtOrder 一致），
// 绝不 CleanTable 该配置表，避免污染 billing_quote_test.go 等依赖默认值（如
// GiftMinutesPerSeatMonth=200）的用例。
//
// 手机号：本文件用例专用 13906000001~13906000005（唯一，不与其他文件重复）。

// adminCfgGetPricingKinds 读取当前 admin 定价配置的 kinds 表。
func adminCfgGetPricingKinds(t *testing.T, r *gin.Engine, adminTok string) map[string]interface{} {
	t.Helper()
	w := doJSON(r, "GET", "/api/v1/admin/billing/pricing", adminTok, nil)
	require.Equal(t, http.StatusOK, w.Code, "读取定价配置失败: %s", w.Body.String())
	return decode(t, w).Data.(map[string]interface{})["kinds"].(map[string]interface{})
}

// adminCfgPutPricingKind 覆盖保存某 kind 的完整定价配置（AdminSavePricing 按 key 整体替换，
// 不做嵌套合并，故调用方必须传完整 KindPricing 形状，否则会把该 kind 的其余字段清零）。
func adminCfgPutPricingKind(t *testing.T, r *gin.Engine, adminTok, kind string, body map[string]interface{}) {
	t.Helper()
	w := doJSON(r, "PUT", "/api/v1/admin/billing/pricing", adminTok, map[string]interface{}{
		"kinds": map[string]interface{}{kind: body},
	})
	require.Equal(t, http.StatusOK, w.Code, "保存定价配置(kind=%s)失败: %s", kind, w.Body.String())
}

// adminCfgGetRuntimeConfig 读取当前时长包配置（RuntimePackCfg 全量）。
func adminCfgGetRuntimeConfig(t *testing.T, r *gin.Engine, adminTok string) map[string]interface{} {
	t.Helper()
	w := doJSON(r, "GET", "/api/v1/admin/billing/runtime-config", adminTok, nil)
	require.Equal(t, http.StatusOK, w.Code, "读取时长包配置失败: %s", w.Body.String())
	return decode(t, w).Data.(map[string]interface{})
}

// adminCfgPutRuntimeConfig 整体覆盖保存时长包配置（AdminSaveRuntimePricing 整体替换 Runtime 结构体）。
func adminCfgPutRuntimeConfig(t *testing.T, r *gin.Engine, adminTok string, body map[string]interface{}) {
	t.Helper()
	w := doJSON(r, "PUT", "/api/v1/admin/billing/runtime-config", adminTok, body)
	require.Equal(t, http.StatusOK, w.Code, "保存时长包配置失败: %s", w.Body.String())
}

// adminCfgGetPaymentMethods 读取当前支付方式配置数组（[]interface{}，元素为 map）。
func adminCfgGetPaymentMethods(t *testing.T, r *gin.Engine, adminTok string) []interface{} {
	t.Helper()
	w := doJSON(r, "GET", "/api/v1/admin/billing/payment-methods", adminTok, nil)
	require.Equal(t, http.StatusOK, w.Code, "读取支付方式失败: %s", w.Body.String())
	return decode(t, w).Data.(map[string]interface{})["payment_methods"].([]interface{})
}

// adminCfgPutPaymentMethods 整体覆盖保存支付方式数组，返回响应记录器供调用方断言状态码/文案。
func adminCfgPutPaymentMethods(r *gin.Engine, adminTok string, methods []interface{}) *httptest.ResponseRecorder {
	return doJSON(r, "PUT", "/api/v1/admin/billing/payment-methods", adminTok, map[string]interface{}{
		"payment_methods": methods,
	})
}

// adminCfgCloneMethods 深拷贝支付方式数组（避免调用方原地改动影响后续 t.Cleanup 里的原样恢复）。
func adminCfgCloneMethods(methods []interface{}) []interface{} {
	out := make([]interface{}, 0, len(methods))
	for _, m := range methods {
		mm := m.(map[string]interface{})
		cp := make(map[string]interface{}, len(mm))
		for k, v := range mm {
			cp[k] = v
		}
		out = append(out, cp)
	}
	return out
}

// adminCfgRestorePaymentMethods 以原始数组原样 PUT 回，恢复共享配置，供 t.Cleanup 使用。
func adminCfgRestorePaymentMethods(t *testing.T, r *gin.Engine, adminTok string, original []interface{}) {
	t.Helper()
	w := doJSON(r, "PUT", "/api/v1/admin/billing/payment-methods", adminTok, map[string]interface{}{
		"payment_methods": original,
	})
	require.Equal(t, http.StatusOK, w.Code, "恢复支付方式配置失败: %s", w.Body.String())
}

// TestAdminConfig_PricingWriteReflectsInUserQuoteAndPurchaseConfig 覆盖 TC-10-026：
// admin 改 boot_slot 单价 → 用户端 GET /billing/purchase-config 与 POST /billing/quote
// 均立即读到新价（无缓存滞后），且报价按新单价 × 数量 × 时长正确核算。
// 用完 t.Cleanup 原样 PUT 回旧的 kind 配置，避免影响其他依赖默认单价(2000分/个)的用例。
func TestAdminConfig_PricingWriteReflectsInUserQuoteAndPurchaseConfig(t *testing.T) {
	r := setupRouter()
	adminTok := adminToken(t, r)
	const phone = "13906000001"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	// 读现状（保留 boot_slot 原始配置，用完恢复）。
	kindsBefore := adminCfgGetPricingKinds(t, r, adminTok)
	bootBefore := kindsBefore["boot_slot"].(map[string]interface{})
	t.Cleanup(func() { adminCfgPutPricingKind(t, r, adminTok, "boot_slot", bootBefore) })

	// 改单价：2000 → 9999 分/个，其余字段原样保留（duration_options 必须带全，否则 7 天档会消失）。
	newPrice := map[string]interface{}{
		"unit_price_cents": 9999,
		"unit_label":       bootBefore["unit_label"],
		"qty_tiers":        bootBefore["qty_tiers"],
		"duration_unit":    bootBefore["duration_unit"],
		"duration_options": bootBefore["duration_options"],
		"notice":           bootBefore["notice"],
		"billing_note":     bootBefore["billing_note"],
	}
	adminCfgPutPricingKind(t, r, adminTok, "boot_slot", newPrice)

	// 用户端 purchase-config 应立即读到新单价。
	pw := doJSON(r, "GET", "/api/v1/billing/purchase-config", token, nil)
	require.Equal(t, http.StatusOK, pw.Code, "purchase-config 失败: %s", pw.Body.String())
	pcKinds := decode(t, pw).Data.(map[string]interface{})["kinds"].(map[string]interface{})
	pcBoot := pcKinds["boot_slot"].(map[string]interface{})
	assert.Equal(t, float64(9999), pcBoot["unit_price_cents"], "purchase-config 应读到新单价")

	// quote 应按新单价核算：boot_slot 计价 billingUnits = quantity × duration_value（computeQuote，
	// duration_unit=天），quantity=1, duration_value=7 → original = 9999 × (1×7) = 69993；
	// 7 天档 discount_bps=10000（无折扣）→ payable = original。
	qw := doJSON(r, "POST", "/api/v1/billing/quote", token, map[string]interface{}{
		"biz_type":       "boot_slot_new",
		"quantity":       1,
		"duration_value": 7,
	})
	require.Equal(t, http.StatusOK, qw.Code, "quote 失败: %s", qw.Body.String())
	qData := decode(t, qw).Data.(map[string]interface{})
	assert.Equal(t, float64(9999), qData["unit_price_cents"], "quote 应读到新单价")
	assert.Equal(t, float64(69993), qData["original_cents"], "1个×7天×9999分/天=69993分")
	assert.Equal(t, float64(69993), qData["payable_cents"], "7天档 discount_bps=10000 无折扣，应付=原价")
}

// TestAdminConfig_PaymentMethodValidationRejectsIllegalFee 覆盖 TC-10-025：
// PUT /admin/billing/payment-methods 对非法手续费配置一律 400（AdminSavePaymentMethods 用
// framework.Fail 直给状态码，非 apperr.Validation，故为 400 而非 422），且给出具体校验文案：
//   - fee_percent_bps 超上限(10001>10000)
//   - fee_fixed_cents 为负
//   - balance 方式配非 0 比例手续费（该方式手续费恒须为 0）
//
// 三种非法配置均不应改变已保存的配置（校验在 SavePartial 之前拦截，落库前置失败）。
func TestAdminConfig_PaymentMethodValidationRejectsIllegalFee(t *testing.T) {
	r := setupRouter()
	adminTok := adminToken(t, r)

	original := adminCfgGetPaymentMethods(t, r, adminTok)
	t.Cleanup(func() { adminCfgRestorePaymentMethods(t, r, adminTok, adminCfgCloneMethods(original)) })

	// 定位数组中第一个非 balance 项 / balance 项的下标（默认 seed 含 wechat/alipay/balance，均存在）。
	firstNonBalance := -1
	firstBalance := -1
	for i, m := range original {
		mm := m.(map[string]interface{})
		if mm["code"] == "balance" {
			if firstBalance == -1 {
				firstBalance = i
			}
			continue
		}
		if firstNonBalance == -1 {
			firstNonBalance = i
		}
	}
	require.NotEqual(t, -1, firstNonBalance, "seed 应含至少一个非 balance 支付方式")
	require.NotEqual(t, -1, firstBalance, "seed 应含 balance 支付方式")

	cases := []struct {
		name      string
		targetIdx int
		mutate    func(m map[string]interface{})
		wantMsg   string
	}{
		{
			name:      "比例手续费超上限",
			targetIdx: firstNonBalance,
			mutate: func(m map[string]interface{}) {
				m["fee_percent_bps"] = 10001
				m["fee_fixed_cents"] = 0
			},
			wantMsg: "比例手续费需在 0-10000 基点",
		},
		{
			name:      "固定手续费为负",
			targetIdx: firstNonBalance,
			mutate: func(m map[string]interface{}) {
				m["fee_percent_bps"] = 200
				m["fee_fixed_cents"] = -1
			},
			wantMsg: "固定手续费不能为负",
		},
		{
			name:      "余额方式非0比例手续费",
			targetIdx: firstBalance,
			mutate: func(m map[string]interface{}) {
				m["fee_percent_bps"] = 100
				m["fee_fixed_cents"] = 0
			},
			wantMsg: "余额支付不可配置手续费",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := adminCfgCloneMethods(original)
			tc.mutate(payload[tc.targetIdx].(map[string]interface{}))

			w := adminCfgPutPaymentMethods(r, adminTok, payload)
			require.Equal(t, http.StatusBadRequest, w.Code, "%s 应 400: %s", tc.name, w.Body.String())
			assert.Contains(t, decode(t, w).Message, tc.wantMsg, "%s 应给出具体校验文案", tc.name)
		})
	}

	// 三次非法保存均未落库：当前配置应仍与恢复前一致（用 fee_percent_bps 为特征比对，避免全字段深比）。
	final := adminCfgGetPaymentMethods(t, r, adminTok)
	require.Equal(t, len(original), len(final), "非法保存不应改变支付方式条数")
	for i, m := range original {
		om := m.(map[string]interface{})
		fm := final[i].(map[string]interface{})
		assert.Equal(t, om["code"], fm["code"])
		assert.Equal(t, om["fee_percent_bps"], fm["fee_percent_bps"], "code=%v 的手续费不应被非法请求污染", om["code"])
	}
}

// TestAdminConfig_FeeSnapshotFixedAfterConfigChange 覆盖 TC-10-023：
// 用户以 wechat（配置 2% + 固定 1 元手续费）下单 seat_new 后，admin 把 wechat 手续费改成
// 5% + 固定 5 元；重查该历史订单详情，fee_percent_bps/fee_fixed_cents/fee_cents 均应保持
// 下单时的快照值不变（对账口径不能被后续改配置污染）。
func TestAdminConfig_FeeSnapshotFixedAfterConfigChange(t *testing.T) {
	r := setupRouter()
	adminTok := adminToken(t, r)
	const phone = "13906000002"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	original := adminCfgGetPaymentMethods(t, r, adminTok)
	t.Cleanup(func() { adminCfgRestorePaymentMethods(t, r, adminTok, adminCfgCloneMethods(original)) })

	// 1) 设置 wechat 手续费为 2%(200bps) + 固定 100 分(¥1)。
	stepOne := adminCfgCloneMethods(original)
	for _, m := range stepOne {
		mm := m.(map[string]interface{})
		if mm["code"] == "wechat" {
			mm["fee_percent_bps"] = 200
			mm["fee_fixed_cents"] = 100
			mm["enabled"] = true
		}
	}
	wSet1 := doJSON(r, "PUT", "/api/v1/admin/billing/payment-methods", adminTok,
		map[string]interface{}{"payment_methods": stepOne})
	require.Equal(t, http.StatusOK, wSet1.Code, "设置手续费失败: %s", wSet1.Body.String())

	// 2) 用户下单 seat_new，quantity=1,duration_value=1，pay_method=wechat（第三方桩即时结算）。
	wCreate := doJSON(r, "POST", "/api/v1/billing/orders", token, map[string]interface{}{
		"biz_type":       "seat_new",
		"quantity":       1,
		"duration_value": 1,
		"pay_method":     "wechat",
	})
	require.Equal(t, http.StatusOK, wCreate.Code, "下单失败: %s", wCreate.Body.String())
	order := decode(t, wCreate).Data.(map[string]interface{})["order"].(map[string]interface{})
	orderID := int(order["id"].(float64))
	require.NotZero(t, orderID)
	totalCents := int64(order["total_cents"].(float64))
	require.Equal(t, int64(3000), totalCents, "seed: 席位 3000 分/台 × 1 台 × 1 月")
	// 下单快照：2%×3000=60 + 100 = 160。
	require.Equal(t, float64(160), order["fee_cents"], "下单时手续费快照应为 60+100=160")

	// 3) admin 改 wechat 手续费为 5%(500bps) + 固定 500 分(¥5)。
	stepTwo := adminCfgCloneMethods(original)
	for _, m := range stepTwo {
		mm := m.(map[string]interface{})
		if mm["code"] == "wechat" {
			mm["fee_percent_bps"] = 500
			mm["fee_fixed_cents"] = 500
			mm["enabled"] = true
		}
	}
	wSet2 := doJSON(r, "PUT", "/api/v1/admin/billing/payment-methods", adminTok,
		map[string]interface{}{"payment_methods": stepTwo})
	require.Equal(t, http.StatusOK, wSet2.Code, "改手续费失败: %s", wSet2.Body.String())

	// 4) 重查该历史订单：fee_* 字段不随新配置变化，仍为下单时的快照。
	gw := doJSON(r, "GET", fmt.Sprintf("/api/v1/billing/orders/%d", orderID), token, nil)
	require.Equal(t, http.StatusOK, gw.Code, "查订单详情失败: %s", gw.Body.String())
	got := decode(t, gw).Data.(map[string]interface{})
	assert.Equal(t, float64(200), got["fee_percent_bps"], "历史订单 fee_percent_bps 应保持下单时快照 200bps")
	assert.Equal(t, float64(100), got["fee_fixed_cents"], "历史订单 fee_fixed_cents 应保持下单时快照 100分")
	assert.Equal(t, float64(160), got["fee_cents"], "历史订单 fee_cents 应保持下单时快照 160分，不随新配置(5%+500)变化")
}

// TestAdminConfig_ZeroCentOrderChargesNoFee 覆盖 TC-10-024：
// total_cents=0 的订单（admin 把 boot_slot 单价改成 0）以第三方方式(wechat，配置非 0 手续费)
// 下单，fee_cents 仍应为 0（computeFee 对 baseCents<=0 直接短路返回 0，含固定部分也不收）。
func TestAdminConfig_ZeroCentOrderChargesNoFee(t *testing.T) {
	r := setupRouter()
	adminTok := adminToken(t, r)
	const phone = "13906000003"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	// 恢复 boot_slot 定价与 wechat 手续费。
	kindsBefore := adminCfgGetPricingKinds(t, r, adminTok)
	bootBefore := kindsBefore["boot_slot"].(map[string]interface{})
	t.Cleanup(func() { adminCfgPutPricingKind(t, r, adminTok, "boot_slot", bootBefore) })

	methodsBefore := adminCfgGetPaymentMethods(t, r, adminTok)
	t.Cleanup(func() { adminCfgRestorePaymentMethods(t, r, adminTok, adminCfgCloneMethods(methodsBefore)) })

	// 1) boot_slot 单价改为 0。
	zeroPrice := map[string]interface{}{
		"unit_price_cents": 0,
		"unit_label":       bootBefore["unit_label"],
		"qty_tiers":        bootBefore["qty_tiers"],
		"duration_unit":    bootBefore["duration_unit"],
		"duration_options": bootBefore["duration_options"],
		"notice":           bootBefore["notice"],
		"billing_note":     bootBefore["billing_note"],
	}
	adminCfgPutPricingKind(t, r, adminTok, "boot_slot", zeroPrice)

	// 2) wechat 配非 0 手续费（2% + 固定 1 元），验证 0 元订单仍不收（含固定部分）。
	feeMethods := adminCfgCloneMethods(methodsBefore)
	for _, m := range feeMethods {
		mm := m.(map[string]interface{})
		if mm["code"] == "wechat" {
			mm["fee_percent_bps"] = 200
			mm["fee_fixed_cents"] = 100
			mm["enabled"] = true
		}
	}
	wSetFee := doJSON(r, "PUT", "/api/v1/admin/billing/payment-methods", adminTok,
		map[string]interface{}{"payment_methods": feeMethods})
	require.Equal(t, http.StatusOK, wSetFee.Code, "设置手续费失败: %s", wSetFee.Body.String())

	// 3) 下单 boot_slot_new，quantity=1,duration_value=7（原价档 discount_bps=10000）→ total=0。
	wCreate := doJSON(r, "POST", "/api/v1/billing/orders", token, map[string]interface{}{
		"biz_type":       "boot_slot_new",
		"quantity":       1,
		"duration_value": 7,
		"pay_method":     "wechat",
	})
	require.Equal(t, http.StatusOK, wCreate.Code, "下单失败: %s", wCreate.Body.String())
	order := decode(t, wCreate).Data.(map[string]interface{})["order"].(map[string]interface{})
	assert.Equal(t, float64(0), order["total_cents"], "单价 0 × 数量 1 × 7 天应为 0 元")
	assert.Equal(t, float64(0), order["fee_cents"], "0 元订单不应收手续费（含固定部分）")
}

// TestAdminConfig_AdjustResourceGrantsSeatBootSlotRuntimeMinute 覆盖 TC-10-087：
// POST /admin/billing/accounts/:userId/adjust-resource 对三种科目分别走统一履约：
//   - subject=seat/boot_slot：FulfillNew(source=grant) 生成对应数量的授权单元，overview 容量增加；
//   - subject=runtime_minute：FulfillRuntimePack(source=grant) 增加临时时长钱包余量；
//
// 三者均应在 billing_ledger_entries 落一条 type=adjust_grant 的流水（subject 对应科目，
// reason=ref="staff:<staffID>"，delta 与操作量一致）。
func TestAdminConfig_AdjustResourceGrantsSeatBootSlotRuntimeMinute(t *testing.T) {
	r := setupRouter()
	adminTok := adminToken(t, r)
	const phone = "13906000004"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM billing_runtime_minute_wallets WHERE user_id = ?", uid) })

	// overview 基线：全 0。
	baseline := billingOverviewData(t, r, token)
	require.Equal(t, float64(0), baseline["seat"].(map[string]interface{})["total"])
	require.Equal(t, float64(0), baseline["boot_slot"].(map[string]interface{})["total"])
	require.Equal(t, float64(0), baseline["runtime_minutes_remaining"])

	// 1) 赠送 3 个 seat，时长 1 个月。
	wSeat := doJSON(r, "POST", fmt.Sprintf("/api/v1/admin/billing/accounts/%d/adjust-resource", uid), adminTok,
		map[string]interface{}{"subject": "seat", "quantity": 3, "duration_value": 1, "reason": "运营赠送"})
	require.Equal(t, http.StatusOK, wSeat.Code, "赠送 seat 失败: %s", wSeat.Body.String())

	// 2) 赠送 2 个 boot_slot，时长 7 天。
	wBoot := doJSON(r, "POST", fmt.Sprintf("/api/v1/admin/billing/accounts/%d/adjust-resource", uid), adminTok,
		map[string]interface{}{"subject": "boot_slot", "quantity": 2, "duration_value": 7, "reason": "运营赠送"})
	require.Equal(t, http.StatusOK, wBoot.Code, "赠送 boot_slot 失败: %s", wBoot.Body.String())

	// 3) 赠送 500 分钟临时时长。
	wRuntime := doJSON(r, "POST", fmt.Sprintf("/api/v1/admin/billing/accounts/%d/adjust-resource", uid), adminTok,
		map[string]interface{}{"subject": "runtime_minute", "minutes": 500, "reason": "运营赠送"})
	require.Equal(t, http.StatusOK, wRuntime.Code, "赠送 runtime_minute 失败: %s", wRuntime.Body.String())

	// 履约生效：overview 容量三项均按赠送量增加。
	after := billingOverviewData(t, r, token)
	assert.Equal(t, float64(3), after["seat"].(map[string]interface{})["total"], "seat 容量应 +3")
	assert.Equal(t, float64(2), after["boot_slot"].(map[string]interface{})["total"], "boot_slot 容量应 +2")
	assert.Equal(t, float64(500), after["runtime_minutes_remaining"], "临时时长钱包应 +500 分钟")

	// 流水：三条 adjust_grant，delta 与赠送量一致。
	assert.Equal(t, int64(3), adminCfgLedgerDelta(t, uid, "seat", "adjust_grant"), "seat 流水 delta 应为 3")
	assert.Equal(t, int64(2), adminCfgLedgerDelta(t, uid, "boot_slot", "adjust_grant"), "boot_slot 流水 delta 应为 2")
	assert.Equal(t, int64(500), adminCfgLedgerDelta(t, uid, "runtime_minute", "adjust_grant"), "runtime_minute 流水 delta 应为 500")
}

// TestAdminConfig_GiftDisabledNoGrantOnSeatPurchase 覆盖 TC-10-092：
// admin 把 gift_minutes_per_seat_month 改为 0 后，seat_new 报价与下单的 gift_runtime_minutes
// 均应为 0（不赠送），但席位本体履约（授权单元）照常发生，不受赠送开关影响。
// 用完 t.Cleanup 原样恢复 runtime-config，避免影响 billing_quote_test.go 等依赖默认
// GiftMinutesPerSeatMonth=200 的用例。
func TestAdminConfig_GiftDisabledNoGrantOnSeatPurchase(t *testing.T) {
	r := setupRouter()
	adminTok := adminToken(t, r)
	const phone = "13906000005"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM billing_runtime_minute_wallets WHERE user_id = ?", uid) })

	rtBefore := adminCfgGetRuntimeConfig(t, r, adminTok)
	t.Cleanup(func() { adminCfgPutRuntimeConfig(t, r, adminTok, rtBefore) })
	require.NotEqual(t, float64(0), rtBefore["gift_minutes_per_seat_month"],
		"前置条件：默认赠送应非 0，否则本用例不能证明关闭生效")

	// 关闭赠送：其余字段原样保留，仅 gift_minutes_per_seat_month 置 0。
	rtOff := map[string]interface{}{
		"unit_price_cents_per_minute": rtBefore["unit_price_cents_per_minute"],
		"min_minutes":                 rtBefore["min_minutes"],
		"packs":                       rtBefore["packs"],
		"notice":                      rtBefore["notice"],
		"daily_cap_minutes":           rtBefore["daily_cap_minutes"],
		"gift_minutes_per_seat_month": 0,
	}
	adminCfgPutRuntimeConfig(t, r, adminTok, rtOff)

	// quote 预览：gift_runtime_minutes=0。
	qw := doJSON(r, "POST", "/api/v1/billing/quote", token, map[string]interface{}{
		"biz_type":       "seat_new",
		"quantity":       2,
		"duration_value": 1,
	})
	require.Equal(t, http.StatusOK, qw.Code, "quote 失败: %s", qw.Body.String())
	assert.Equal(t, float64(0), decode(t, qw).Data.(map[string]interface{})["gift_runtime_minutes"],
		"赠送关闭后 quote 预览应为 0")

	// 充值余额以便余额下单。
	wTopup := doJSON(r, "POST", "/api/v1/billing/topup", token, map[string]interface{}{"amount_cents": 100000})
	require.Equal(t, http.StatusOK, wTopup.Code, "充值失败: %s", wTopup.Body.String())

	// 下单 seat_new：本体履约照常（2 个席位），但不赠送临时时长。
	wCreate := doJSON(r, "POST", "/api/v1/billing/orders", token, map[string]interface{}{
		"biz_type":       "seat_new",
		"quantity":       2,
		"duration_value": 1,
		"pay_method":     "balance",
	})
	require.Equal(t, http.StatusOK, wCreate.Code, "下单失败: %s", wCreate.Body.String())
	order := decode(t, wCreate).Data.(map[string]interface{})["order"].(map[string]interface{})
	assert.Equal(t, "paid", order["status"], "余额购买应即时置已付")
	assert.Equal(t, float64(0), order["gift_runtime_minutes"], "赠送关闭后订单落库 gift_runtime_minutes 应为 0")

	overview := billingOverviewData(t, r, token)
	assert.Equal(t, float64(2), overview["seat"].(map[string]interface{})["total"], "席位本体履约应照常生效(2个)")
	assert.Equal(t, float64(0), overview["runtime_minutes_remaining"], "赠送关闭后临时时长钱包应仍为 0")
}

// billingOverviewData 打 GET /billing/overview 取 Data（本文件专用别名，避免与其他文件的
// libOverview/seatTotal 等同类 helper 混淆；仅返回原始 map 由调用方自行断言字段）。
func billingOverviewData(t *testing.T, r *gin.Engine, token string) map[string]interface{} {
	t.Helper()
	w := doJSON(r, "GET", "/api/v1/billing/overview", token, nil)
	require.Equal(t, http.StatusOK, w.Code, "overview 失败: %s", w.Body.String())
	return decode(t, w).Data.(map[string]interface{})
}

// adminCfgLedgerDelta 查某用户在指定科目(subject)+流水类型(type)下的流水 delta 之和
// （本文件场景每科目只操作一次，取 SUM 等价于取唯一一条的 delta，且对多次调用更健壮）。
func adminCfgLedgerDelta(t *testing.T, userID uint, subject, ledgerType string) int64 {
	t.Helper()
	var sum int64
	require.NoError(t, framework.DB.Table("billing_ledger_entries").
		Where("user_id = ? AND subject = ? AND type = ?", userID, subject, ledgerType).
		Select("COALESCE(SUM(delta), 0)").Scan(&sum).Error)
	return sum
}
