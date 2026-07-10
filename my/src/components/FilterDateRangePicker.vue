<script setup lang="ts">
import type { DateRange } from 'reka-ui'
import { CalendarDate } from '@internationalized/date'
import { CalendarIcon } from 'lucide-vue-next'
import { toDate } from 'reka-ui/date'
import { useDateFormatter } from 'reka-ui'
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { filterDateButtonClass, filterMutedTextClass, filterPopupAlign, filterPopupSideOffset } from '@/components/filterField'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { RangeCalendar } from '@/components/ui/range-calendar'

export interface FilterDateRangeValue {
  from: string
  to: string
}

const model = defineModel<FilterDateRangeValue>({
  default: () => ({ from: '', to: '' }),
})

withDefaults(
  defineProps<{
    placeholder?: string
  }>(),
  { placeholder: '' },
)

const { t, locale } = useI18n()
const open = ref(false)
const draftRange = ref<DateRange>({ start: undefined, end: undefined })
const dateFormatter = useDateFormatter(locale.value)

function pad2(value: number | string): string {
  return String(value).padStart(2, '0')
}

function parseDate(value: string): CalendarDate | undefined {
  const match = value.match(/^(\d{4})-(\d{2})-(\d{2})/)
  if (!match)
    return undefined
  return new CalendarDate(Number(match[1]), Number(match[2]), Number(match[3]))
}

function formatDate(date: CalendarDate): string {
  return `${date.year}-${pad2(date.month)}-${pad2(date.day)}`
}

function formatDisplayDate(value: string): string {
  const date = parseDate(value)
  if (!date)
    return '…'
  return dateFormatter.custom(toDate(date), {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

function formatDisplay(value: FilterDateRangeValue, placeholder: string): string {
  if (!value.from && !value.to)
    return placeholder || t('comp.filterDateRangeEmpty')

  const from = value.from ? formatDisplayDate(value.from) : '…'
  const to = value.to ? formatDisplayDate(value.to) : '…'
  return `${from} - ${to}`
}

function syncDraftFromModel() {
  draftRange.value = {
    start: parseDate(model.value.from),
    end: parseDate(model.value.to),
  }
}

function clear() {
  draftRange.value = { start: undefined, end: undefined }
  model.value = { from: '', to: '' }
}

function confirm() {
  model.value = {
    from: draftRange.value.start ? formatDate(draftRange.value.start) : '',
    to: draftRange.value.end ? formatDate(draftRange.value.end) : '',
  }
  open.value = false
}

watch(open, (isOpen) => {
  if (isOpen)
    syncDraftFromModel()
})
</script>

<template>
  <Popover v-model:open="open">
    <PopoverTrigger as-child>
      <button type="button" :class="filterDateButtonClass">
        <span :class="model.from || model.to ? 'text-foreground' : filterMutedTextClass">
          {{ formatDisplay(model, placeholder) }}
        </span>
        <CalendarIcon class="size-4 shrink-0" :class="filterMutedTextClass" />
      </button>
    </PopoverTrigger>
    <PopoverContent
      class="w-auto p-0"
      :align="filterPopupAlign"
      :side-offset="filterPopupSideOffset"
    >
      <RangeCalendar
        v-model="draftRange"
        locale="zh-CN"
        :number-of-months="2"
        disable-days-outside-current-view
        initial-focus
      />
      <div class="border-border flex justify-end gap-2 border-t p-3">
        <Button variant="ghost" size="sm" class="h-9" @click="clear">
          {{ t('common.clear') }}
        </Button>
        <Button size="sm" class="h-9" @click="confirm">
          {{ t('common.confirm') }}
        </Button>
      </div>
    </PopoverContent>
  </Popover>
</template>
