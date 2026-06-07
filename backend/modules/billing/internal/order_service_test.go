package billing

import (
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ordUser = 9101

func TestCreateOrderSnapshotsAndTotal(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_orders", "billing_order_items", "billing_skus", "billing_discount_tiers")
	})
	require.NoError(t, SeedCatalog(framework.DB))

	d, err := OrderService.CreateOrder(ordUser, &OrderCreate{
		PayMethod: PayBalance,
		Items: []OrderItemRequest{
			{SkuCode: "instance_fee", CycleMonths: 12, Quantity: 5},
			{SkuCode: "time_pack", CycleMonths: 0, Quantity: 1000},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, OrderPending, d.Order.Status)
	assert.Equal(t, int64(142000), d.Order.TotalCents)
	require.Len(t, d.Items, 2)
	assert.Equal(t, int64(126000), d.Items[0].PayableCents)
	assert.Equal(t, "云手机实例费", d.Items[0].SkuName)
	assert.NotEmpty(t, d.Order.OrderNo)

	got, err := OrderService.GetOrder(ordUser, int(d.Order.ID))
	require.NoError(t, err)
	assert.Equal(t, d.Order.OrderNo, got.Order.OrderNo)
	_, err = OrderService.GetOrder(ordUser+1, int(d.Order.ID))
	assert.Error(t, err)
}

func TestCreateOrderGuards(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_orders", "billing_order_items", "billing_skus", "billing_discount_tiers")
	})
	require.NoError(t, SeedCatalog(framework.DB))

	_, err := OrderService.CreateOrder(ordUser, &OrderCreate{PayMethod: "btc", Items: []OrderItemRequest{{SkuCode: "instance_fee", CycleMonths: 1, Quantity: 1}}})
	assert.Error(t, err)
	_, err = OrderService.CreateOrder(ordUser, &OrderCreate{PayMethod: PayBalance, Items: []OrderItemRequest{{SkuCode: "nope", CycleMonths: 1, Quantity: 1}}})
	assert.Error(t, err)
}

func TestPayWithBalanceFulfills(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_orders", "billing_order_items", "billing_skus", "billing_discount_tiers",
			"billing_accounts", "billing_ledger_entries", "billing_entitlement_batches")
	})
	require.NoError(t, SeedCatalog(framework.DB))

	_, err := BillingService.Topup(ordUser, 1000000, "充值", "user:9101")
	require.NoError(t, err)

	d, err := OrderService.CreateOrder(ordUser, &OrderCreate{
		PayMethod: PayBalance,
		Items: []OrderItemRequest{
			{SkuCode: "instance_fee", CycleMonths: 1, Quantity: 2},
			{SkuCode: "time_pack", CycleMonths: 0, Quantity: 1000},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(22000), d.Order.TotalCents)

	paid, err := OrderService.PayWithBalance(ordUser, int(d.Order.ID))
	require.NoError(t, err)
	assert.Equal(t, OrderPaid, paid.Order.Status)
	require.NotNil(t, paid.Order.PaidAt)

	acc, err := BillingService.GetAccount(ordUser)
	require.NoError(t, err)
	assert.Equal(t, int64(1000000-22000), acc.BalanceCents)

	snap, err := EntitlementService.Capacities(ordUser)
	require.NoError(t, err)
	assert.Equal(t, int64(2), snap.InstanceSeat)
	assert.Equal(t, int64(60000), snap.RuntimeMinute)

	_, err = OrderService.PayWithBalance(ordUser, int(d.Order.ID))
	assert.Error(t, err)
	snap, _ = EntitlementService.Capacities(ordUser)
	assert.Equal(t, int64(2), snap.InstanceSeat)
}

func TestPayWithBalanceInsufficient(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_orders", "billing_order_items", "billing_skus", "billing_discount_tiers",
			"billing_accounts", "billing_ledger_entries", "billing_entitlement_batches")
	})
	require.NoError(t, SeedCatalog(framework.DB))

	d, err := OrderService.CreateOrder(ordUser, &OrderCreate{PayMethod: PayBalance,
		Items: []OrderItemRequest{{SkuCode: "instance_fee", CycleMonths: 1, Quantity: 2}}})
	require.NoError(t, err)

	_, err = OrderService.PayWithBalance(ordUser, int(d.Order.ID))
	assert.Error(t, err)

	got, _ := OrderService.GetOrder(ordUser, int(d.Order.ID))
	assert.Equal(t, OrderPending, got.Order.Status)
	snap, _ := EntitlementService.Capacities(ordUser)
	assert.Equal(t, int64(0), snap.InstanceSeat)
}

func TestMarkPaidFulfillsWithoutBalance(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_orders", "billing_order_items", "billing_skus", "billing_discount_tiers",
			"billing_accounts", "billing_ledger_entries", "billing_entitlement_batches")
	})
	require.NoError(t, SeedCatalog(framework.DB))

	d, err := OrderService.CreateOrder(ordUser, &OrderCreate{PayMethod: PayWechat,
		Items: []OrderItemRequest{{SkuCode: "boot_pack", CycleMonths: 3, Quantity: 4}}})
	require.NoError(t, err)

	paid, err := OrderService.MarkPaid(int(d.Order.ID))
	require.NoError(t, err)
	assert.Equal(t, OrderPaid, paid.Order.Status)

	snap, err := EntitlementService.Capacities(ordUser)
	require.NoError(t, err)
	assert.Equal(t, int64(4), snap.BootSeat)

	acc, _ := BillingService.GetAccount(ordUser)
	assert.Equal(t, int64(0), acc.BalanceCents)
}

func TestPaidOrderLedgerLinkedByOrderID(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_orders", "billing_order_items", "billing_skus", "billing_discount_tiers",
			"billing_accounts", "billing_ledger_entries", "billing_entitlement_batches")
	})
	require.NoError(t, SeedCatalog(framework.DB))
	_, err := BillingService.Topup(ordUser, 1000000, "充值", "user:9101")
	require.NoError(t, err)

	d, err := OrderService.CreateOrder(ordUser, &OrderCreate{PayMethod: PayBalance,
		Items: []OrderItemRequest{
			{SkuCode: "instance_fee", CycleMonths: 1, Quantity: 2},
			{SkuCode: "time_pack", CycleMonths: 0, Quantity: 1000},
		}})
	require.NoError(t, err)
	_, err = OrderService.PayWithBalance(ordUser, int(d.Order.ID))
	require.NoError(t, err)

	// 该订单关联的流水：1 条余额消费 + 2 条资源发放，都应带 OrderID
	var cnt int64
	framework.DB.Model(&LedgerEntry{}).Where("order_id = ?", d.Order.ID).Count(&cnt)
	assert.Equal(t, int64(3), cnt)
}
