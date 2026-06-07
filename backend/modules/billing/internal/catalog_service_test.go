package billing

import (
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
