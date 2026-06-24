<script setup lang="ts">
import type { QuoteResult2 } from '@/types/billing'
import { Loader2 } from 'lucide-vue-next'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Separator } from '@/components/ui/separator'
import { formatDate } from '@/utils/date'
import { computeFeeCents, fmtCents, fmtDiscountBps, fmtFeeHint } from '@/utils/money'

// 订单摘要：数量 / 折后单价 / 总价 / 到期时间。折扣相乘（服务端已算，前端只展示）。
const props = defineProps<{
  quote: QuoteResult2 | null
  // 到期时间（续费/新购时由父级按选中时长计算后传入；时长包/充值不传）
  expireAt?: string | null
  unitLabel?: string
  loading?: boolean
  // 选中支付方式的手续费配置（父面板按选中 code 从 config.payment_methods 反查后传入）；
  // 缺省视为无手续费（向后兼容）。
  fee?: { fee_percent_bps: number, fee_fixed_cents: number } | null
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

// 手续费基数 = 折后应付（payable_cents）；与后端 computeFee 同公式实时预览。
const feeCents = computed(() => {
  const q = props.quote
  if (!q) return 0
  return computeFeeCents(q.payable_cents, props.fee?.fee_percent_bps ?? 0, props.fee?.fee_fixed_cents ?? 0)
})
const hasFee = computed(() => feeCents.value > 0)
// 实付 = 折后应付 + 手续费。
const payActualCents = computed(() => (props.quote?.payable_cents ?? 0) + feeCents.value)
// 手续费构成标注，如「2% + ¥1」。
const feeHint = computed(() => fmtFeeHint(props.fee?.fee_percent_bps ?? 0, props.fee?.fee_fixed_cents ?? 0))
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
        <!-- 无手续费：维持「应付总价」突出大字（原样）。 -->
        <div v-if="!hasFee" class="flex items-baseline justify-between">
          <span class="font-medium">{{ t('billing.purchase2.sumTotal') }}</span>
          <span class="flex items-baseline gap-2">
            <span v-if="quote.original_cents > quote.payable_cents" class="text-muted-foreground text-sm line-through tabular-nums">¥{{ fmtCents(quote.original_cents) }}</span>
            <span class="text-2xl font-semibold tabular-nums text-red-600">¥{{ fmtCents(quote.payable_cents) }}</span>
          </span>
        </div>
        <!-- 有手续费：应付总价降权为小计 → 手续费 → 实付（唯一突出）。 -->
        <template v-else>
          <div class="flex items-baseline justify-between">
            <span class="text-muted-foreground">{{ t('billing.purchase2.sumTotal') }}</span>
            <span class="flex items-baseline gap-2">
              <span v-if="quote.original_cents > quote.payable_cents" class="text-muted-foreground text-xs line-through tabular-nums">¥{{ fmtCents(quote.original_cents) }}</span>
              <span class="tabular-nums">¥{{ fmtCents(quote.payable_cents) }}</span>
            </span>
          </div>
          <div class="flex items-baseline justify-between">
            <span class="text-muted-foreground">
              {{ t('billing.purchase2.sumFee') }}
              <span v-if="feeHint" class="text-xs">（{{ feeHint }}）</span>
            </span>
            <span class="tabular-nums">¥{{ fmtCents(feeCents) }}</span>
          </div>
          <div class="flex items-baseline justify-between">
            <span class="font-medium">{{ t('billing.purchase2.sumPayActual') }}</span>
            <span class="text-2xl font-semibold tabular-nums text-red-600">¥{{ fmtCents(payActualCents) }}</span>
          </div>
        </template>
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
