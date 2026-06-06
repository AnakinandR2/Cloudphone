<script setup lang="ts">
import { cn } from '~/lib/utils'

const props = defineProps<{
  modelValue?: string | number
  class?: any
}>()

const emits = defineEmits<{ (e: 'update:modelValue', value: string | number): void }>()

function onInput(e: Event) {
  emits('update:modelValue', (e.target as HTMLInputElement).value)
}
</script>

<template>
  <input
    :value="modelValue"
    @input="onInput"
    :class="
      cn(
        'flex h-11 w-full rounded-lg border border-input bg-background px-3.5 py-2 text-sm ring-offset-background transition-colors file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
        props.class,
      )
    "
  />
</template>
