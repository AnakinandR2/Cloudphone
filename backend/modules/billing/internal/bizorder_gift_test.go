package billing

import (
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setSeatGift 显式设置每席位每月赠送分钟数，使测试独立于默认/seed 值。
func setSeatGift(t *testing.T, minutes int) {
	t.Helper()
	_, err := PricingConfigService.SavePartial(func(d *PricingConfigData) {
		rt := d.Runtime
		rt.GiftMinutesPerSeatMonth = minutes
		d.Runtime = rt
	})
	require.NoError(t, err)
}

func cleanGiftTables() {
	framework.CleanTable("billing_license_units")
	framework.CleanTable("billing_biz_orders")
	framework.CleanTable("billing_biz_order_items")
	framework.CleanTable("billing_runtime_minute_wallets")
}

func TestBizOrder_SeatNew_GiftsRuntimeMinutes(t *testing.T) {
	t.Cleanup(cleanGiftTables)
	setSeatGift(t, 200)
	uid := 950101
	require.NoError(t, WalletService.TopUp(uid, 10_000_000, "test", "test"))

	res, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: BizSeatNew, Quantity: 2, DurationValue: 3, PayMethod: PayBalance,
	})
	require.NoError(t, err)
	require.Equal(t, BizOrderPaid, res.Pay.Status)

	// 赠送 = 200 × 2 席位 × 3 月 = 1200 分钟。
	rem, _ := RuntimeWalletService.Remaining(uid)
	assert.Equal(t, int64(1200), rem)
	assert.Equal(t, 1200, res.Order.GiftRuntimeMinutes)
}

func TestBizOrder_SeatRenew_GiftsRuntimeMinutes(t *testing.T) {
	t.Cleanup(cleanGiftTables)
	setSeatGift(t, 200)
	uid := 950102
	require.NoError(t, WalletService.TopUp(uid, 10_000_000, "test", "test"))

	// 先买 1 席（赠 200×1×1=200），再续费 3 月（赠 200×1×3=600），合计 800。
	_, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{BizType: BizSeatNew, Quantity: 1, DurationValue: 1, PayMethod: PayBalance})
	require.NoError(t, err)
	units, err := LicenseService.repo.listByUserKind(uid, KindSeat)
	require.NoError(t, err)
	require.Len(t, units, 1)

	res, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: BizSeatRenew, UnitIDs: []uint{units[0].ID}, DurationValue: 3, PayMethod: PayBalance,
	})
	require.NoError(t, err)
	assert.Equal(t, 600, res.Order.GiftRuntimeMinutes)

	rem, _ := RuntimeWalletService.Remaining(uid)
	assert.Equal(t, int64(800), rem)
}

func TestBizOrder_GiftDisabled_NoGrant(t *testing.T) {
	t.Cleanup(cleanGiftTables)
	setSeatGift(t, 0)
	uid := 950103
	require.NoError(t, WalletService.TopUp(uid, 10_000_000, "test", "test"))

	res, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: BizSeatNew, Quantity: 5, DurationValue: 12, PayMethod: PayBalance,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, res.Order.GiftRuntimeMinutes)
	rem, _ := RuntimeWalletService.Remaining(uid)
	assert.Equal(t, int64(0), rem)
}

func TestBizOrder_BootSlotNew_NoGift(t *testing.T) {
	t.Cleanup(cleanGiftTables)
	setSeatGift(t, 200)
	uid := 950104
	require.NoError(t, WalletService.TopUp(uid, 10_000_000, "test", "test"))

	res, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: BizBootSlotNew, Quantity: 2, DurationValue: 30, PayMethod: PayBalance,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, res.Order.GiftRuntimeMinutes)
	rem, _ := RuntimeWalletService.Remaining(uid)
	assert.Equal(t, int64(0), rem)
}

func TestBizOrder_QuoteSeat_IncludesGift(t *testing.T) {
	setSeatGift(t, 200)
	q, err := BizOrderService.Quote(0, &BizQuoteRequest{BizType: BizSeatNew, Quantity: 2, DurationValue: 3})
	require.NoError(t, err)
	assert.Equal(t, 1200, q.GiftRuntimeMinutes)

	// 包月开机数不赠送。
	q2, err := BizOrderService.Quote(0, &BizQuoteRequest{BizType: BizBootSlotNew, Quantity: 2, DurationValue: 7})
	require.NoError(t, err)
	assert.Equal(t, 0, q2.GiftRuntimeMinutes)
}

func TestBizOrder_ListOrders_ItemsGiftAndFilters(t *testing.T) {
	t.Cleanup(cleanGiftTables)
	setSeatGift(t, 200)
	uid := 950105
	require.NoError(t, WalletService.TopUp(uid, 10_000_000, "test", "test"))
	_, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{BizType: BizSeatNew, Quantity: 2, DurationValue: 3, PayMethod: PayBalance})
	require.NoError(t, err)

	// 随单返回明细 + 赠送量。
	list, total, err := BizOrderService.ListOrders(uid, 1, 20, "", "", time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	require.Len(t, list[0].Items, 1)
	assert.Equal(t, 2, list[0].Items[0].Quantity)
	assert.Equal(t, 3, list[0].Items[0].DurationValue)
	assert.Equal(t, 1200, list[0].GiftRuntimeMinutes)

	// 状态过滤：无 unpaid。
	_, t2, err := BizOrderService.ListOrders(uid, 1, 20, BizOrderUnpaid, "", time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Equal(t, int64(0), t2)

	// 时间过滤：from 在未来 → 排除。
	_, t3, err := BizOrderService.ListOrders(uid, 1, 20, "", "", time.Now().Add(time.Hour), time.Time{})
	require.NoError(t, err)
	assert.Equal(t, int64(0), t3)
}
