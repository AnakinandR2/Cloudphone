<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps<{
  class?: HTMLAttributes['class']
}>()
</script>

<template>
    <div :class="cn('flex min-w-0 flex-wrap items-center gap-3 py-[3px]', props.class)">
    <slot />
  </div>
</template>
