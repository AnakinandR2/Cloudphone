<script setup lang="ts">
import type { AcceptableValue } from 'reka-ui'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  filterPopupAlign,
  filterPopupSideOffset,
  filterSelectContentClass,
  filterSelectTriggerClass,
} from '@/components/filterField'

export interface FilterSelectOption {
  value: string
  label: string
}

const model = defineModel<string>({ default: '' })

withDefaults(
  defineProps<{
    options: FilterSelectOption[]
    placeholder?: string
  }>(),
  { placeholder: '' },
)

function onUpdate(value: AcceptableValue) {
  model.value = value == null ? '' : String(value)
}
</script>

<template>
  <Select :model-value="model || undefined" class="flex h-full min-w-0 flex-1" @update:model-value="onUpdate">
    <SelectTrigger :class="filterSelectTriggerClass">
      <SelectValue :placeholder="placeholder" />
    </SelectTrigger>
    <SelectContent
      :align="filterPopupAlign"
      :side-offset="filterPopupSideOffset"
      :class="filterSelectContentClass"
    >
      <SelectItem v-for="opt in options" :key="opt.value" :value="opt.value">
        {{ opt.label }}
      </SelectItem>
    </SelectContent>
  </Select>
</template>
