package billing

import (
	"encoding/json"
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setPaymentMethodsWithFee 临时把支付方式配置改为带手续费（balance fee=0；wechat/alipay 2%+¥1），
// 用例结束还原默认，避免污染共享单例。
func setPaymentMethodsWithFee(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { _ = PricingConfigService.Save(defaultPricingConfig()) })
	_, err := PricingConfigService.SavePartial(func(d *PricingConfigData) {
		d.PaymentMethods = []PaymentMethod{
			{Code: PayBalance, Name: "余额支付", Enabled: true, Sort: 0},
			{Code: PayWechat, Name: "微信支付", Enabled: true, Sort: 1, FeePercentBps: 200, FeeFixedCents: 100},
			{Code: PayAlipay, Name: "支付宝", Enabled: true, Sort: 2, FeePercentBps: 200, FeeFixedCents: 100},
		}
	})
	require.NoError(t, err)
}

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

func TestBizOrder_ThirdPartyAutoSettles(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_biz_orders")
		framework.CleanTable("billing_biz_order_items")
		framework.CleanTable("billing_license_units")
	})
	uid := 950002
	// 桩网关：无真实第三方对接，下单即视为支付成功 → 即时履约、置 paid（不扣余额）。
	res, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: BizSeatNew, Quantity: 1, DurationValue: 1, PayMethod: PayWechat,
	})
	require.NoError(t, err)
	assert.Equal(t, BizOrderPaid, res.Pay.Status)
	cap, _ := LicenseService.Capacity(uid, KindSeat)
	assert.Equal(t, 1, cap)
}

func TestBizOrder_ThirdPartyRechargeCreditsBalance(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_biz_orders")
		framework.CleanTable("billing_biz_order_items")
	})
	uid := 950007
	// 第三方充值：桩网关即时到账 → 订单 paid 且余额增加（不来自余额扣款）。
	res, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: BizRecharge, AmountCents: 50000, PayMethod: PayAlipay,
	})
	require.NoError(t, err)
	assert.Equal(t, BizOrderPaid, res.Pay.Status)
	bal, _ := WalletService.BalanceCents(uid)
	assert.Equal(t, int64(50000), bal)
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

// ---- 手续费固化（§4） ----

// 第三方新购：fee 字段在入库前固化（DB 读回非 0 且正确）；2% + ¥1 外加。
func TestBizOrder_FeeFixedOnCreate_ThirdPartySeatNew(t *testing.T) {
	setPaymentMethodsWithFee(t)
	t.Cleanup(func() {
		framework.CleanTable("billing_biz_orders", "billing_biz_order_items", "billing_license_units")
	})
	uid := 950010
	res, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: BizSeatNew, Quantity: 10, DurationValue: 12, PayMethod: PayWechat,
	})
	require.NoError(t, err)
	// 3000×10×12×0.9×0.7 = 226800；fee = 2% + ¥1 = 4536 + 100 = 4636。
	assert.Equal(t, int64(226800), res.Order.TotalCents)
	assert.Equal(t, int64(4636), res.Order.FeeCents)
	assert.Equal(t, 200, res.Order.FeePercentBps)
	assert.Equal(t, int64(100), res.Order.FeeFixedCents)

	// DB 读回：fee 三字段确已落库（非 0、正确）。
	o, _, err := BizOrderService.GetOrder(uid, int(res.Order.ID))
	require.NoError(t, err)
	assert.Equal(t, int64(4636), o.FeeCents)
	assert.Equal(t, 200, o.FeePercentBps)
	assert.Equal(t, int64(100), o.FeeFixedCents)
}

// 第三方充值：基数 = 面额；fee 固化但不进余额（外加语义）。
func TestBizOrder_FeeFixedOnCreate_ThirdPartyRecharge(t *testing.T) {
	setPaymentMethodsWithFee(t)
	t.Cleanup(func() {
		framework.CleanTable("billing_biz_orders", "billing_biz_order_items")
	})
	uid := 950011
	res, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: BizRecharge, AmountCents: 100000, PayMethod: PayAlipay,
	})
	require.NoError(t, err)
	// fee = 100000×2% + ¥1 = 2000 + 100 = 2100。
	assert.Equal(t, int64(100000), res.Order.TotalCents)
	assert.Equal(t, int64(2100), res.Order.FeeCents)

	o, _, err := BizOrderService.GetOrder(uid, int(res.Order.ID))
	require.NoError(t, err)
	assert.Equal(t, int64(2100), o.FeeCents)

	// 到账余额 = 面额（手续费外加、不进余额）。
	bal, _ := WalletService.BalanceCents(uid)
	assert.Equal(t, int64(100000), bal)
}

// 第三方续费：fee 基数 = 折后应付。
func TestBizOrder_FeeFixedOnCreate_ThirdPartyRenew(t *testing.T) {
	setPaymentMethodsWithFee(t)
	t.Cleanup(func() {
		framework.CleanTable("billing_biz_orders", "billing_biz_order_items", "billing_license_units")
	})
	uid := 950012
	// 先用第三方买 1 个 seat（1 月）。
	_, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{BizType: BizSeatNew, Quantity: 1, DurationValue: 1, PayMethod: PayWechat})
	require.NoError(t, err)
	units, err := LicenseService.repo.listByUserKind(uid, KindSeat)
	require.NoError(t, err)
	require.Len(t, units, 1)

	res, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: BizSeatRenew, UnitIDs: []uint{units[0].ID}, DurationValue: 3, PayMethod: PayWechat,
	})
	require.NoError(t, err)
	// 续费 1 个席位 3 月：3000×1×3×0.85 = 7650；fee = 153 + 100 = 253。
	assert.Equal(t, int64(7650), res.Order.TotalCents)
	assert.Equal(t, int64(253), res.Order.FeeCents)
}

// 余额支付：fee 恒 0（balance 配置为 0），实付扣款 = total（+0），行为不变。
func TestBizOrder_BalancePayFeeIsZero(t *testing.T) {
	setPaymentMethodsWithFee(t)
	t.Cleanup(func() {
		framework.CleanTable("billing_biz_orders", "billing_biz_order_items", "billing_license_units")
	})
	uid := 950013
	require.NoError(t, WalletService.TopUp(uid, 1000000, "test", "test"))
	res, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: BizSeatNew, Quantity: 10, DurationValue: 12, PayMethod: PayBalance,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(226800), res.Order.TotalCents)
	assert.Equal(t, int64(0), res.Order.FeeCents)
	// 扣款 = total + fee = total + 0。
	bal, _ := WalletService.BalanceCents(uid)
	assert.Equal(t, int64(1000000-226800), bal)
}

// 实付扣款 = total + fee：当余额方式被异常配置了非 0 fee（双保险），仍按 total+fee 扣款。
func TestBizOrder_BalancePayChargesTotalPlusFee(t *testing.T) {
	t.Cleanup(func() {
		_ = PricingConfigService.Save(defaultPricingConfig())
		framework.CleanTable("billing_biz_orders", "billing_biz_order_items", "billing_license_units")
	})
	// 故意给 balance 配 fee（绕过 API 校验、直接存配置）以验证扣款行=total+fee。
	_, err := PricingConfigService.SavePartial(func(d *PricingConfigData) {
		d.PaymentMethods = []PaymentMethod{
			{Code: PayBalance, Name: "余额支付", Enabled: true, Sort: 0, FeePercentBps: 200, FeeFixedCents: 100},
		}
	})
	require.NoError(t, err)
	uid := 950014
	require.NoError(t, WalletService.TopUp(uid, 1000000, "test", "test"))
	res, err := BizOrderService.CreateOrder(uid, &BizOrderCreate{
		BizType: BizSeatNew, Quantity: 1, DurationValue: 1, PayMethod: PayBalance,
	})
	require.NoError(t, err)
	// total = 3000×1×1×1 = 3000；fee = 60 + 100 = 160。
	assert.Equal(t, int64(3000), res.Order.TotalCents)
	assert.Equal(t, int64(160), res.Order.FeeCents)
	bal, _ := WalletService.BalanceCents(uid)
	assert.Equal(t, int64(1000000-3000-160), bal)
}

// §4.3：下单选了被禁用的支付方式 → 拒绝。
func TestBizOrder_RejectsDisabledPayMethod(t *testing.T) {
	t.Cleanup(func() { _ = PricingConfigService.Save(defaultPricingConfig()) })
	_, err := PricingConfigService.SavePartial(func(d *PricingConfigData) {
		d.PaymentMethods = []PaymentMethod{
			{Code: PayBalance, Name: "余额支付", Enabled: true, Sort: 0},
			{Code: PayWechat, Name: "微信支付", Enabled: false, Sort: 1},
			{Code: PayAlipay, Name: "支付宝", Enabled: true, Sort: 2},
		}
	})
	require.NoError(t, err)
	_, err = BizOrderService.CreateOrder(950015, &BizOrderCreate{
		BizType: BizSeatNew, Quantity: 1, DurationValue: 1, PayMethod: PayWechat,
	})
	assert.Error(t, err)
}

// 向后兼容：缺 fee 字段的旧 PaymentMethod JSON 解码为 0。
func TestPaymentMethod_LegacyJSONDecodesFeeZero(t *testing.T) {
	var pm PaymentMethod
	require.NoError(t, json.Unmarshal([]byte(`{"code":"wechat","name":"微信","enabled":true,"sort":1}`), &pm))
	assert.Equal(t, 0, pm.FeePercentBps)
	assert.Equal(t, int64(0), pm.FeeFixedCents)
}

// 向后兼容：缺 fee 字段的旧 BizOrder JSON 解码为 0。
func TestBizOrder_LegacyJSONDecodesFeeZero(t *testing.T) {
	var o BizOrder
	require.NoError(t, json.Unmarshal([]byte(`{"id":1,"biz_type":"seat_new","status":"paid","total_cents":3000,"pay_method":"balance"}`), &o))
	assert.Equal(t, int64(0), o.FeeCents)
	assert.Equal(t, 0, o.FeePercentBps)
	assert.Equal(t, int64(0), o.FeeFixedCents)
}
