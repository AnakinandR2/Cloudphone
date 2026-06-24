<script setup lang="ts">
import type { PurchaseConfig } from '@/types/billing'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import billingApi from '@/api/modules/billing'
import { Input } from '@/components/ui/input'
import { fmtCents, fmtDiscountBps } from '@/utils/money'
import OrderSummary from './OrderSummary.vue'
import PaymentBox from './PaymentBox.vue'
import PurchaseNotice from './PurchaseNotice.vue'
import { feeOf, useQuote } from './useQuote'

// 临时开机时长包面板：须知 + 时长包按钮组/手输(≥min) + OrderSummary + PaymentBox。
const props = defineProps<{
  config: PurchaseConfig
  balanceCents: number
}>()

const emit = defineEmits<{ paid: [] }>()
const { t } = useI18n()

const rt = computed(() => props.config.runtime_pack)
const selected = ref<number | 'custom'>(rt.value.packs[0]?.minutes ?? 'custom')
const customMinutes = ref<number | undefined>(undefined)
const payMethod = ref('')
const submitting = ref(false)

const minutes = computed(() => {
  if (selected.value === 'custom') return Math.max(0, Math.floor(customMinutes.value || 0))
  return selected.value
})
const customInvalid = computed(() =>
  selected.value === 'custom' && minutes.value < rt.value.min_minutes)

// 自定义分钟数命中的时长包折扣（≤ 分钟数的最高门槛档），用于自定义输入右边的「已享 x折」提示。
const hitBps = computed(() => {
  let bestMin: number | null = null
  let bps = 10000
  for (const p of rt.value.packs) {
    if (p.minutes <= minutes.value && (bestMin === null || p.minutes > bestMin)) {
      bestMin = p.minutes
      bps = p.discount_bps
    }
  }
  return bps
})
const hitLabel = computed(() => fmtDiscountBps(hitBps.value))

const { quote, loading } = useQuote(() => {
  if (minutes.value <= 0 || (selected.value === 'custom' && customInvalid.value)) return null
  return { biz_type: 'runtime_pack', minutes: minutes.value }
})

// 选中支付方式的手续费配置（传给 OrderSummary 实时预览实付）。
const selectedFee = computed(() => feeOf(props.config.payment_methods, payMethod.value))

async function confirm() {
  if (minutes.value < rt.value.min_minutes) {
    toast.error(t('billing.purchase2.runtimeMinHint', { min: rt.value.min_minutes }))
    return
  }
  submitting.value = true
  try {
    await billingApi.createOrder2({ biz_type: 'runtime_pack', minutes: minutes.value, pay_method: payMethod.value })
    toast.success(t('billing.purchase2.runtimeOk'))
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
    <PurchaseNotice :notice="rt.notice" />

    <div class="flex flex-col gap-2.5">
      <span class="text-muted-foreground text-sm">{{ t('billing.purchase2.runtimePack') }}</span>
      <div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <button
          v-for="p in rt.packs"
          :key="p.minutes"
          type="button"
          class="relative cursor-pointer rounded-lg border px-3 py-3 text-center transition-colors"
          :class="selected === p.minutes ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
          @click="selected = p.minutes"
        >
          <span
            v-if="fmtDiscountBps(p.discount_bps)"
            class="absolute -top-2 -right-1.5 rounded-full bg-red-500 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-white shadow-sm"
          >{{ fmtDiscountBps(p.discount_bps) }}</span>
          <div class="text-sm font-medium tabular-nums">
            {{ p.minutes }} {{ t('billing.purchase2.minuteUnit') }}
          </div>
        </button>
        <button
          type="button"
          class="cursor-pointer rounded-lg border px-3 py-3 text-center transition-colors"
          :class="selected === 'custom' ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
          @click="selected = 'custom'"
        >
          <div class="text-sm font-medium">
            {{ t('billing.purchase2.custom') }}
          </div>
          <div class="text-muted-foreground text-xs tabular-nums">
            ¥{{ fmtCents(rt.unit_price_cents_per_minute) }} / {{ t('billing.purchase2.minuteUnit') }}
          </div>
        </button>
      </div>
    </div>

    <div v-if="selected === 'custom'" class="flex flex-col gap-1.5">
      <span class="text-muted-foreground text-sm">{{ t('billing.purchase2.customMinutes') }}</span>
      <div class="flex items-center gap-2">
        <Input
          v-model.number="customMinutes"
          type="number"
          :min="rt.min_minutes"
          step="10"
          class="h-9 w-40 tabular-nums"
          :aria-invalid="customInvalid"
          :class="customInvalid ? 'border-red-500 focus-visible:ring-red-500/30' : ''"
        />
        <span class="text-muted-foreground text-sm">{{ t('billing.purchase2.minuteUnit') }}</span>
        <span v-if="!customInvalid && hitBps < 10000" class="rounded-full bg-red-500/10 px-2 py-0.5 text-xs font-medium text-red-600 dark:text-red-400">
          {{ t('billing.purchase2.hitTier', { zhe: hitLabel }) }}
        </span>
      </div>
      <p :class="customInvalid ? 'text-red-500' : 'text-muted-foreground'" class="text-xs">
        {{ t('billing.purchase2.runtimeMinHint', { min: rt.min_minutes }) }}
      </p>
    </div>

    <OrderSummary :quote="quote" :loading="loading" :unit-label="t('billing.purchase2.minuteUnit')" :fee="selectedFee" />

    <PaymentBox
      v-model="payMethod"
      :methods="config.payment_methods"
      :allow-balance="true"
      :amount-cents="quote?.payable_cents ?? 0"
      :balance-cents="balanceCents"
      :disabled="customInvalid"
      :submitting="submitting"
      @confirm="confirm"
    />
  </div>
</template>
