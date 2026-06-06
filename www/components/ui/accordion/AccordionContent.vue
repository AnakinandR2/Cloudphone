<script setup lang="ts">
import { AccordionContent } from 'reka-ui'
import { cn } from '~/lib/utils'

const props = defineProps<{ class?: any }>()
</script>

<template>
  <AccordionContent
    class="overflow-hidden text-sm data-[state=closed]:animate-accordion-up data-[state=open]:animate-accordion-down"
  >
    <div :class="cn('pb-5 pt-0 text-[15px] leading-relaxed text-muted-foreground', props.class)">
      <slot />
    </div>
  </AccordionContent>
</template>
