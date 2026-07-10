<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

// `interactive` is reserved for small clickable cards.
// Default cards are static page containers and should not react on hover.
const props = defineProps<{ class?: HTMLAttributes['class'], interactive?: boolean }>()
</script>

<template>
  <div
    data-slot="card"
    :class="cn(
      'bg-card text-card-foreground flex flex-col gap-6 rounded-[1.25rem] border py-7 shadow-[var(--shadow-sm)]',
      interactive && 'transition-[transform,box-shadow,border-color] duration-200 hover:-translate-y-0.5 hover:border-[var(--border-strong)] hover:shadow-[var(--shadow-md)]',
      props.class,
    )"
  >
    <slot />
  </div>
</template>
