package apptest

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// billing_quote_test.go 覆盖 TC-10-001~005（报价矩阵）与 TC-10-090~092（赠送时长预览）。
// 端点 POST /api/v1/billing/quote（BizQuoteRequest）；报价为纯读操作、不落库订单，
// 因此各子测试只需注册用户 + 清理 users 行本身，无需清理 billing 订单/账本。
//
// 承接关系：modules/billing/internal/api_handler_test.go 的 TestBizQuote_OK/_BadRequest 只覆盖
// 「200 基本形状」与「绑定失败 400」，本文件在 HTTP 层补齐服务端权威报价的具体数值矩阵
// （数量阶梯折扣命中、时长非法/命中选项、时长包阶梯、充值报价拒绝）与赠送时长预览口径，
// 与 internal 单测不重复。
//
// 用例点使用的定价基线为 modules/billing/internal/pricingconfig_repository.go 的
// defaultPricingConfig（未被其他 apptest 文件改写）：
//   - seat：单价 3000 分/台/月；数量阶梯 ≥10→9000bps、≥100→8000bps；
//     时长选项 1月→10000bps、3月→8500bps、12月→7000bps；赠送 200 分钟/席位/月。
//   - boot_slot：单价 2000 分/个/天；数量阶梯 ≥10→9000bps；时长选项 7天→10000bps、30天→9000bps。
//   - runtime_pack：单价 20 分/分钟；min_minutes=60；包阶梯 600分钟→10000bps、3000分钟→9000bps。

// TestBillingQuote_SeatNew 覆盖 TC-10-001：席位新购报价，断言服务端权威口径
// unit_price_cents/billing_units/original_cents/qty_discount_bps/duration_discount_bps/payable_cents
// 与手算完全一致，且席位报价附带 gift_runtime_minutes（赠送时长预览）。
func TestBillingQuote_SeatNew(t *testing.T) {
	const phone = "13901000001"
	r := setupRouter()
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	// quantity=2（未命中任何数量阶梯，全价 10000bps）、duration_value=3（命中时长选项 8500bps）。
	w := doJSON(r, "POST", "/api/v1/billing/quote", token, map[string]interface{}{
		"biz_type":       "seat_new",
		"quantity":       2,
		"duration_value": 3,
	})
	require.Equal(t, http.StatusOK, w.Code, "报价应成功: %s", w.Body.String())
	data := decode(t, w).Data.(map[string]interface{})

	assert.Equal(t, float64(2), data["quantity"])
	assert.Equal(t, float64(3), data["duration_value"])
	assert.Equal(t, float64(3000), data["unit_price_cents"], "seed: 席位单价 3000 分/台/月")
	assert.Equal(t, float64(6), data["billing_units"], "billing_units = quantity(2) × duration_value(3)")
	assert.Equal(t, float64(18000), data["original_cents"], "original = 3000×6")
	assert.Equal(t, float64(10000), data["qty_discount_bps"], "quantity=2 未命中任何数量阶梯，应全价")
	assert.Equal(t, float64(8500), data["duration_discount_bps"], "duration_value=3 命中时长选项 8500bps")
	assert.Equal(t, float64(15300), data["payable_cents"], "payable = round(round(18000×10000/10000)×8500/10000)")
	assert.Equal(t, float64(1200), data["gift_runtime_minutes"],
		"seed: 每席位每月赠送 200 分钟 × 2 台 × 3 月 = 1200")
}

// TestBillingQuote_SeatQtyTierDiscount 覆盖 TC-10-002：数量阶梯折扣按「≤quantity 中门槛最高档」命中，
// 未达档全价（10000bps）。固定 duration_value=1（全价 10000bps）以隔离数量折扣变量。
func TestBillingQuote_SeatQtyTierDiscount(t *testing.T) {
	const phone = "13901000002"
	r := setupRouter()
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	seatQuote := func(quantity int) map[string]interface{} {
		w := doJSON(r, "POST", "/api/v1/billing/quote", token, map[string]interface{}{
			"biz_type":       "seat_new",
			"quantity":       quantity,
			"duration_value": 1,
		})
		require.Equal(t, http.StatusOK, w.Code, "quantity=%d 报价应成功: %s", quantity, w.Body.String())
		return decode(t, w).Data.(map[string]interface{})
	}

	// 未达 10 档：全价 10000bps。
	below := seatQuote(9)
	assert.Equal(t, float64(10000), below["qty_discount_bps"], "quantity=9 未达 10 档，应全价")
	assert.Equal(t, float64(27000), below["original_cents"], "9×3000")
	assert.Equal(t, float64(27000), below["payable_cents"])

	// 达到 10 档但未达 100 档：命中 9000bps。
	mid := seatQuote(50)
	assert.Equal(t, float64(9000), mid["qty_discount_bps"], "quantity=50 达到 10 档，应命中 9000bps")
	assert.Equal(t, float64(150000), mid["original_cents"], "50×3000")
	assert.Equal(t, float64(135000), mid["payable_cents"], "150000×9000/10000")

	// 达到 100 档：命中最优 8000bps。
	high := seatQuote(100)
	assert.Equal(t, float64(8000), high["qty_discount_bps"], "quantity=100 达到 100 档，应命中最优 8000bps")
	assert.Equal(t, float64(300000), high["original_cents"], "100×3000")
	assert.Equal(t, float64(240000), high["payable_cents"], "300000×8000/10000")
}

// TestBillingQuote_DurationMustMatchOption 覆盖 TC-10-003：时长不可手输，必须精确命中某个
// duration_options 的 value，否则 422「非法的购买时长」。
func TestBillingQuote_DurationMustMatchOption(t *testing.T) {
	const phone = "13901000003"
	r := setupRouter()
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	// seed 席位时长选项仅 {1,3,12} 月；2 不在选项内。
	w := doJSON(r, "POST", "/api/v1/billing/quote", token, map[string]interface{}{
		"biz_type":       "seat_new",
		"quantity":       1,
		"duration_value": 2,
	})
	require.Equal(t, http.StatusUnprocessableEntity, w.Code, "非法时长应 422: %s", w.Body.String())
	assert.Contains(t, decode(t, w).Message, "非法的购买时长")
}

// TestBillingQuote_RuntimePackTiers 覆盖 TC-10-004：临时时长包按分钟阶梯计价——
// 命中包预设的最优折扣档；低于最低购买时长 422「低于最低购买时长」。
func TestBillingQuote_RuntimePackTiers(t *testing.T) {
	const phone = "13901000004"
	r := setupRouter()
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	// 低于 min_minutes(60) → 422。
	wBelow := doJSON(r, "POST", "/api/v1/billing/quote", token, map[string]interface{}{
		"biz_type": "runtime_pack",
		"minutes":  59,
	})
	require.Equal(t, http.StatusUnprocessableEntity, wBelow.Code, "低于最低购买时长应 422: %s", wBelow.Body.String())
	assert.Contains(t, decode(t, wBelow).Message, "低于最低购买时长")

	runtimeQuote := func(minutes int) map[string]interface{} {
		w := doJSON(r, "POST", "/api/v1/billing/quote", token, map[string]interface{}{
			"biz_type": "runtime_pack",
			"minutes":  minutes,
		})
		require.Equal(t, http.StatusOK, w.Code, "minutes=%d 报价应成功: %s", minutes, w.Body.String())
		return decode(t, w).Data.(map[string]interface{})
	}

	// 达到最低但未命中任何包档：全价 10000bps，单价 20 分/分钟。
	base := runtimeQuote(60)
	assert.Equal(t, float64(60), base["quantity"], "quantity=minutes")
	assert.Equal(t, float64(20), base["unit_price_cents"], "seed: 每分钟单价 20 分")
	assert.Equal(t, float64(10000), base["qty_discount_bps"], "未命中任何包档应全价")
	assert.Equal(t, float64(1200), base["original_cents"], "60×20")
	assert.Equal(t, float64(1200), base["payable_cents"])

	// 命中 600 分钟档（10000bps，等同全价，但已进入折扣档位判定路径）。
	pack600 := runtimeQuote(600)
	assert.Equal(t, float64(10000), pack600["qty_discount_bps"])
	assert.Equal(t, float64(12000), pack600["original_cents"], "600×20")
	assert.Equal(t, float64(12000), pack600["payable_cents"])

	// 命中 3000 分钟档（9000bps）。
	pack3000 := runtimeQuote(3000)
	assert.Equal(t, float64(9000), pack3000["qty_discount_bps"], "命中 3000 分钟档应 9000bps")
	assert.Equal(t, float64(60000), pack3000["original_cents"], "3000×20")
	assert.Equal(t, float64(54000), pack3000["payable_cents"], "60000×9000/10000")
}

// TestBillingQuote_RechargeRejected 覆盖 TC-10-005：充值无需报价，422「充值无需报价」。
func TestBillingQuote_RechargeRejected(t *testing.T) {
	const phone = "13901000005"
	r := setupRouter()
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	w := doJSON(r, "POST", "/api/v1/billing/quote", token, map[string]interface{}{
		"biz_type": "recharge",
	})
	require.Equal(t, http.StatusUnprocessableEntity, w.Code, "充值报价应 422: %s", w.Body.String())
	assert.Contains(t, decode(t, w).Message, "充值无需报价")
}

// TestBillingQuote_GiftRuntimePreview_SeatNewAndRenew 覆盖 TC-10-090/091：
// 席位新购/续费报价均附带赠送时长预览 gift_runtime_minutes = per × quantity × duration_value；
// boot_slot 报价不返回赠送时长（该资源不参与赠送策略）。
func TestBillingQuote_GiftRuntimePreview_SeatNewAndRenew(t *testing.T) {
	const phone = "13901000006"
	r := setupRouter()
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	// TC-10-090：席位新购，quantity=3,duration_value=1。
	wNew := doJSON(r, "POST", "/api/v1/billing/quote", token, map[string]interface{}{
		"biz_type":       "seat_new",
		"quantity":       3,
		"duration_value": 1,
	})
	require.Equal(t, http.StatusOK, wNew.Code, "seat_new 报价应成功: %s", wNew.Body.String())
	dataNew := decode(t, wNew).Data.(map[string]interface{})
	assert.Equal(t, float64(600), dataNew["gift_runtime_minutes"], "200×3×1=600")

	// TC-10-091：席位续费，quantity 对应 len(unit_ids)=2,duration_value=1（quote 接口本身不校验
	// unit_ids 是否真实存在——报价阶段只按 kindOf(seat_renew)=seat 计算数量×时长赠送预览）。
	wRenew := doJSON(r, "POST", "/api/v1/billing/quote", token, map[string]interface{}{
		"biz_type":       "seat_renew",
		"quantity":       2,
		"duration_value": 1,
	})
	require.Equal(t, http.StatusOK, wRenew.Code, "seat_renew 报价应成功: %s", wRenew.Body.String())
	dataRenew := decode(t, wRenew).Data.(map[string]interface{})
	assert.Equal(t, float64(400), dataRenew["gift_runtime_minutes"], "200×2×1=400")

	// boot_slot：不参与赠送策略，响应始终携带 gift_runtime_minutes 字段（handler 用 gin.H 固定输出该键），
	// 但取值应为零值 0（BizOrderService.Quote 只在 kind==KindSeat 分支才计算 GiftRuntimeMinutes）。
	wBoot := doJSON(r, "POST", "/api/v1/billing/quote", token, map[string]interface{}{
		"biz_type":       "boot_slot_new",
		"quantity":       3,
		"duration_value": 7,
	})
	require.Equal(t, http.StatusOK, wBoot.Code, "boot_slot_new 报价应成功: %s", wBoot.Body.String())
	dataBoot := decode(t, wBoot).Data.(map[string]interface{})
	assert.Equal(t, float64(42000), dataBoot["original_cents"], "2000×3×7")
	require.Contains(t, dataBoot, "gift_runtime_minutes", "响应应始终携带该字段（即便值为 0）")
	assert.Equal(t, float64(0), dataBoot["gift_runtime_minutes"], "boot_slot 不赠送时长，应为 0")
}
