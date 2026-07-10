<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps<{ class?: HTMLAttributes['class'] }>()
</script>

<template>
  <div data-slot="card-title" :class="cn('text-2xl leading-none font-semibold', props.class)">
    <slot />
  </div>
</template>
