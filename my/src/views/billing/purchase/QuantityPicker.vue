<script setup lang="ts">
import type { QtyTier } from '@/types/billing'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Input } from '@/components/ui/input'
import { fmtDiscountBps } from '@/utils/money'

// 数量拉条（按钮档位 + 手输）+ 展示当前数量命中的阶梯折扣。
// 本环境无 Slider 组件，沿用项目既有「按钮档位 + 数字输入」交互。
const props = defineProps<{
  options: number[]
  tiers: QtyTier[]
  unitLabel?: string
  min?: number
  max?: number
}>()

const model = defineModel<number>({ default: 1 })
const { t } = useI18n()

const minQty = computed(() => props.min ?? 1)
const maxQty = computed(() => props.max ?? 100000)

// 当前数量命中的阶梯折扣（取 min_quantity 最大且 <= 数量者）。
const hitBps = computed(() => {
  const hit = props.tiers
    .filter(tr => tr.min_quantity <= model.value)
    .reduce<QtyTier | null>((best, tr) => (!best || tr.min_quantity > best.min_quantity ? tr : best), null)
  return hit?.discount_bps ?? 10000
})
const hitLabel = computed(() => fmtDiscountBps(hitBps.value))

function clamp(v: number) {
  if (!Number.isFinite(v)) return minQty.value
  return Math.min(maxQty.value, Math.max(minQty.value, Math.floor(v)))
}
function pick(v: number) {
  model.value = clamp(v)
}
function onInput() {
  model.value = clamp(model.value)
}
</script>

<template>
  <div class="flex flex-col gap-2.5">
    <div class="flex items-center justify-between">
      <span class="text-muted-foreground text-sm">{{ t('billing.purchase2.quantity') }}</span>
      <span v-if="hitLabel" class="rounded-full bg-red-500/10 px-2 py-0.5 text-xs font-medium text-red-600 dark:text-red-400">
        {{ t('billing.purchase2.hitTier', { zhe: hitLabel }) }}
      </span>
    </div>
    <div class="flex flex-wrap gap-2">
      <button
        v-for="opt in options"
        :key="opt"
        type="button"
        class="cursor-pointer rounded-lg border px-3 py-1.5 text-sm font-medium tabular-nums transition-colors"
        :class="model === opt ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
        @click="pick(opt)"
      >
        {{ opt }}
      </button>
    </div>
    <div class="flex items-center gap-2">
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
    </div>
  </div>
</template>
