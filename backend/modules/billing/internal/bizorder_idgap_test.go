package billing

import (
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 订单号（主键）不连续：下单时在自增基础上额外加随机 0..4，相邻间隔恒在 [1,5]
// 且出现 >1 的不规则跳变，避免订单号被顺序枚举遍历他人订单（CP-0037 / #36，对标 user 模块）。
func TestBizOrderIDRandomGap(t *testing.T) {
	// 清空后从空表起算，使 MAX(id)=0、prev=0 成立；用例结束再清理自造行。
	framework.CleanTable("billing_biz_orders", "billing_biz_order_items")
	t.Cleanup(func() {
		framework.CleanTable("billing_biz_orders", "billing_biz_order_items")
	})

	repo := newBizOrderRepository(framework.DB)
	prev := uint(0)
	sawGap := false
	for i := 0; i < 15; i++ {
		o := &BizOrder{UserID: 960001, BizType: BizRecharge, Status: BizOrderUnpaid, TotalCents: 1000, PayMethod: PayAlipay}
		require.NoError(t, repo.create(o, nil))
		require.NotZero(t, o.ID)
		gap := o.ID - prev
		assert.GreaterOrEqual(t, gap, uint(1), "订单号必须严格递增")
		assert.LessOrEqual(t, gap, uint(5), "相邻订单号间隔应不超过 5")
		if gap > 1 {
			sawGap = true
		}
		prev = o.ID
	}
	assert.True(t, sawGap, "订单号应出现 >1 的随机间隔（非纯自增，防枚举）")
}
