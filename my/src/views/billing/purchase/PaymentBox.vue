<script setup lang="ts">
import type { PaymentMethod } from '@/types/billing'
import { Check } from 'lucide-vue-next'
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { computeFeeCents, feeWaived, fmtCents } from '@/utils/money'

// 支付方式选择 + 确认支付。
// allowBalance：充值=false（不能用余额买余额），其它=true。
// 方法来自 purchase-config 的 payment_methods（开关/排序）。
const props = withDefaults(defineProps<{
  methods: PaymentMethod[]
  allowBalance?: boolean
  amountCents: number
  balanceCents?: number
  disabled?: boolean
  submitting?: boolean
  confirmLabel?: string
}>(), { allowBalance: true })

const emit = defineEmits<{ confirm: [] }>()

const model = defineModel<string>({ default: '' })
const { t } = useI18n()

// 视觉标识（mark/颜色）按 code 映射，未知 code 用首字 + 中性色。
const STYLE: Record<string, { mark: string, color: string }> = {
  balance: { mark: '余', color: '#6366f1' },
  wechat: { mark: '微', color: '#07C160' },
  alipay: { mark: '支', color: '#1677FF' },
}
function styleOf(code: string) {
  return STYLE[code] ?? { mark: code.slice(0, 1).toUpperCase(), color: '#64748b' }
}

// logo 加载失败的 code 集合 → 回退首字方块（避免白块）。
const logoError = reactive<Record<string, boolean>>({})

// 每个支付方式在当前订单金额下的手续费展示（只显金额）：
// 无配置(bps=0&&fixed=0) → 无手续费；满额免(达标且本应收>0) → 免手续费；否则 「{手续费} ¥{rawFee}」。
// amountCents<=0（金额未定）返回空串 → 不显示该行。
function feeTextOf(m: PaymentMethod): string {
  if (props.amountCents <= 0) return ''
  if (m.fee_percent_bps === 0 && m.fee_fixed_cents === 0) return t('billing.purchase2.feeNone')
  const rawFee = computeFeeCents(props.amountCents, m.fee_percent_bps, m.fee_fixed_cents)
  if (rawFee > 0 && feeWaived(props.amountCents, m.fee_free_threshold_cents)) return t('billing.purchase2.feeWaived')
  return `${t('billing.purchase2.sumFee')} ¥${fmtCents(rawFee)}`
}

// 可选支付方式：启用 + 按 sort 排序；allowBalance=false 时剔除余额。
const usable = computed(() => props.methods
  .filter(m => m.enabled && (props.allowBalance || m.code !== 'balance'))
  .sort((a, b) => a.sort - b.sort))

// 默认选中第一个可用方式；若当前选中失效则重置。
watch(usable, (list) => {
  if (!list.length) return
  if (!list.some(m => m.code === model.value)) model.value = list[0].code
}, { immediate: true })

// 余额支付时校验余额是否充足
const balanceShort = computed(() =>
  model.value === 'balance'
  && props.balanceCents !== undefined
  && props.balanceCents < props.amountCents)

const payDisabled = computed(() =>
  props.disabled || props.submitting || !usable.value.length || balanceShort.value || props.amountCents <= 0)
</script>

<template>
  <div class="flex flex-col gap-3">
    <span class="text-muted-foreground text-sm">{{ t('billing.purchase2.payMethod') }}</span>
    <div class="grid gap-2 sm:grid-cols-3">
      <button
        v-for="m in usable"
        :key="m.code"
        type="button"
        class="relative flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2.5 text-left transition-colors"
        :class="model === m.code ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
        @click="model = m.code"
      >
        <img
          v-if="m.logo_url && !logoError[m.code]"
          :src="m.logo_url"
          :alt="m.name"
          class="size-6 shrink-0 rounded object-contain"
          @error="logoError[m.code] = true"
        >
        <span v-else class="flex size-6 shrink-0 items-center justify-center rounded text-xs font-bold text-white" :style="{ backgroundColor: styleOf(m.code).color }">{{ styleOf(m.code).mark }}</span>
        <span class="flex min-w-0 flex-col">
          <span class="text-sm font-medium">{{ m.name }}</span>
          <span v-if="feeTextOf(m)" class="text-muted-foreground text-xs tabular-nums">{{ feeTextOf(m) }}</span>
        </span>
        <Check v-if="model === m.code" class="text-primary absolute top-1/2 right-2 size-3.5 -translate-y-1/2" />
      </button>
    </div>

    <!-- 余额支付 + 余额展示/不足提示 -->
    <div v-if="model === 'balance' && balanceCents !== undefined" class="flex items-center justify-between text-xs">
      <span class="text-muted-foreground">{{ t('billing.purchase2.balanceRemain') }}</span>
      <span :class="balanceShort ? 'text-red-500 font-medium' : 'text-muted-foreground'">
        ¥{{ (balanceCents / 100).toFixed(2) }}
        <span v-if="balanceShort" class="ml-1">{{ t('billing.purchase2.balanceShort') }}</span>
      </span>
    </div>

    <Button class="w-full" size="lg" :disabled="payDisabled" @click="emit('confirm')">
      {{ submitting ? t('billing.purchase2.submitting') : (confirmLabel || t('billing.purchase2.confirmPay')) }}
    </Button>
  </div>
</template>
