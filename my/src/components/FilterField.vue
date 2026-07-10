<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { filterFieldShellClass, filterFieldWidthClass } from '@/components/filterField'
import { cn } from '@/lib/utils'

const props = defineProps<{
  label: string
  class?: HTMLAttributes['class']
}>()
</script>

<template>
  <div
    :class="cn(
      filterFieldShellClass,
      filterFieldWidthClass,
      props.class,
    )"
  >
    <span class="bg-muted/30 text-foreground flex h-full shrink-0 items-center rounded-l-[calc(var(--radius-md)-1px)] border-r px-3 text-sm font-medium whitespace-nowrap">
      {{ label }}
    </span>
    <div class="flex h-full min-w-0 flex-1 items-center overflow-hidden rounded-r-[calc(var(--radius-md)-1px)] [&>*]:h-full [&>*]:min-w-0 [&>*]:flex-1">
      <slot />
    </div>
  </div>
</template>
