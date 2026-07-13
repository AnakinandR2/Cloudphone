<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps<{
  class?: HTMLAttributes['class']
}>()
</script>

<template>
  <!-- 图2 风格：展开层浅灰底 + 宽松内边距；内容区由调用方放嵌套表 / Tab -->
  <div
    :class="cn(
      'bg-[#f7f8fa] py-4 pr-6 shadow-[inset_0_8px_10px_-8px_rgb(15_17_26/0.1)]',
      props.class,
    )"
    :style="{ paddingLeft: 'var(--table-expand-indent, 0px)' }"
  >
    <div v-if="$slots.toolbar" class="mb-3 flex flex-wrap items-center gap-2">
      <slot name="toolbar" />
    </div>
    <slot />
  </div>
</template>
