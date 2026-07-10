<script setup lang="ts">
import { Search, X } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { filterInputClass, filterMutedTextClass } from '@/components/filterField'

const model = defineModel<string>({ default: '' })
defineProps<{
  placeholder?: string
}>()

const { t } = useI18n()

function clear() {
  model.value = ''
}
</script>

<template>
  <div class="relative flex h-full min-w-0 flex-1 items-center">
    <input
      v-model="model"
      type="text"
      :class="[filterInputClass, 'pr-9']"
      :placeholder="placeholder"
    >
    <button
      v-if="model"
      type="button"
      class="text-muted-foreground hover:text-foreground absolute right-2 inline-flex size-6 items-center justify-center rounded-sm transition-colors"
      :aria-label="t('common.clear')"
      @click="clear"
    >
      <X class="size-3.5" />
    </button>
    <Search
      v-else
      class="pointer-events-none absolute right-2.5 size-4 shrink-0"
      :class="filterMutedTextClass"
      aria-hidden="true"
    />
  </div>
</template>
