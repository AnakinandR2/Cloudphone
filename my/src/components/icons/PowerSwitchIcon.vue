<!-- 电源开关：直接使用 assets/icons/power-switch.svg（由用户提供的图标导出）。 -->
<script setup lang="ts">
import powerSwitchSvg from '@/assets/icons/power-switch.svg?url'

defineOptions({ inheritAttrs: false })
</script>

<template>
  <span
    class="inline-block size-4 shrink-0 bg-current"
    :style="{
      WebkitMaskImage: `url(${powerSwitchSvg})`,
      maskImage: `url(${powerSwitchSvg})`,
      WebkitMaskSize: 'contain',
      maskSize: 'contain',
      WebkitMaskRepeat: 'no-repeat',
      maskRepeat: 'no-repeat',
      WebkitMaskPosition: 'center',
      maskPosition: 'center',
    }"
    aria-hidden="true"
    v-bind="$attrs"
  />
</template>
