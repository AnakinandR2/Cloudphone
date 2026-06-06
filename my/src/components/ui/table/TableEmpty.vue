<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const props = withDefaults(defineProps<{ class?: HTMLAttributes['class']; colspan?: number }>(), {
  colspan: 1,
})
</script>

<template>
  <tr>
    <td :colspan="props.colspan" :class="cn('text-muted-foreground p-4 text-center align-middle text-sm', props.class)">
      <slot />
    </td>
  </tr>
</template>
