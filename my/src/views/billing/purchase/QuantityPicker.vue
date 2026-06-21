<script setup lang="ts">
import type { QtyTier } from '@/types/billing'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Input } from '@/components/ui/input'
import { fmtDiscountBps } from '@/utils/money'

// 数量档位：预设按钮（含单位 + 右上角折扣徽标）+ 末尾「自定义」按钮（选中才出现输入框）。
// 档位直接由「数量阶梯」派生：始终含 1（全价基准）+ 各阶梯 min_quantity，去重升序。
const props = defineProps<{
  tiers: QtyTier[]
  unitLabel?: string
  min?: number
  max?: number
}>()

const model = defineModel<number>({ default: 1 })
const { t } = useI18n()

const minQty = computed(() => props.min ?? 1)
const maxQty = computed(() => props.max ?? 100000)

// 是否处于自定义输入模式（选中「自定义」按钮）。
const custom = ref(false)

// 可选数量档位 = 1 + 各阶梯门槛，去重升序。
const options = computed(() => {
  const set = new Set<number>([1])
  for (const tr of props.tiers) {
    if (tr.min_quantity > 0)
      set.add(tr.min_quantity)
  }
  return [...set].sort((a, b) => a - b)
})

// 某数量命中的阶梯折扣 bps（取 min_quantity 最大且 <= 数量者）。
function bpsForQty(qty: number) {
  const hit = props.tiers
    .filter(tr => tr.min_quantity <= qty)
    .reduce<QtyTier | null>((best, tr) => (!best || tr.min_quantity > best.min_quantity ? tr : best), null)
  return hit?.discount_bps ?? 10000
}
// 某档位的折扣文案（仅在有折扣时返回，全价返回空）。
function discountLabelForQty(qty: number) {
  const bps = bpsForQty(qty)
  return bps < 10000 ? fmtDiscountBps(bps) : ''
}

const hitBps = computed(() => bpsForQty(model.value))
const hitLabel = computed(() => fmtDiscountBps(hitBps.value))

function clamp(v: number) {
  if (!Number.isFinite(v)) return minQty.value
  return Math.min(maxQty.value, Math.max(minQty.value, Math.floor(v)))
}
// 选预设档位：退出自定义模式。
function pick(v: number) {
  model.value = clamp(v)
  custom.value = false
}
// 进入自定义模式。
function enterCustom() {
  custom.value = true
}
function onInput() {
  model.value = clamp(model.value)
}
</script>

<template>
  <div class="flex flex-col gap-2.5">
    <span class="text-muted-foreground text-sm">{{ t('billing.purchase2.quantity') }}</span>
    <div class="flex flex-wrap gap-x-2 gap-y-3">
      <button
        v-for="opt in options"
        :key="opt"
        type="button"
        class="relative cursor-pointer rounded-lg border px-3 py-2 text-center transition-colors"
        :class="!custom && model === opt ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
        @click="pick(opt)"
      >
        <span
          v-if="discountLabelForQty(opt)"
          class="absolute -top-2 -right-1.5 rounded-full bg-red-500 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-white shadow-sm"
        >{{ discountLabelForQty(opt) }}</span>
        <span class="text-sm font-medium tabular-nums">{{ opt }} {{ unitLabel || t('billing.purchase2.unitDefault') }}</span>
      </button>
      <!-- 自定义：选中才出现输入框 -->
      <button
        type="button"
        class="cursor-pointer rounded-lg border px-3 py-2 text-center text-sm font-medium transition-colors"
        :class="custom ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
        @click="enterCustom"
      >
        {{ t('billing.purchase2.customQty') }}
      </button>
    </div>
    <!-- 自定义输入 + 命中折扣提示（说明自定义数量已享对应折扣） -->
    <div v-if="custom" class="flex items-center gap-2">
      <Input
        v-model.number="model"
        type="number"
        :min="minQty"
        :max="maxQty"
        class="h-9 w-28 tabular-nums"
        @blur="onInput"
        @keyup.enter="onInput"
      />
      <span class="text-muted-foreground text-sm">{{ unitLabel || t('billing.purchase2.unitDefault') }}</span>
      <span v-if="hitBps < 10000" class="rounded-full bg-red-500/10 px-2 py-0.5 text-xs font-medium text-red-600 dark:text-red-400">
        {{ t('billing.purchase2.hitTier', { zhe: hitLabel }) }}
      </span>
    </div>
  </div>
</template>
