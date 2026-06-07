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
