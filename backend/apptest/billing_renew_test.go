package apptest

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"manager-backend/framework"
	"manager-backend/modules/billing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// billing_renew_test.go 覆盖 TC-10 计费文档「四、授权单元到期与续费」与「一、报价与下单」中的
// 续费与临时时长包用例点（任务 3，Plan B）：
//   - TC-10-008 续费按 unit_ids 数量计价、履约延长到期
//   - TC-10-030 继续支付未付订单（unpaid → pay → 扣款+履约+置 paid）
//   - TC-10-043 续费单位按 kind（seat 按月 AddDate(0,v,0)、boot_slot 按天 AddDate(0,0,v)）
//   - TC-10-044 续费越权拒绝（A 用 B 的 unit_id 续费 → 422「存在不属于当前用户的授权单元」）
//   - TC-10-046 续费列表 GET /billing/license-units 按 kind/keyword 过滤 + 属主隔离
//   - TC-10-009 临时时长包下单 → runtime_minute 钱包 +N 分钟
//
// 承接 Plan A：modules/billing/internal 的 fulfill_service_test.go / bizorder_service_test.go /
// api_handler_test.go 已在单元层覆盖 FulfillRenew 的日期计算、RuntimePack 加钱包、ListLicenseUnits
// 的 keyword 过滤等纯逻辑分支；本文件在接口层（HTTP + framework.DB 断言）补齐同等用例点，
// 确认 handler → service → repository 整条链路在真实路由/鉴权下的行为与文档预期一致，
// 且不重复 Plan A 已用桩覆盖的纯函数分支。
//
// 定价口径（seed 默认值，参考 pricingconfig_repository.go:defaultPricingConfig）：
// seat 单价 3000 分/台/月，数量阶梯 10 台起 9 折、100 台起 8 折；boot_slot 单价 2000 分/个/天，
// 数量阶梯 10 个起 9 折；时长选项不可自定义，seat 命中 1/3/12 月，boot_slot 命中 7/30 天。

// renewTopUp 前台余额充值（老接口 POST /billing/topup），供续费/时长包购买需要余额时使用。
func renewTopUp(t *testing.T, r *gin.Engine, token string, amountCents int64) {
	t.Helper()
	w := doJSON(r, "POST", "/api/v1/billing/topup", token, map[string]interface{}{
		"amount_cents": amountCents,
	})
	require.Equal(t, http.StatusOK, w.Code, "充值失败: %s", w.Body.String())
}

// renewListUnits 拉取 GET /billing/license-units（可选 kind/keyword），返回 items 切片。
func renewListUnits(t *testing.T, r *gin.Engine, token, kind, keyword string) []interface{} {
	t.Helper()
	path := "/api/v1/billing/license-units"
	sep := "?"
	if kind != "" {
		path += sep + "kind=" + kind
		sep = "&"
	}
	if keyword != "" {
		path += sep + "keyword=" + keyword
	}
	w := doJSON(r, "GET", path, token, nil)
	require.Equal(t, http.StatusOK, w.Code, "续费列表失败: %s", w.Body.String())
	data := decode(t, w).Data.(map[string]interface{})
	items, _ := data["items"].([]interface{})
	return items
}

// renewUnitIDs 从 renewListUnits 返回的 items 中提取全部 id（float64→uint）。
func renewUnitIDs(items []interface{}) []uint {
	out := make([]uint, 0, len(items))
	for _, it := range items {
		m := it.(map[string]interface{})
		out = append(out, uint(m["id"].(float64)))
	}
	return out
}

// renewBalanceCents 查某用户当前余额（分）。
func renewBalanceCents(t *testing.T, userID uint) int64 {
	t.Helper()
	var bal int64
	require.NoError(t, framework.DB.Table("billing_accounts").
		Where("user_id = ?", userID).Select("balance_cents").Scan(&bal).Error)
	return bal
}

// renewUnitExpireAt 查某授权单元当前 expire_at。
func renewUnitExpireAt(t *testing.T, unitID uint) time.Time {
	t.Helper()
	var expireAt time.Time
	require.NoError(t, framework.DB.Table("billing_license_units").
		Where("id = ?", unitID).Select("expire_at").Scan(&expireAt).Error)
	return expireAt
}

// TestBizOrder_SeatRenewHTTP 覆盖 TC-10-008：续费按 unit_ids 数量计价、履约延长到期。
// 用测试门面发放 2 个 seat 授权单元（30 月到期，充分未过期）→ 经 HTTP 续费列表取其 id →
// POST /orders {biz_type:seat_renew, unit_ids:[a,b], duration_value:1, pay_method:balance} →
// 断言 quantity=len(unit_ids)=2、total_cents=3000*2*1（无折扣，2 台不到 10 台阶梯）、
// 支付即时 paid，且两个单元的 expire_at 均较续费前延长约 1 个月。
func TestBizOrder_SeatRenewHTTP(t *testing.T) {
	r := setupRouter()
	const phone = "13903000001"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	require.NoError(t, billing.GrantSeatLicensesForTest(int(uid), 2))
	renewTopUp(t, r, token, 100000)

	units := renewListUnits(t, r, token, "seat", "")
	require.Len(t, units, 2, "续费前应能看到 2 个未过期 seat 单元")
	ids := renewUnitIDs(units)

	beforeA := renewUnitExpireAt(t, ids[0])
	beforeB := renewUnitExpireAt(t, ids[1])

	w := doJSON(r, "POST", "/api/v1/billing/orders", token, map[string]interface{}{
		"biz_type":       "seat_renew",
		"unit_ids":       []uint{ids[0], ids[1]},
		"duration_value": 1,
		"pay_method":     "balance",
	})
	require.Equal(t, http.StatusOK, w.Code, "续费下单失败: %s", w.Body.String())
	data := decode(t, w).Data.(map[string]interface{})
	order := data["order"].(map[string]interface{})
	pay := data["pay"].(map[string]interface{})

	assert.Equal(t, "seat_renew", order["biz_type"])
	assert.Equal(t, "paid", order["status"], "余额支付续费应即时置已付")
	assert.Equal(t, "paid", pay["status"])
	assert.Equal(t, float64(6000), order["total_cents"], "quantity=2 × unit_price=3000 × 1月，无折扣")

	afterA := renewUnitExpireAt(t, ids[0])
	afterB := renewUnitExpireAt(t, ids[1])
	assert.WithinDuration(t, beforeA.AddDate(0, 1, 0), afterA, 24*time.Hour, "单元 A 到期应延长约 1 个月")
	assert.WithinDuration(t, beforeB.AddDate(0, 1, 0), afterB, 24*time.Hour, "单元 B 到期应延长约 1 个月")
}

// TestBizOrder_PayUnpaidOrderHTTP 覆盖 TC-10-030：继续支付未付订单。
// 先在余额为 0 时经 HTTP 下单 seat_new（余额支付）：CreateOrder 的 repo.create 与 pay 分属两个
// 事务，扣款失败时订单已落库为 unpaid（不回滚订单创建，只回滚履约+扣款+置已付那一段）——
// 故下单请求本身返回 422「余额不足」，但订单行已生成。经 framework.DB 查出该未付订单 id，
// 充值后调用 POST /orders/:id/pay，断言走扣款+履约+置已付事务：返回 pay.status=paid，
// 订单状态、席位单元、余额三方一致。
func TestBizOrder_PayUnpaidOrderHTTP(t *testing.T) {
	r := setupRouter()
	const phone = "13903000002"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	// 余额为 0：seat_new 余额支付下单必然扣款失败，但订单已先行落库为 unpaid。
	wCreate := doJSON(r, "POST", "/api/v1/billing/orders", token, map[string]interface{}{
		"biz_type":       "seat_new",
		"quantity":       1,
		"duration_value": 1,
		"pay_method":     "balance",
	})
	require.Equal(t, http.StatusUnprocessableEntity, wCreate.Code, "余额不足应 422: %s", wCreate.Body.String())
	assert.Contains(t, decode(t, wCreate).Message, "余额不足")

	var orderID int
	require.NoError(t, framework.DB.Table("billing_biz_orders").
		Where("user_id = ? AND status = ?", uid, "unpaid").Select("id").Scan(&orderID).Error)
	require.NotZero(t, orderID, "扣款失败也应已落一张 unpaid 订单")

	// 此时不应有席位单元、余额不变（回滚了履约与扣款，但没有回滚订单创建）。
	var seatCount int64
	require.NoError(t, framework.DB.Table("billing_license_units").
		Where("user_id = ? AND kind = ?", uid, "seat").Count(&seatCount).Error)
	assert.Equal(t, int64(0), seatCount, "扣款失败不应履约生成席位")

	// 充值到位后继续支付该未付订单。
	renewTopUp(t, r, token, 100000)
	wPay := doJSON(r, "POST", fmt.Sprintf("/api/v1/billing/orders/%d/pay", orderID), token, nil)
	require.Equal(t, http.StatusOK, wPay.Code, "继续支付失败: %s", wPay.Body.String())
	payData := decode(t, wPay).Data.(map[string]interface{})
	pay := payData["pay"].(map[string]interface{})
	assert.Equal(t, "paid", pay["status"], "继续支付应扣款+履约+置已付")

	var status string
	require.NoError(t, framework.DB.Table("billing_biz_orders").
		Where("id = ?", orderID).Select("status").Scan(&status).Error)
	assert.Equal(t, "paid", status, "订单应落库为已付")

	require.NoError(t, framework.DB.Table("billing_license_units").
		Where("user_id = ? AND kind = ?", uid, "seat").Count(&seatCount).Error)
	assert.Equal(t, int64(1), seatCount, "继续支付应履约生成 1 个席位单元")

	assert.Equal(t, int64(100000-3000), renewBalanceCents(t, uid), "继续支付应按订单 total_cents 扣款")
}

// TestBizOrder_RenewDurationUnitByKindHTTP 覆盖 TC-10-043：续费单位按 kind——
// seat 按月叠加（AddDate(0,v,0)）、boot_slot 按天叠加（AddDate(0,0,v)），订单项 duration_unit
// 随之为 month / day。用测试门面分别发放 1 个 seat + 1 个 boot_slot，各自续费后核对
// 订单详情 items[0].duration_unit 与两者 expire_at 增量的数量级差异（月 vs 天）。
func TestBizOrder_RenewDurationUnitByKindHTTP(t *testing.T) {
	r := setupRouter()
	const phone = "13903000003"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	require.NoError(t, billing.GrantSeatLicensesForTest(int(uid), 1))
	require.NoError(t, billing.GrantBootSlotLicensesForTest(int(uid), 1))
	renewTopUp(t, r, token, 100000)

	seatUnits := renewListUnits(t, r, token, "seat", "")
	require.Len(t, seatUnits, 1)
	seatID := renewUnitIDs(seatUnits)[0]
	seatBefore := renewUnitExpireAt(t, seatID)

	bootUnits := renewListUnits(t, r, token, "boot_slot", "")
	require.Len(t, bootUnits, 1)
	bootID := renewUnitIDs(bootUnits)[0]
	bootBefore := renewUnitExpireAt(t, bootID)

	// seat 续费 1 月：命中时长选项 value=1。
	wSeat := doJSON(r, "POST", "/api/v1/billing/orders", token, map[string]interface{}{
		"biz_type":       "seat_renew",
		"unit_ids":       []uint{seatID},
		"duration_value": 1,
		"pay_method":     "balance",
	})
	require.Equal(t, http.StatusOK, wSeat.Code, "seat 续费失败: %s", wSeat.Body.String())
	seatOrder := decode(t, wSeat).Data.(map[string]interface{})["order"].(map[string]interface{})
	seatOrderID := int(seatOrder["id"].(float64))

	// boot_slot 续费 7 天：命中时长选项 value=7。
	wBoot := doJSON(r, "POST", "/api/v1/billing/orders", token, map[string]interface{}{
		"biz_type":       "boot_slot_renew",
		"unit_ids":       []uint{bootID},
		"duration_value": 7,
		"pay_method":     "balance",
	})
	require.Equal(t, http.StatusOK, wBoot.Code, "boot_slot 续费失败: %s", wBoot.Body.String())
	bootOrder := decode(t, wBoot).Data.(map[string]interface{})["order"].(map[string]interface{})
	bootOrderID := int(bootOrder["id"].(float64))

	// 订单详情 items[0].duration_unit：seat=month，boot_slot=day。
	wSeatDetail := doJSON(r, "GET", fmt.Sprintf("/api/v1/billing/orders/%d", seatOrderID), token, nil)
	require.Equal(t, http.StatusOK, wSeatDetail.Code)
	seatDetail := decode(t, wSeatDetail).Data.(map[string]interface{})
	seatItems := seatDetail["items"].([]interface{})
	require.Len(t, seatItems, 1)
	assert.Equal(t, "month", seatItems[0].(map[string]interface{})["duration_unit"], "seat 续费订单项单位应为 month")

	wBootDetail := doJSON(r, "GET", fmt.Sprintf("/api/v1/billing/orders/%d", bootOrderID), token, nil)
	require.Equal(t, http.StatusOK, wBootDetail.Code)
	bootDetail := decode(t, wBootDetail).Data.(map[string]interface{})
	bootItems := bootDetail["items"].([]interface{})
	require.Len(t, bootItems, 1)
	assert.Equal(t, "day", bootItems[0].(map[string]interface{})["duration_unit"], "boot_slot 续费订单项单位应为 day")

	// 到期延长量级核验：seat 按月（AddDate(0,1,0)），boot_slot 按天（AddDate(0,0,7)）。
	seatAfter := renewUnitExpireAt(t, seatID)
	bootAfter := renewUnitExpireAt(t, bootID)
	assert.WithinDuration(t, seatBefore.AddDate(0, 1, 0), seatAfter, 24*time.Hour, "seat 应按月叠加到期")
	assert.WithinDuration(t, bootBefore.AddDate(0, 0, 7), bootAfter, time.Hour, "boot_slot 应按天叠加到期")
}

// TestBizOrder_RenewForbidsOtherUsersUnitHTTP 覆盖 TC-10-044：续费越权拒绝——
// A 传入 B 的 unit_id 发起 seat_renew，getByIDs 按 (user_id, kind, id) 联合查询查不到该单元，
// 数量不匹配 → 422「存在不属于当前用户的授权单元」。断言具体状态码 + 特定文案，并核对
// B 的单元 expire_at 未被越权修改、A 未产生订单履约副作用。
func TestBizOrder_RenewForbidsOtherUsersUnitHTTP(t *testing.T) {
	r := setupRouter()
	const phoneA = "13903000004"
	const phoneB = "13903000005"
	tokenA := registerUser(t, r, phoneA)
	_ = registerUser(t, r, phoneB)
	uidA := userIDByPhone(t, phoneA)
	uidB := userIDByPhone(t, phoneB)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidA) })
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidB) })

	require.NoError(t, billing.GrantSeatLicensesForTest(int(uidB), 1))
	renewTopUp(t, r, tokenA, 100000)

	var bUnitID uint
	require.NoError(t, framework.DB.Table("billing_license_units").
		Where("user_id = ? AND kind = ?", uidB, "seat").Select("id").Scan(&bUnitID).Error)
	require.NotZero(t, bUnitID)
	bBefore := renewUnitExpireAt(t, bUnitID)

	w := doJSON(r, "POST", "/api/v1/billing/orders", tokenA, map[string]interface{}{
		"biz_type":       "seat_renew",
		"unit_ids":       []uint{bUnitID},
		"duration_value": 1,
		"pay_method":     "balance",
	})
	require.Equal(t, http.StatusUnprocessableEntity, w.Code, "越权续费应 422: %s", w.Body.String())
	assert.Contains(t, decode(t, w).Message, "存在不属于当前用户的授权单元")

	// B 的单元不应被越权延长；A 不应因这次失败下单留下已付订单/席位。
	bAfter := renewUnitExpireAt(t, bUnitID)
	assert.True(t, bAfter.Equal(bBefore), "越权续费不应修改 B 的单元到期")

	var aPaidCount int64
	require.NoError(t, framework.DB.Table("billing_biz_orders").
		Where("user_id = ? AND status = ?", uidA, "paid").Count(&aPaidCount).Error)
	assert.Equal(t, int64(0), aPaidCount, "越权续费不应产生 A 的已付订单")

	// A 余额应原样保留（越权在扣款前就 422 拒绝；即便进入事务也应原子回滚）。
	var aBalance int64
	require.NoError(t, framework.DB.Table("billing_accounts").
		Where("user_id = ?", uidA).Select("balance_cents").Scan(&aBalance).Error)
	assert.Equal(t, int64(100000), aBalance, "越权续费不应扣减 A 的余额")
}

// TestBizOrder_ListLicenseUnitsHTTP 覆盖 TC-10-046：续费列表 GET /billing/license-units
// 按 kind 过滤、按 keyword 模糊过滤（模糊字段为占用实例名/cpId，空闲单元 instance=null 不命中）、
// 且严格按属主隔离（B 看不到 A 的单元）。
func TestBizOrder_ListLicenseUnitsHTTP(t *testing.T) {
	r := setupRouter()
	const phoneA = "13903000006"
	const phoneB = "13903000007"
	tokenA := registerUser(t, r, phoneA)
	tokenB := registerUser(t, r, phoneB)
	uidA := userIDByPhone(t, phoneA)
	uidB := userIDByPhone(t, phoneB)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidA) })
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidB) })

	require.NoError(t, billing.GrantSeatLicensesForTest(int(uidA), 2))
	require.NoError(t, billing.GrantBootSlotLicensesForTest(int(uidA), 1))
	require.NoError(t, billing.GrantSeatLicensesForTest(int(uidB), 1))

	// kind 过滤：A 只查 seat，应恰好 2 条，且都不含 boot_slot。
	aSeats := renewListUnits(t, r, tokenA, "seat", "")
	require.Len(t, aSeats, 2, "A 应有 2 个 seat 单元")
	for _, it := range aSeats {
		assert.Equal(t, "seat", it.(map[string]interface{})["kind"])
	}

	aBoots := renewListUnits(t, r, tokenA, "boot_slot", "")
	require.Len(t, aBoots, 1, "A 应有 1 个 boot_slot 单元")

	// 属主隔离：B 只应看到自己的 1 个 seat 单元，看不到 A 的。
	bSeats := renewListUnits(t, r, tokenB, "seat", "")
	require.Len(t, bSeats, 1, "B 应只看到自己的 1 个 seat 单元")
	bIDs := renewUnitIDs(bSeats)
	aIDs := renewUnitIDs(aSeats)
	for _, bid := range bIDs {
		for _, aid := range aIDs {
			assert.NotEqual(t, aid, bid, "B 不应看到 A 的单元 id")
		}
	}

	// keyword 过滤：新发放的单元均空闲（current_instance_id=""，instance=nil），
	// filterUnitsByKeyword 的 hay 只由空字符串 + 无 instance 组成，任何非空关键字都不命中。
	aSeatsFiltered := renewListUnits(t, r, tokenA, "seat", "no-such-instance")
	assert.Len(t, aSeatsFiltered, 0, "空闲单元不应命中与实例名/cpId 无关的关键字")
}

// TestBizOrder_RuntimePackHTTP 覆盖 TC-10-009：临时时长包下单 → runtime_minute 钱包 +N 分钟，
// 订单项 target_kind=runtime_minute、quantity=minutes。用 600 分钟命中 seed 的第一档包（不打折），
// 单价 20 分/分钟 → total_cents=600*20=12000。支付后经 GET /billing/overview 核对
// runtime_minutes_remaining 增加对应分钟数，且订单详情 items[0] 字段与文档预期一致。
func TestBizOrder_RuntimePackHTTP(t *testing.T) {
	r := setupRouter()
	const phone = "13903000008"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	renewTopUp(t, r, token, 100000)

	overviewBefore := doJSON(r, "GET", "/api/v1/billing/overview", token, nil)
	require.Equal(t, http.StatusOK, overviewBefore.Code)
	minsBefore := int64(decode(t, overviewBefore).Data.(map[string]interface{})["runtime_minutes_remaining"].(float64))

	w := doJSON(r, "POST", "/api/v1/billing/orders", token, map[string]interface{}{
		"biz_type":   "runtime_pack",
		"minutes":    600,
		"pay_method": "balance",
	})
	require.Equal(t, http.StatusOK, w.Code, "时长包下单失败: %s", w.Body.String())
	data := decode(t, w).Data.(map[string]interface{})
	order := data["order"].(map[string]interface{})
	pay := data["pay"].(map[string]interface{})
	assert.Equal(t, "runtime_pack", order["biz_type"])
	assert.Equal(t, "paid", order["status"])
	assert.Equal(t, "paid", pay["status"])
	assert.Equal(t, float64(12000), order["total_cents"], "600 分钟命中 seed 首档、不打折：600*20=12000")

	orderID := int(order["id"].(float64))
	wDetail := doJSON(r, "GET", fmt.Sprintf("/api/v1/billing/orders/%d", orderID), token, nil)
	require.Equal(t, http.StatusOK, wDetail.Code)
	detail := decode(t, wDetail).Data.(map[string]interface{})
	items := detail["items"].([]interface{})
	require.Len(t, items, 1)
	item := items[0].(map[string]interface{})
	assert.Equal(t, "runtime_minute", item["target_kind"], "时长包订单项 target_kind 应为 runtime_minute")
	assert.Equal(t, float64(600), item["quantity"], "订单项 quantity 应等于购买分钟数")

	overviewAfter := doJSON(r, "GET", "/api/v1/billing/overview", token, nil)
	require.Equal(t, http.StatusOK, overviewAfter.Code)
	minsAfter := int64(decode(t, overviewAfter).Data.(map[string]interface{})["runtime_minutes_remaining"].(float64))
	assert.Equal(t, minsBefore+600, minsAfter, "购买时长包后余量应增加对应分钟数")
}
