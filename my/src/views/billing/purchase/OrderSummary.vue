<script setup lang="ts">
import type { PaymentMethod, QuoteResult2 } from '@/types/billing'
import { HelpCircle, Loader2 } from 'lucide-vue-next'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { Separator } from '@/components/ui/separator'
import { formatDate } from '@/utils/date'
import { computeFeeCents, feeWaived, fmtCents, fmtDiscountBps, fmtFeeHint } from '@/utils/money'

// 订单摘要：数量 / 折后单价 / 总价 / 到期时间。折扣相乘（服务端已算，前端只展示）。
const props = defineProps<{
  quote: QuoteResult2 | null
  // 到期时间（续费/新购时由父级按选中时长计算后传入；时长包/充值不传）
  expireAt?: string | null
  unitLabel?: string
  loading?: boolean
  // 选中的支付方式（父面板按选中 code 从 config.payment_methods 反查后传入）；
  // 据此取 fee 费率/满额免阈值/logo_url；缺省视为无手续费、无 logo（向后兼容）。
  method?: PaymentMethod | null
}>()

const { t } = useI18n()

const logoError = ref(false)
// 切换支付方式 / logo 变化时重置失败标记，避免上一方式的失败残留把新方式的有效 logo 也隐藏。
watch(() => props.method?.logo_url, () => { logoError.value = false })

// 折后单价 = payable / billing_units（展示用，含数量×时长两层折扣的均摊）
const discountedUnit = computed(() => {
  const q = props.quote
  if (!q || q.billing_units <= 0) return 0
  return Math.round(q.payable_cents / q.billing_units)
})
const qtyZhe = computed(() => fmtDiscountBps(props.quote?.qty_discount_bps ?? 10000))
const durZhe = computed(() => fmtDiscountBps(props.quote?.duration_discount_bps ?? 10000))

// 手续费基数 = 折后应付（payable_cents）。
const baseCents = computed(() => props.quote?.payable_cents ?? 0)
// 原始应收手续费（不含阈值；与后端 applyBps 同舍入），用于满额免时画删除线。
const rawFeeCents = computed(() =>
  computeFeeCents(baseCents.value, props.method?.fee_percent_bps ?? 0, props.method?.fee_fixed_cents ?? 0))
// 满额免：本应收>0 且 基数达阈值。
const waived = computed(() =>
  rawFeeCents.value > 0 && feeWaived(baseCents.value, props.method?.fee_free_threshold_cents ?? 0))
// 实收手续费 = 满额免则 0，否则原始应收。
const feeCents = computed(() => (waived.value ? 0 : rawFeeCents.value))
// 进入「有手续费分支」的条件：本应收>0（满额免也走此分支，显示删除线 + ? 提示）。
const hasFee = computed(() => rawFeeCents.value > 0)
// 实付 = 折后应付 + 实收手续费。
const payActualCents = computed(() => baseCents.value + feeCents.value)
// 手续费构成标注，如「2% + ¥1」。
const feeHint = computed(() => fmtFeeHint(props.method?.fee_percent_bps ?? 0, props.method?.fee_fixed_cents ?? 0))
// 满额免阈值（>0 时显示 ? 气泡）。
const freeThresholdCents = computed(() => props.method?.fee_free_threshold_cents ?? 0)
const freeThresholdYuan = computed(() => fmtCents(freeThresholdCents.value))

// 行首渠道 logo 回退首字方块（与 PaymentBox 的 STYLE 映射一致）。
const LOGO_STYLE: Record<string, { mark: string, color: string }> = {
  balance: { mark: '余', color: '#6366f1' },
  wechat: { mark: '微', color: '#07C160' },
  alipay: { mark: '支', color: '#1677FF' },
}
const feeLogoColor = computed(() => LOGO_STYLE[props.method?.code ?? '']?.color ?? '#64748b')
const feeLogoMark = computed(() => {
  const code = props.method?.code ?? ''
  return LOGO_STYLE[code]?.mark ?? code.slice(0, 1).toUpperCase()
})
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
            <span class="text-muted-foreground flex items-center gap-1.5">
              <template v-if="method">
                <img
                  v-if="method.logo_url && !logoError"
                  :src="method.logo_url"
                  :alt="method.name"
                  class="size-4 shrink-0 self-center rounded object-contain"
                  @error="logoError = true"
                >
                <span v-else class="flex size-4 shrink-0 items-center justify-center self-center rounded text-[9px] font-bold text-white" :style="{ backgroundColor: feeLogoColor }">{{ feeLogoMark }}</span>
              </template>
              <span>{{ t('billing.purchase2.sumFee') }}</span>
              <span v-if="feeHint" class="text-xs">（{{ feeHint }}）</span>
              <TooltipProvider v-if="freeThresholdCents > 0" :delay-duration="100">
                <Tooltip>
                  <TooltipTrigger as-child>
                    <button type="button" class="text-muted-foreground hover:text-foreground inline-flex cursor-pointer self-center transition-colors">
                      <HelpCircle class="size-3.5" />
                    </button>
                  </TooltipTrigger>
                  <TooltipContent class="max-w-60 text-xs" :side-offset="6">
                    {{ t('billing.purchase2.feeFreeHint', { amount: freeThresholdYuan }) }}
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </span>
            <span class="tabular-nums">
              <span v-if="waived" class="text-muted-foreground mr-1 line-through">¥{{ fmtCents(rawFeeCents) }}</span>
              <span :class="waived ? 'text-emerald-600 dark:text-emerald-400' : ''">¥{{ fmtCents(feeCents) }}</span>
            </span>
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
