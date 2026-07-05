<script setup lang="ts">
import type { PurchaseConfig } from '@/types/billing'
import { HelpCircle } from 'lucide-vue-next'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import billingApi from '@/api/modules/billing'
import { Input } from '@/components/ui/input'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { Separator } from '@/components/ui/separator'
import { computeFeeCents, feeWaived, fmtCents, fmtFeeHint, MAX_RECHARGE_CENTS, validateRechargeYuan } from '@/utils/money'
import PaymentBox from './PaymentBox.vue'
import { feeOf } from './useQuote'

// 充值面板：预设金额 + 手输 + PaymentBox(allowBalance=false)。
const props = defineProps<{
  config: PurchaseConfig
  balanceCents: number
}>()

const emit = defineEmits<{ paid: [] }>()
const { t } = useI18n()

// 选中预设（cents）或自定义（元）
const presetCents = ref<number | 'custom'>(props.config.recharge_presets_cents[0] ?? 'custom')
const customYuan = ref<number | undefined>(undefined)
const payMethod = ref('')
const submitting = ref(false)

// 自定义金额校验（CP-0041 / #37）：拒绝 >2 位小数（不静默截断）、越上限，纯函数集中判定。
const customCheck = computed(() => validateRechargeYuan(customYuan.value))
const amountCents = computed(() => {
  if (presetCents.value === 'custom')
    return customCheck.value.error ? 0 : customCheck.value.cents
  return presetCents.value
})
const customInvalid = computed(() => presetCents.value === 'custom' && !!customCheck.value.error)
// 就地错误文案：仅对「精度/上限」这类明确非法给出提示；空输入不飘红文案（交给确认时兜底）。
const customError = computed(() => {
  if (presetCents.value !== 'custom') return ''
  switch (customCheck.value.error) {
    case 'precision': return t('billing.purchase2.rechargeAmountPrecision')
    case 'max': return t('billing.purchase2.rechargeAmountMax', { max: fmtCents(MAX_RECHARGE_CENTS) })
    default: return ''
  }
})

// 选中的支付方式（充值不允许余额方式；缺省视为无手续费、无 logo）。
const selectedMethod = computed(() => feeOf(props.config.payment_methods, payMethod.value))
// 手续费基数 = 充值面额（amountCents）。原始应收手续费（不含阈值，用于满额免画删除线）。
const rawFeeCents = computed(() =>
  computeFeeCents(amountCents.value, selectedMethod.value?.fee_percent_bps ?? 0, selectedMethod.value?.fee_fixed_cents ?? 0))
// 满额免：本应收>0 且 面额达阈值。
const waived = computed(() =>
  rawFeeCents.value > 0 && feeWaived(amountCents.value, selectedMethod.value?.fee_free_threshold_cents ?? 0))
// 实收手续费 = 满额免则 0，否则原始应收。
const feeCents = computed(() => (waived.value ? 0 : rawFeeCents.value))
// 进入「有手续费分支」的条件：本应收>0（满额免也走此分支，显示删除线 + ? 提示）。
const hasFee = computed(() => rawFeeCents.value > 0)
const payActualCents = computed(() => amountCents.value + feeCents.value)
// 手续费构成标注，如「2% + ¥1」。
const feeHint = computed(() => fmtFeeHint(selectedMethod.value?.fee_percent_bps ?? 0, selectedMethod.value?.fee_fixed_cents ?? 0))
// 满额免阈值（>0 时显示 ? 气泡）。
const freeThresholdCents = computed(() => selectedMethod.value?.fee_free_threshold_cents ?? 0)
const freeThresholdYuan = computed(() => fmtCents(freeThresholdCents.value))

// 行首渠道 logo 回退首字方块（与 PaymentBox 的 STYLE 映射一致）。
const logoError = ref(false)
// 切换支付方式 / logo 变化时重置失败标记，避免上一方式的失败残留把新方式的有效 logo 也隐藏。
watch(() => selectedMethod.value?.logo_url, () => { logoError.value = false })
const LOGO_STYLE: Record<string, { mark: string, color: string }> = {
  balance: { mark: '余', color: '#6366f1' },
  wechat: { mark: '微', color: '#07C160' },
  alipay: { mark: '支', color: '#1677FF' },
}
const feeLogoColor = computed(() => LOGO_STYLE[selectedMethod.value?.code ?? '']?.color ?? '#64748b')
const feeLogoMark = computed(() => {
  const code = selectedMethod.value?.code ?? ''
  return LOGO_STYLE[code]?.mark ?? code.slice(0, 1).toUpperCase()
})

async function confirm() {
  // 自定义金额非法（精度/上限/空）→ 优先给出具体原因，绝不带着被截断的金额下单。
  if (presetCents.value === 'custom' && customCheck.value.error) {
    toast.error(customError.value || t('billing.purchase2.rechargeAmountRequired'))
    return
  }
  if (amountCents.value <= 0) {
    toast.error(t('billing.purchase2.rechargeAmountRequired'))
    return
  }
  submitting.value = true
  try {
    await billingApi.createOrder2({
      biz_type: 'recharge',
      amount_cents: amountCents.value,
      pay_method: payMethod.value,
    })
    toast.success(t('billing.purchase2.rechargeOk'))
    presetCents.value = props.config.recharge_presets_cents[0] ?? 'custom'
    customYuan.value = undefined
    emit('paid')
  }
  catch {
    // 失败由响应拦截器统一提示
  }
  finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-5">
    <div class="flex flex-col gap-2.5">
      <span class="text-muted-foreground text-sm">{{ t('billing.purchase2.rechargeAmount') }}</span>
      <div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
        <button
          v-for="c in config.recharge_presets_cents"
          :key="c"
          type="button"
          class="cursor-pointer rounded-lg border px-3 py-3 text-center transition-colors"
          :class="presetCents === c ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
          @click="presetCents = c"
        >
          <span class="text-lg font-semibold tabular-nums">¥{{ fmtCents(c) }}</span>
        </button>
        <button
          type="button"
          class="cursor-pointer rounded-lg border px-3 py-3 text-center transition-colors"
          :class="presetCents === 'custom' ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
          @click="presetCents = 'custom'"
        >
          <span class="text-sm font-medium">{{ t('billing.purchase2.custom') }}</span>
        </button>
      </div>
    </div>

    <div v-if="presetCents === 'custom'" class="flex flex-col gap-1.5">
      <span class="text-muted-foreground text-sm">{{ t('billing.purchase2.customAmount') }}</span>
      <div class="flex items-center gap-2">
        <span class="text-muted-foreground text-sm">¥</span>
        <Input
          v-model.number="customYuan"
          type="number"
          min="0.01"
          max="100000"
          step="0.01"
          class="h-9 w-40 tabular-nums"
          :aria-invalid="customInvalid"
          :class="customInvalid ? 'border-red-500 focus-visible:ring-red-500/30' : ''"
        />
        <span class="text-muted-foreground text-sm">{{ t('billing.purchase2.yuan') }}</span>
      </div>
      <p v-if="customError" class="text-xs text-red-600">{{ customError }}</p>
    </div>

    <Separator />
    <!-- 充值小结：充值金额 → 手续费 → 实付（实付为突出大字）。 -->
    <div class="flex flex-col gap-2 text-sm">
      <div class="flex items-baseline justify-between">
        <span :class="hasFee ? 'text-muted-foreground' : 'font-medium'">{{ t('billing.purchase2.rechargeFaceAmount') }}</span>
        <span class="tabular-nums" :class="hasFee ? '' : 'text-2xl font-semibold text-red-600'">¥{{ fmtCents(amountCents) }}</span>
      </div>
      <template v-if="hasFee">
        <div class="flex items-baseline justify-between">
          <span class="text-muted-foreground flex items-center gap-1.5">
            <template v-if="selectedMethod">
              <img
                v-if="selectedMethod.logo_url && !logoError"
                :src="selectedMethod.logo_url"
                :alt="selectedMethod.name"
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
    </div>

    <!-- 充值不能用余额支付 -->
    <PaymentBox
      v-model="payMethod"
      :methods="config.payment_methods"
      :allow-balance="false"
      :amount-cents="amountCents"
      :submitting="submitting"
      :confirm-label="t('billing.purchase2.rechargeConfirm')"
      @confirm="confirm"
    />
  </div>
</template>
