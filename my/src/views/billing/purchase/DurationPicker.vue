<script setup lang="ts">
import type { DurationOption } from '@/types/billing'
import { useI18n } from 'vue-i18n'
import { fmtDiscountBps } from '@/utils/money'

// 时长按钮组（不可手输）。单位按 kind：席位=月、包月数=天。
defineProps<{
  options: DurationOption[]
  unit: 'month' | 'day'
}>()

const model = defineModel<number>({ default: 1 })
const { t } = useI18n()
</script>

<template>
  <div class="flex flex-col gap-2.5">
    <span class="text-muted-foreground text-sm">{{ t('billing.purchase2.duration') }}</span>
    <div class="grid grid-cols-3 gap-2">
      <button
        v-for="opt in options"
        :key="opt.value"
        type="button"
        class="relative cursor-pointer rounded-lg border px-3 py-2.5 text-center transition-colors"
        :class="model === opt.value ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
        @click="model = opt.value"
      >
        <span
          v-if="fmtDiscountBps(opt.discount_bps)"
          class="absolute -top-2 -right-1.5 rounded-full bg-red-500 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-white shadow-sm"
        >{{ fmtDiscountBps(opt.discount_bps) }}</span>
        <div class="text-sm font-medium tabular-nums">
          {{ opt.value }} {{ unit === 'month' ? t('billing.purchase2.monthUnit') : t('billing.purchase2.dayUnit') }}
        </div>
      </button>
    </div>
  </div>
</template>
