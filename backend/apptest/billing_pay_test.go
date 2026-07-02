package apptest

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 回归防护（审查报告 B1/B2）：订单支付的 markPaid 以 WHERE status<>paid 的 CAS 守卫置已付，
// pay/MarkPaid 均 CAS 先行——并发/重复回调下仅一个赢家履约，杜绝重复扣款 + 重复履约。
//
// 场景：前台用户先充值 → 创建 seat_new 余额订单（下单即扣款+履约一次并置已付）→
// 并发 + 重复多次调用后台 mark-paid（对已付订单幂等）→ 用 framework.DB 断言：
//   - billing_ledger_entries 中该订单 purchase 类流水仅 1 条（余额只扣一次，B1）
//   - billing_license_units 中该订单只生成 1 个席位授权单元（履约只发生一次，B2）
//   - billing_biz_orders 该订单 total_cents 只被扣一次对应的余额（用充值后余额差断言）。
//
// 唯一手机号 + t.Cleanup 精确清理自造数据，绝不截断共享表。
func TestBizOrderMarkPaidIdempotentHTTP(t *testing.T) {
	r := setupRouter()

	const phone = "13977700701" // 唯一手机号，避免与其他用例冲突
	token := registerUser(t, r, phone)
	adminTok := adminToken(t, r)

	// 由手机号解析该用户的自增 ID（apptest 不能 import user internal，走 framework.DB 原生查 users 表）。
	var userID uint
	require.NoError(t,
		framework.DB.Table("users").Where("phone = ?", phone).Select("id").Scan(&userID).Error)
	require.NotZero(t, userID, "应能查到刚注册用户的 ID")

	// 只清理本用例造的行（按 user_id / phone 精确删），不 CleanTable 共享表。
	t.Cleanup(func() {
		framework.DB.Exec("DELETE FROM billing_biz_order_items WHERE order_id IN (SELECT id FROM billing_biz_orders WHERE user_id = ?)", userID)
		framework.DB.Exec("DELETE FROM billing_biz_orders WHERE user_id = ?", userID)
		framework.DB.Exec("DELETE FROM billing_ledger_entries WHERE user_id = ?", userID)
		framework.DB.Exec("DELETE FROM billing_license_units WHERE user_id = ?", userID)
		framework.DB.Exec("DELETE FROM billing_accounts WHERE user_id = ?", userID)
		framework.DB.Exec("DELETE FROM users WHERE id = ?", userID)
	})

	// 1) 充值：桩直充，余额足够买 1 席位（seed 单价 3000 分/台/月，充 100000 分绰绰有余）。
	wTopup := doJSON(r, "POST", "/api/v1/billing/topup", token, map[string]interface{}{
		"amount_cents": 100000,
	})
	require.Equal(t, http.StatusOK, wTopup.Code, "充值失败: %s", wTopup.Body.String())

	// 记录充值后、下单前的余额（用于断言只扣一次）。
	var balBefore int64
	require.NoError(t,
		framework.DB.Table("billing_accounts").Where("user_id = ?", userID).Select("balance_cents").Scan(&balBefore).Error)
	require.Equal(t, int64(100000), balBefore)

	// 2) 创建 seat_new 余额订单：CreateOrder 恒调 pay()，余额购买即时扣款 + 履约一次并置已付。
	wCreate := doJSON(r, "POST", "/api/v1/billing/orders", token, map[string]interface{}{
		"biz_type":       "seat_new",
		"quantity":       1,
		"duration_value": 1,
		"pay_method":     "balance",
	})
	require.Equal(t, http.StatusOK, wCreate.Code, "下单失败: %s", wCreate.Body.String())
	createData := decode(t, wCreate).Data.(map[string]interface{})
	order := createData["order"].(map[string]interface{})
	orderID := int(order["id"].(float64))
	require.NotZero(t, orderID)
	assert.Equal(t, "paid", order["status"], "余额购买下单即置已付")
	totalCents := int64(order["total_cents"].(float64))
	require.Equal(t, int64(3000), totalCents, "seed: 席位 3000 分/台 × 1 台 × 1 月")

	// 履约一次后的基线：purchase 流水 1 条、席位单元 1 个、余额已扣 total。
	assert.Equal(t, int64(1), countPurchaseLedger(t, userID, orderID), "下单即扣款一次")
	assert.Equal(t, int64(1), countSeatUnits(t, userID, orderID), "下单即履约生成 1 个席位单元")

	// 3) 并发 + 重复多次 mark-paid（订单已 paid，CAS 命中 0 行、提前返回，均应幂等）。
	//    sqlite 下事务串行，但幂等语义仍必须成立。
	const workers = 8
	var wg sync.WaitGroup
	codes := make([]int, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			w := doJSON(r, "POST",
				fmt.Sprintf("/api/v1/admin/billing/biz-orders/%d/mark-paid", orderID), adminTok, nil)
			codes[idx] = w.Code
		}(i)
	}
	wg.Wait()
	for i, code := range codes {
		assert.Equal(t, http.StatusOK, code, "第 %d 次 mark-paid 应幂等成功返回 200", i)
	}

	// 4) 断言：重复回调后无重复履约、无重复扣款。
	assert.Equal(t, int64(1), countPurchaseLedger(t, userID, orderID),
		"重复 mark-paid 后 purchase 流水仍应仅 1 条（B1 不重复扣款）")
	assert.Equal(t, int64(1), countSeatUnits(t, userID, orderID),
		"重复 mark-paid 后席位单元仍应仅 1 个（B2 不重复履约）")

	var balAfter int64
	require.NoError(t,
		framework.DB.Table("billing_accounts").Where("user_id = ?", userID).Select("balance_cents").Scan(&balAfter).Error)
	assert.Equal(t, balBefore-totalCents, balAfter, "余额只应被扣一次 total_cents")
}

// countPurchaseLedger 统计某订单在统一流水中的 purchase 类记录数（每次履约扣款写一条）。
func countPurchaseLedger(t *testing.T, userID uint, orderID int) int64 {
	t.Helper()
	var n int64
	require.NoError(t, framework.DB.Table("billing_ledger_entries").
		Where("user_id = ? AND type = ? AND order_id = ?", userID, "purchase", orderID).
		Count(&n).Error)
	return n
}

// countSeatUnits 统计某订单履约生成的席位授权单元数（source_ref 由履约固化为 biz_order:<id>）。
func countSeatUnits(t *testing.T, userID uint, orderID int) int64 {
	t.Helper()
	var n int64
	require.NoError(t, framework.DB.Table("billing_license_units").
		Where("user_id = ? AND kind = ? AND source = ? AND source_ref = ?",
			userID, "seat", "order", fmt.Sprintf("biz_order:%d", orderID)).
		Count(&n).Error)
	return n
}
