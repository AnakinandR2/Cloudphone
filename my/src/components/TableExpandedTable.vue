<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps<{
  class?: HTMLAttributes['class']
}>()
</script>

<template>
  <!-- 图2 嵌套表：仅横线分隔，无竖线/外框 -->
  <div :class="cn('w-full overflow-x-auto', props.class)">
    <table class="w-full min-w-[640px] border-collapse text-sm">
      <thead v-if="$slots.head">
        <tr class="border-b border-[#e5e6eb] text-left text-[#646a73]">
          <slot name="head" />
        </tr>
      </thead>
      <tbody class="text-[#1f2329] [&_tr]:border-b [&_tr]:border-[#e5e6eb] [&_tr:last-child]:border-b-0">
        <slot />
      </tbody>
    </table>
  </div>
</template>
