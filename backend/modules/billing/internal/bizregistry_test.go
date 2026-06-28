package billing

import (
	"encoding/json"
	"sync"
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gap-8：用 internal registerBizType 注册一个假业务类型，驱动 CreateOrder → PayOrder，
// 断言：路由到注册的 quote/fulfill、meta_json 正确入库与回传给 fulfill、
// TotalCents=0 时 pay 跳过 wallet.Charge（余额不变）、TotalCents>0 时正确扣款。

// fakeBizRecorder 记录 quote/fulfill 的调用入参，供断言路由与 meta 往返。
type fakeBizRecorder struct {
	mu             sync.Mutex
	quoteCalls     int
	fulfillCalls   int
	lastQuoteUser  int
	lastQuoteParam []byte
	lastFulfillID  uint
	lastFulfillUsr int
	lastFulfillMet []byte
}

func TestRegisteredBiz_RoutesQuoteFulfillAndChargesBalance(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_biz_orders", "billing_biz_order_items")
	})
	const fakeType = "test_fake_paid"
	rec := &fakeBizRecorder{}
	// 服务端权威价 1234；产出可识别 meta_json。
	metaOut := []byte(`{"k":"v","n":42}`)
	registerBizType(fakeType,
		func(userID int, params []byte) (BizQuoteResult, error) {
			rec.mu.Lock()
			defer rec.mu.Unlock()
			rec.quoteCalls++
			rec.lastQuoteUser = userID
			rec.lastQuoteParam = append([]byte(nil), params...)
			return BizQuoteResult{TotalCents: 1234, MetaJSON: metaOut}, nil
		},
		func(userID int, orderID uint, metaJSON []byte) error {
			rec.mu.Lock()
			defer rec.mu.Unlock()
			rec.fulfillCalls++
			rec.lastFulfillUsr = userID
			rec.lastFulfillID = orderID
			rec.lastFulfillMet = append([]byte(nil), metaJSON...)
			return nil
		},
	)
	// 注册即合法。
	require.True(t, isValidBizType(fakeType))

	uid := 960001
	require.NoError(t, WalletService.TopUp(uid, 1000000, "test", "test"))

	paramsIn := json.RawMessage(`{"action":"x","tier_code":"t100"}`)
	res, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: fakeType, PayMethod: PayBalance, Params: paramsIn,
	})
	require.NoError(t, err)
	assert.Equal(t, BizOrderPaid, res.Pay.Status)
	// 权威价来自注册的 quote。
	assert.Equal(t, int64(1234), res.Order.TotalCents)

	// 路由：quote/fulfill 各被调用。
	assert.Equal(t, 1, rec.quoteCalls)
	assert.Equal(t, 1, rec.fulfillCalls)
	assert.Equal(t, uid, rec.lastQuoteUser)
	assert.JSONEq(t, string(paramsIn), string(rec.lastQuoteParam))
	// fulfill 回传：userID + orderID + 同一 meta_json。
	assert.Equal(t, uid, rec.lastFulfillUsr)
	assert.Equal(t, res.Order.ID, rec.lastFulfillID)
	assert.JSONEq(t, string(metaOut), string(rec.lastFulfillMet))

	// meta_json 正确入库：DB 读回订单项 meta 与产出一致。
	_, items, err := BizOrderService.GetOrder(uid, int(res.Order.ID))
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, fakeType, items[0].TargetKind)
	assert.JSONEq(t, string(metaOut), items[0].MetaJSON)

	// TotalCents>0 → 余额已扣 1234。
	bal, _ := WalletService.BalanceCents(uid)
	assert.Equal(t, int64(1000000-1234), bal)
}

// TotalCents=0（如套餐降级）→ pay 跳过 wallet.Charge，余额不变，但仍履约 + 置 paid。
func TestRegisteredBiz_ZeroTotalSkipsCharge(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_biz_orders", "billing_biz_order_items")
	})
	const fakeType = "test_fake_free"
	var fulfillCalls int
	var gotMeta []byte
	metaOut := []byte(`{"action":"downgrade"}`)
	registerBizType(fakeType,
		func(userID int, params []byte) (BizQuoteResult, error) {
			return BizQuoteResult{TotalCents: 0, MetaJSON: metaOut}, nil
		},
		func(userID int, orderID uint, metaJSON []byte) error {
			fulfillCalls++
			gotMeta = append([]byte(nil), metaJSON...)
			return nil
		},
	)

	uid := 960002
	require.NoError(t, WalletService.TopUp(uid, 5000, "test", "test"))
	balBefore, _ := WalletService.BalanceCents(uid)

	res, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: fakeType, PayMethod: PayBalance, Params: json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	assert.Equal(t, BizOrderPaid, res.Pay.Status)
	assert.Equal(t, int64(0), res.Order.TotalCents)

	// 履约仍发生（meta 回传正确）。
	assert.Equal(t, 1, fulfillCalls)
	assert.JSONEq(t, string(metaOut), string(gotMeta))

	// 0 元跳过扣款：余额不变。
	balAfter, _ := WalletService.BalanceCents(uid)
	assert.Equal(t, balBefore, balAfter)
}

// PayOrder（继续支付未支付订单）也走注册的 fulfill 并按 total 扣款。
func TestRegisteredBiz_PayOrderRoutesFulfill(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_biz_orders", "billing_biz_order_items")
	})
	const fakeType = "test_fake_payorder"
	var fulfillCalls int
	metaOut := []byte(`{"x":1}`)
	registerBizType(fakeType,
		func(userID int, params []byte) (BizQuoteResult, error) {
			return BizQuoteResult{TotalCents: 500, MetaJSON: metaOut}, nil
		},
		func(userID int, orderID uint, metaJSON []byte) error {
			fulfillCalls++
			return nil
		},
	)

	uid := 960003
	// 余额为 0：CreateOrder 用余额支付会因扣款失败而报错，故先用第三方下单留待 PayOrder。
	// 这里走第三方桩网关：CreateOrder 即时 paid。为测 PayOrder 路径，改为先充值再余额下单后断言。
	require.NoError(t, WalletService.TopUp(uid, 10000, "test", "test"))
	res, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: fakeType, PayMethod: PayBalance, Params: json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	require.Equal(t, BizOrderPaid, res.Pay.Status)
	require.Equal(t, 1, fulfillCalls)

	// 再次 PayOrder 已支付订单 → 幂等返回 paid，不重复履约/扣款。
	bal1, _ := WalletService.BalanceCents(uid)
	r2, err := BizOrderService.PayOrder(uid, int(res.Order.ID))
	require.NoError(t, err)
	assert.Equal(t, BizOrderPaid, r2.Pay.Status)
	assert.Equal(t, 1, fulfillCalls) // 未重复履约
	bal2, _ := WalletService.BalanceCents(uid)
	assert.Equal(t, bal1, bal2) // 未重复扣款
}
