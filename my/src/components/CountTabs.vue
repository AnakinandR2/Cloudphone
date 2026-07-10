<script setup lang="ts">
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

export interface CountTabItem {
  value: string
  label: string
  count?: number
}

const model = defineModel<string>({ required: true })

const props = withDefaults(
  defineProps<{
    items: CountTabItem[]
    countCap?: number
  }>(),
  { countCap: 99 },
)

function displayCount(count?: number): string | null {
  if (count == null || count <= 0) return null
  if (count > props.countCap) return `${props.countCap}+`
  return String(count)
}
</script>

<template>
  <Tabs v-model="model">
    <TabsList class="h-auto gap-2">
      <TabsTrigger
        v-for="item in items"
        :key="item.value"
        :value="item.value"
        class="h-auto flex-none gap-1.5 px-3 py-1.5"
      >
        {{ item.label }}
        <span v-if="displayCount(item.count)" class="text-primary tabular-nums">
          {{ displayCount(item.count) }}
        </span>
      </TabsTrigger>
    </TabsList>
  </Tabs>
</template>
