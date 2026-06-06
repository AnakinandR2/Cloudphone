<script setup lang="ts">
import { AccordionItem, type AccordionItemProps } from 'reka-ui'
import { cn } from '~/lib/utils'

const props = defineProps<AccordionItemProps & { class?: any }>()
</script>

<template>
  <AccordionItem
    :value="props.value"
    :disabled="props.disabled"
    :class="cn('border-b', props.class)"
  >
    <slot />
  </AccordionItem>
</template>
