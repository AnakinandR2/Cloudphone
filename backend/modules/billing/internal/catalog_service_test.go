package billing

import (
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkuAndTierCRUD(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_skus", "billing_discount_tiers") })

	listed := true
	sku, err := CatalogService.CreateSku(&SkuCreate{Code: "x_fee", Category: CategoryInstanceFee, Name: "测试费", UnitPriceCents: 5000, Unit: "台/月", Listed: &listed})
	require.NoError(t, err)
	assert.NotZero(t, sku.ID)
	assert.True(t, sku.Listed)

	_, err = CatalogService.CreateSku(&SkuCreate{Code: "x_fee", Category: CategoryInstanceFee, Name: "重复", UnitPriceCents: 1})
	assert.Error(t, err)

	_, err = CatalogService.CreateSku(&SkuCreate{Code: "bad", Category: "nope", Name: "x", UnitPriceCents: 1})
	assert.Error(t, err)

	newPrice := int64(6000)
	upd, err := CatalogService.UpdateSku(int(sku.ID), &SkuUpdate{UnitPriceCents: &newPrice})
	require.NoError(t, err)
	assert.Equal(t, int64(6000), upd.UnitPriceCents)

	tier, err := CatalogService.CreateTier(int(sku.ID), &TierCreate{CycleMonths: 12, MinQuantity: 1, DiscountBps: 7000})
	require.NoError(t, err)
	assert.Equal(t, uint(sku.ID), tier.SkuID)

	_, err = CatalogService.CreateTier(int(sku.ID), &TierCreate{CycleMonths: 1, MinQuantity: 1, DiscountBps: 12000})
	assert.Error(t, err)

	tiers, err := CatalogService.ListTiers(int(sku.ID))
	require.NoError(t, err)
	require.Len(t, tiers, 1)

	require.NoError(t, CatalogService.DeleteTier(int(tier.ID)))
	require.NoError(t, CatalogService.DeleteSku(int(sku.ID)))
	_, err = CatalogService.GetSku(int(sku.ID))
	assert.Error(t, err)
}

func TestListListedSkus(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_skus", "billing_discount_tiers") })
	require.NoError(t, SeedCatalog(framework.DB))

	all, err := CatalogService.ListSkus(true)
	require.NoError(t, err)
	require.NotEmpty(t, all)
	notListed := false
	_, err = CatalogService.UpdateSku(int(all[0].ID), &SkuUpdate{Listed: &notListed})
	require.NoError(t, err)

	listed, err := CatalogService.ListListedSkus()
	require.NoError(t, err)
	assert.Len(t, listed, 2)
	for _, s := range listed {
		assert.True(t, s.Sku.Listed)
	}
}

func TestSeedCatalogIdempotent(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_skus", "billing_discount_tiers") })

	require.NoError(t, SeedCatalog(framework.DB))
	var skuCount, tierCount int64
	framework.DB.Model(&Sku{}).Count(&skuCount)
	framework.DB.Model(&DiscountTier{}).Count(&tierCount)
	assert.Equal(t, int64(3), skuCount)
	assert.Equal(t, int64(9), tierCount)

	require.NoError(t, SeedCatalog(framework.DB))
	framework.DB.Model(&Sku{}).Count(&skuCount)
	framework.DB.Model(&DiscountTier{}).Count(&tierCount)
	assert.Equal(t, int64(3), skuCount)
	assert.Equal(t, int64(9), tierCount)
}

func TestDeleteSkuCascadesTiers(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_skus", "billing_discount_tiers") })

	listed := true
	sku, err := CatalogService.CreateSku(&SkuCreate{Code: "casc_fee", Category: CategoryInstanceFee, Name: "级联测试", UnitPriceCents: 1000, Unit: "台/月", Listed: &listed})
	require.NoError(t, err)
	_, err = CatalogService.CreateTier(int(sku.ID), &TierCreate{CycleMonths: 1, MinQuantity: 1, DiscountBps: 9000})
	require.NoError(t, err)
	_, err = CatalogService.CreateTier(int(sku.ID), &TierCreate{CycleMonths: 12, MinQuantity: 1, DiscountBps: 7000})
	require.NoError(t, err)

	// 直接删 SKU（不先删 tier）→ 应级联删掉其 2 条折扣阶梯
	require.NoError(t, CatalogService.DeleteSku(int(sku.ID)))

	_, err = CatalogService.GetSku(int(sku.ID))
	assert.Error(t, err)
	var remaining int64
	framework.DB.Model(&DiscountTier{}).Where("sku_id = ?", sku.ID).Count(&remaining)
	assert.Equal(t, int64(0), remaining)
}
