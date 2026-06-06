<script setup lang="ts">
import { progressValue, progressVisible } from '@/utils/progress'
</script>

<template>
  <div
    v-show="progressVisible"
    class="pointer-events-none fixed inset-x-0 top-0 z-[100] h-0.5"
  >
    <div
      class="bg-primary h-full transition-[width] duration-150 ease-out"
      :style="{ width: `${progressValue}%`, boxShadow: '0 0 8px var(--primary)' }"
    />
  </div>
</template>
