<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

// `interactive` = small / clickable card: adds a subtle hover offset.
// Default (big card): theme-color glow + accent border on hover, no offset.
const props = defineProps<{ class?: HTMLAttributes['class'], interactive?: boolean }>()
</script>

<template>
  <div
    data-slot="card"
    :class="cn(
      'bg-card text-card-foreground flex flex-col gap-6 rounded-[1.25rem] border py-7 shadow-[var(--shadow-sm)] transition-all duration-300 hover:border-[var(--card-hover-border)] hover:shadow-[var(--shadow-card-hover)]',
      interactive && 'hover:-translate-y-1',
      props.class,
    )"
  >
    <slot />
  </div>
</template>
