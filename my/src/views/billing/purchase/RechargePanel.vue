<script setup lang="ts">
import type { PurchaseConfig } from '@/types/billing'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import billingApi from '@/api/modules/billing'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import { fmtCents } from '@/utils/money'
import PaymentBox from './PaymentBox.vue'

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

const amountCents = computed(() => {
  if (presetCents.value === 'custom') {
    const y = customYuan.value
    if (!y || y <= 0) return 0
    return Math.round(y * 100)
  }
  return presetCents.value
})
const customInvalid = computed(() => presetCents.value === 'custom' && amountCents.value <= 0)

async function confirm() {
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
          step="0.01"
          class="h-9 w-40 tabular-nums"
          :aria-invalid="customInvalid"
          :class="customInvalid ? 'border-red-500 focus-visible:ring-red-500/30' : ''"
        />
        <span class="text-muted-foreground text-sm">{{ t('billing.purchase2.yuan') }}</span>
      </div>
    </div>

    <Separator />
    <div class="flex items-baseline justify-between">
      <span class="text-sm font-medium">{{ t('billing.purchase2.rechargePayable') }}</span>
      <span class="text-2xl font-semibold tabular-nums text-red-600">¥{{ fmtCents(amountCents) }}</span>
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
