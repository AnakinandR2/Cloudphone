<script setup lang="ts">
import type { QuoteResult2 } from '@/types/billing'
import { Loader2 } from 'lucide-vue-next'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Separator } from '@/components/ui/separator'
import { formatDate } from '@/utils/date'
import { fmtCents, fmtDiscountBps } from '@/utils/money'

// 订单摘要：数量 / 折后单价 / 总价 / 到期时间。折扣相乘（服务端已算，前端只展示）。
const props = defineProps<{
  quote: QuoteResult2 | null
  // 到期时间（续费/新购时由父级按选中时长计算后传入；时长包/充值不传）
  expireAt?: string | null
  unitLabel?: string
  loading?: boolean
}>()

const { t } = useI18n()

// 折后单价 = payable / billing_units（展示用，含数量×时长两层折扣的均摊）
const discountedUnit = computed(() => {
  const q = props.quote
  if (!q || q.billing_units <= 0) return 0
  return Math.round(q.payable_cents / q.billing_units)
})
const qtyZhe = computed(() => fmtDiscountBps(props.quote?.qty_discount_bps ?? 10000))
const durZhe = computed(() => fmtDiscountBps(props.quote?.duration_discount_bps ?? 10000))
</script>

<template>
  <div class="bg-muted/30 relative flex flex-col gap-2 rounded-lg border px-4 py-3.5 text-sm">
    <div class="flex flex-col gap-2 transition-opacity" :class="loading ? 'opacity-40' : ''">
      <template v-if="quote">
        <div class="flex items-center justify-between">
          <span class="text-muted-foreground">{{ t('billing.purchase2.sumQuantity') }}</span>
          <span class="tabular-nums">{{ quote.quantity }} {{ unitLabel || t('billing.purchase2.unitDefault') }}</span>
        </div>
        <div v-if="quote.duration_value" class="flex items-center justify-between">
          <span class="text-muted-foreground">{{ t('billing.purchase2.sumBillingUnits') }}</span>
          <span class="tabular-nums">{{ quote.billing_units }}</span>
        </div>
        <div class="flex items-center justify-between">
          <span class="text-muted-foreground">{{ t('billing.purchase2.sumUnitPrice') }}</span>
          <span class="tabular-nums">¥{{ fmtCents(discountedUnit) }}</span>
        </div>
        <div v-if="qtyZhe || durZhe" class="flex items-center justify-between">
          <span class="text-muted-foreground">{{ t('billing.purchase2.sumDiscount') }}</span>
          <span class="tabular-nums text-red-500">
            <span v-if="qtyZhe">{{ qtyZhe }}</span>
            <span v-if="qtyZhe && durZhe"> × </span>
            <span v-if="durZhe">{{ durZhe }}</span>
          </span>
        </div>
        <div v-if="expireAt" class="flex items-center justify-between">
          <span class="text-muted-foreground">{{ t('billing.purchase2.sumExpireAt') }}</span>
          <span class="tabular-nums">{{ formatDate(expireAt) }}</span>
        </div>
        <div v-if="quote.gift_runtime_minutes > 0" class="flex items-center justify-between">
          <span class="text-muted-foreground">{{ t('billing.purchase2.sumGift') }}</span>
          <span class="tabular-nums font-medium text-emerald-600 dark:text-emerald-400">
            +{{ quote.gift_runtime_minutes }} {{ t('billing.purchase2.minuteUnit') }}
          </span>
        </div>
        <Separator />
        <div class="flex items-baseline justify-between">
          <span class="font-medium">{{ t('billing.purchase2.sumTotal') }}</span>
          <span class="flex items-baseline gap-2">
            <span v-if="quote.original_cents > quote.payable_cents" class="text-muted-foreground text-sm line-through tabular-nums">¥{{ fmtCents(quote.original_cents) }}</span>
            <span class="text-2xl font-semibold tabular-nums text-red-600">¥{{ fmtCents(quote.payable_cents) }}</span>
          </span>
        </div>
      </template>
      <div v-else class="text-muted-foreground py-2 text-center text-xs">
        {{ t('billing.purchase2.selectToQuote') }}
      </div>
    </div>

    <!-- 报价中：覆盖加载动画（含已有数据时的重新报价，避免数据无提示突变） -->
    <div v-if="loading" class="text-muted-foreground absolute inset-0 flex items-center justify-center gap-2 text-xs">
      <Loader2 class="size-4 animate-spin" /> {{ t('billing.purchase2.quoting') }}
    </div>
  </div>
</template>
