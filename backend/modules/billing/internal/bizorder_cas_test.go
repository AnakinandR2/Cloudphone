package billing

import (
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 回归测试：订单支付 CAS 幂等守卫（审查报告 B1/B2）。
// markPaid 以 WHERE status<>paid 做状态守卫并返回受影响行数，
// pay/MarkPaid CAS 先行——并发/重复回调下仅一个赢家履约，杜绝重复扣款+重复履约。

// TestMarkPaidRepo_CASGuardIsIdempotent 直接验证仓储层 CAS：首次命中 1 行，再次命中 0 行。
func TestMarkPaidRepo_CASGuardIsIdempotent(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_biz_orders") })
	repo := newBizOrderRepository(framework.DB)
	o := &BizOrder{
		UserID:     960001,
		BizType:    BizRecharge,
		Status:     BizOrderUnpaid,
		TotalCents: 10000,
		PayMethod:  PayBalance,
	}
	require.NoError(t, repo.create(o, nil))

	rows1, err := repo.markPaid(int(o.ID))
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows1, "首次 CAS 应命中 1 行(unpaid→paid)")

	rows2, err := repo.markPaid(int(o.ID))
	require.NoError(t, err)
	assert.Equal(t, int64(0), rows2, "重复 CAS 应命中 0 行(已付)，据此跳过重复履约")
}

// TestMarkPaidService_RechargeFulfilledOnce 验证重复回调不重复履约（充值只入账一次）。
func TestMarkPaidService_RechargeFulfilledOnce(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_biz_orders")
		framework.CleanTable("billing_biz_order_items")
	})
	uid := 960002
	repo := newBizOrderRepository(framework.DB)
	o := &BizOrder{
		UserID:     uint(uid),
		BizType:    BizRecharge,
		Status:     BizOrderUnpaid,
		TotalCents: 5000,
		PayMethod:  PayBalance,
	}
	require.NoError(t, repo.create(o, nil))

	before, err := WalletService.BalanceCents(uid)
	require.NoError(t, err)

	_, err = BizOrderService.MarkPaid(int(o.ID))
	require.NoError(t, err)
	after1, err := WalletService.BalanceCents(uid)
	require.NoError(t, err)
	assert.Equal(t, before+5000, after1, "首次 MarkPaid 应充值一次")

	// 模拟网关重试的重复回调：订单已 paid，不得重复入账。
	_, err = BizOrderService.MarkPaid(int(o.ID))
	require.NoError(t, err)
	after2, err := WalletService.BalanceCents(uid)
	require.NoError(t, err)
	assert.Equal(t, after1, after2, "重复 MarkPaid 不应重复充值(B2 幂等)")
}

// TestPay_ChargeFailureLeavesOrderUnpaid 验证扣款失败时订单回滚为未支付、余额不被扣（可重试）——
// CAS 与扣款同事务，杜绝"已置已付却未扣款"（对抗复核 C2 回归）。
func TestPay_ChargeFailureLeavesOrderUnpaid(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_biz_orders")
		framework.CleanTable("billing_biz_order_items")
		framework.CleanTable("billing_license_units")
	})
	uid := 960003 // 不 TopUp：余额为 0，买席位必然扣款失败

	_, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: BizSeatNew, Quantity: 1, DurationValue: 1, PayMethod: PayBalance,
	})
	require.Error(t, err, "余额不足应支付失败")

	bal, err := WalletService.BalanceCents(uid)
	require.NoError(t, err)
	assert.Equal(t, int64(0), bal, "扣款失败余额不应变动")

	_, paidTotal, err := BizOrderService.ListOrders(uid, 1, 20, BizOrderPaid, "", time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Equal(t, int64(0), paidTotal, "不应留下已支付订单（应回滚为未支付，可重试）")
}
