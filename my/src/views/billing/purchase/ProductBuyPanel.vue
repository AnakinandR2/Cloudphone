<script setup lang="ts">
import type { LicenseKind, PurchaseConfig } from '@/types/billing'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import DurationPicker from './DurationPicker.vue'
import OrderSummary from './OrderSummary.vue'
import PaymentBox from './PaymentBox.vue'
import PurchaseNotice from './PurchaseNotice.vue'
import QuantityPicker from './QuantityPicker.vue'
import { bizTypeOf, feeOf, useQuote } from './useQuote'

// 组合面板（新购）：kind=seat → 购买新实例；kind=boot_slot → 购买包月开机数（时长按天）。
const props = defineProps<{
  kind: LicenseKind
  config: PurchaseConfig
  balanceCents: number
}>()

const emit = defineEmits<{ paid: [] }>()
const { t } = useI18n()

const kindCfg = computed(() => props.config.kinds[props.kind])

// 单次下单数量上限，与后端 billing MaxOrderQuantity(=1000) 对齐：超限后端返回 422。
// 后端已权威拦截，这里前置卡上限只为给出即时反馈、避免无谓往返。
const MAX_ORDER_QUANTITY = 1000
const quantity = ref(1)
const durationValue = ref(kindCfg.value.duration_options[0]?.value ?? 1)
const payMethod = ref('')
const submitting = ref(false)

const { quote, loading } = useQuote(() => ({
  biz_type: bizTypeOf(props.kind, 'new'),
  quantity: quantity.value,
  duration_value: durationValue.value,
}))

// 选中的支付方式（传给 OrderSummary：fee 费率 / 满额免阈值 / logo 实时预览实付）。
const selectedMethod = computed(() => feeOf(props.config.payment_methods, payMethod.value))

// 到期 = now + 时长（月=按月加，天=按天加），仅用于摘要展示。
const expireAt = computed(() => {
  const d = new Date()
  if (kindCfg.value.duration_unit === 'month') d.setMonth(d.getMonth() + durationValue.value)
  else d.setDate(d.getDate() + durationValue.value)
  return d.toISOString()
})

async function confirm() {
  const billingApi = (await import('@/api/modules/billing')).default
  submitting.value = true
  try {
    await billingApi.createOrder2({
      biz_type: bizTypeOf(props.kind, 'new'),
      quantity: quantity.value,
      duration_value: durationValue.value,
      pay_method: payMethod.value,
    })
    const { toast } = await import('vue-sonner')
    toast.success(t('billing.purchase2.orderOk'))
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
    <PurchaseNotice :notice="kindCfg.notice" :billing-note="kindCfg.billing_note" />

    <QuantityPicker
      v-model="quantity"
      :tiers="kindCfg.qty_tiers"
      :unit-label="kindCfg.unit_label"
      :max="MAX_ORDER_QUANTITY"
    />

    <DurationPicker
      v-model="durationValue"
      :options="kindCfg.duration_options"
      :unit="kindCfg.duration_unit"
    />

    <OrderSummary
      :quote="quote"
      :loading="loading"
      :expire-at="expireAt"
      :unit-label="kindCfg.unit_label"
      :method="selectedMethod"
    />

    <PaymentBox
      v-model="payMethod"
      :methods="config.payment_methods"
      :allow-balance="true"
      :amount-cents="quote?.payable_cents ?? 0"
      :balance-cents="balanceCents"
      :submitting="submitting"
      @confirm="confirm"
    />
  </div>
</template>
