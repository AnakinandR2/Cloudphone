<script setup lang="ts">
import { computed } from 'vue'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

const props = withDefaults(
  defineProps<{
    text?: string | null
    /** 表格展示最大字数（按 Unicode 码点计，中英文均计 1） */
    max?: number
    class?: string
    empty?: string
  }>(),
  { max: 8, empty: '-' },
)

const full = computed(() => (props.text ?? '').trim() ? String(props.text) : '')
const chars = computed(() => [...full.value])
const truncated = computed(() => chars.value.length > props.max)
const display = computed(() => {
  if (!full.value)
    return props.empty
  if (!truncated.value)
    return full.value
  return `${chars.value.slice(0, props.max).join('')}…`
})
</script>

<template>
  <TooltipProvider v-if="truncated" :delay-duration="200">
    <Tooltip>
      <TooltipTrigger as-child>
        <span :class="props.class">{{ display }}</span>
      </TooltipTrigger>
      <TooltipContent class="max-w-xs break-words text-xs">
        {{ full }}
      </TooltipContent>
    </Tooltip>
  </TooltipProvider>
  <span v-else :class="props.class">{{ display }}</span>
</template>
