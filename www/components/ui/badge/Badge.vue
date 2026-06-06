<script setup lang="ts">
import { cn } from '~/lib/utils'
import { badgeVariants, type BadgeVariants } from '.'

const props = defineProps<{
  variant?: BadgeVariants['variant']
  class?: any
}>()
</script>

<template>
  <div :class="cn(badgeVariants({ variant }), props.class)">
    <slot />
  </div>
</template>
