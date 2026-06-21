package billing

import (
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPricingConfig_QuoteSeatFromConfig(t *testing.T) {
	// 默认配置：seat 单价 3000，数量10命中9折，时长12命中7折。
	q, err := PricingConfigService.QuoteKindDuration(KindSeat, 10, 12)
	require.NoError(t, err)
	// 3000 × 10 × 12 × 0.9 × 0.7 = 226800
	assert.Equal(t, int64(226800), q.PayableCents)
	assert.Equal(t, 9000, q.QtyDiscountBps)
	assert.Equal(t, 7000, q.DurationDiscountBps)
}

func TestPricingConfig_QuoteRuntimePack(t *testing.T) {
	// 默认：每分钟 20 分，600 分钟包全价。
	q, err := PricingConfigService.QuoteRuntimePack(600)
	require.NoError(t, err)
	assert.Equal(t, int64(12000), q.PayableCents)

	// 3000 分钟享 9 折：20 × 3000 × 0.9 = 54000
	q2, err := PricingConfigService.QuoteRuntimePack(3000)
	require.NoError(t, err)
	assert.Equal(t, int64(54000), q2.PayableCents)

	// 低于最低值报错。
	_, err = PricingConfigService.QuoteRuntimePack(10)
	assert.Error(t, err)
}

func TestPricingConfig_SavePartialPersists(t *testing.T) {
	t.Cleanup(func() {
		// 还原默认，避免污染其他用例。
		_ = PricingConfigService.Save(defaultPricingConfig())
	})
	_, err := PricingConfigService.SavePartial(func(d *PricingConfigData) {
		kp := d.Kinds[KindSeat]
		kp.UnitPriceCents = 9999
		d.Kinds[KindSeat] = kp
	})
	require.NoError(t, err)

	got, err := PricingConfigService.Get()
	require.NoError(t, err)
	assert.Equal(t, int64(9999), got.Kinds[KindSeat].UnitPriceCents)
	_ = framework.DB
}
