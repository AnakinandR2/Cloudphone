<script setup lang="ts">
import type { LicenseKind, PurchaseConfig } from '@/types/billing'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import billingApi from '@/api/modules/billing'
import DurationPicker from './DurationPicker.vue'
import OrderSummary from './OrderSummary.vue'
import PaymentBox from './PaymentBox.vue'
import UnitRenewTable from './UnitRenewTable.vue'
import { bizTypeOf, feeOf, useQuote } from './useQuote'

// 组合面板（续费）：续费实例 / 续费包月数 共用，按 kind 参数化。
// 续费数量 = 选中的单元数（享数量阶梯折扣）。
const props = defineProps<{
  kind: LicenseKind
  config: PurchaseConfig
  balanceCents: number
}>()

const emit = defineEmits<{ paid: [] }>()
const { t } = useI18n()

const kindCfg = computed(() => props.config.kinds[props.kind])

const selectedIds = ref<number[]>([])
const durationValue = ref(kindCfg.value.duration_options[0]?.value ?? 1)
const payMethod = ref('')
const submitting = ref(false)
const renewTable = ref<InstanceType<typeof UnitRenewTable> | null>(null)

const { quote, loading } = useQuote(() => {
  if (!selectedIds.value.length) return null
  return {
    biz_type: bizTypeOf(props.kind, 'renew'),
    quantity: selectedIds.value.length,
    duration_value: durationValue.value,
  }
})

// 选中支付方式的手续费配置（传给 OrderSummary 实时预览实付）。
const selectedFee = computed(() => feeOf(props.config.payment_methods, payMethod.value))

async function confirm() {
  if (!selectedIds.value.length) {
    toast.error(t('billing.purchase2.renewNeedSelect'))
    return
  }
  submitting.value = true
  try {
    await billingApi.createOrder2({
      biz_type: bizTypeOf(props.kind, 'renew'),
      quantity: selectedIds.value.length,
      duration_value: durationValue.value,
      unit_ids: selectedIds.value,
      pay_method: payMethod.value,
    })
    toast.success(t('billing.purchase2.renewOk'))
    selectedIds.value = []
    renewTable.value?.reload()
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
    <UnitRenewTable ref="renewTable" v-model="selectedIds" :kind="kind" />

    <DurationPicker
      v-model="durationValue"
      :options="kindCfg.duration_options"
      :unit="kindCfg.duration_unit"
    />

    <p class="text-muted-foreground text-xs">
      {{ kindCfg.billing_note }}
    </p>

    <OrderSummary
      :quote="quote"
      :loading="loading"
      :unit-label="kindCfg.unit_label"
      :fee="selectedFee"
    />

    <PaymentBox
      v-model="payMethod"
      :methods="config.payment_methods"
      :allow-balance="true"
      :amount-cents="quote?.payable_cents ?? 0"
      :balance-cents="balanceCents"
      :disabled="!selectedIds.length"
      :submitting="submitting"
      :confirm-label="t('billing.purchase2.renewConfirm')"
      @confirm="confirm"
    />
  </div>
</template>
