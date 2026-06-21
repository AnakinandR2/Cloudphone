package billing

import (
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBizOrder_BalancePaySeatNew_FulfillsAndChargesBalance(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_license_units")
		framework.CleanTable("billing_biz_orders")
		framework.CleanTable("billing_biz_order_items")
	})
	uid := 950001
	// 充值足够余额。
	require.NoError(t, WalletService.TopUp(uid, 1000000, "test", "test"))

	res, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: BizSeatNew, Quantity: 10, DurationValue: 12, PayMethod: PayBalance,
	})
	require.NoError(t, err)
	assert.Equal(t, BizOrderPaid, res.Pay.Status)
	// 3000×10×12×0.9×0.7 = 226800
	assert.Equal(t, int64(226800), res.Order.TotalCents)

	// 履约：10 个 seat 单元。
	cap, err := LicenseService.Capacity(uid, KindSeat)
	require.NoError(t, err)
	assert.Equal(t, 10, cap)

	// 余额已扣。
	bal, _ := WalletService.BalanceCents(uid)
	assert.Equal(t, int64(1000000-226800), bal)
}

func TestBizOrder_ThirdPartyPending(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_biz_orders")
		framework.CleanTable("billing_biz_order_items")
		framework.CleanTable("billing_license_units")
	})
	uid := 950002
	res, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: BizSeatNew, Quantity: 1, DurationValue: 1, PayMethod: PayWechat,
	})
	require.NoError(t, err)
	assert.Equal(t, "pending", res.Pay.Status)
	// 未履约。
	cap, _ := LicenseService.Capacity(uid, KindSeat)
	assert.Equal(t, 0, cap)

	// 继续支付（桩：第三方仍 pending，需走 mark-paid 回调）。
	o, err := BizOrderService.MarkPaid(int(res.Order.ID))
	require.NoError(t, err)
	assert.Equal(t, BizOrderPaid, o.Status)
	cap, _ = LicenseService.Capacity(uid, KindSeat)
	assert.Equal(t, 1, cap)
}

func TestBizOrder_RenewAccumulatesExpiry(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_license_units")
		framework.CleanTable("billing_biz_orders")
		framework.CleanTable("billing_biz_order_items")
	})
	uid := 950005
	require.NoError(t, WalletService.TopUp(uid, 1000000, "test", "test"))
	// 先买 1 个 seat（1 月）。
	_, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{BizType: BizSeatNew, Quantity: 1, DurationValue: 1, PayMethod: PayBalance})
	require.NoError(t, err)
	units, err := LicenseService.repo.listByUserKind(uid, KindSeat)
	require.NoError(t, err)
	require.Len(t, units, 1)
	before := units[0].ExpireAt

	// 续费 3 月。
	res, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: BizSeatRenew, UnitIDs: []uint{units[0].ID}, DurationValue: 3, PayMethod: PayBalance,
	})
	require.NoError(t, err)
	assert.Equal(t, BizOrderPaid, res.Pay.Status)

	after, err := LicenseService.repo.getByIDs(uid, []uint{units[0].ID}, KindSeat)
	require.NoError(t, err)
	assert.WithinDuration(t, before.AddDate(0, 3, 0), after[0].ExpireAt, 24*time.Hour)
}

func TestBizOrder_RechargeRejectsBalancePay(t *testing.T) {
	uid := 950003
	_, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: BizRecharge, AmountCents: 10000, PayMethod: PayBalance,
	})
	assert.Error(t, err)
}

func TestBizOrder_RuntimePackBalancePay(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_biz_orders")
		framework.CleanTable("billing_biz_order_items")
		framework.CleanTable("billing_runtime_minute_wallets")
	})
	uid := 950004
	require.NoError(t, WalletService.TopUp(uid, 1000000, "test", "test"))
	res, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: BizRuntimePack, Minutes: 600, PayMethod: PayBalance,
	})
	require.NoError(t, err)
	assert.Equal(t, BizOrderPaid, res.Pay.Status)
	rem, _ := RuntimeWalletService.Remaining(uid)
	assert.Equal(t, int64(600), rem)
}
