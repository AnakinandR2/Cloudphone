<script setup lang="ts">
import type { AcceptableValue } from 'reka-ui'
import { CalendarDate, getLocalTimeZone, today } from '@internationalized/date'
import { CalendarIcon } from 'lucide-vue-next'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { filterDateButtonClass, filterMutedTextClass, filterPopupAlign, filterPopupSideOffset } from '@/components/filterField'
import { Button } from '@/components/ui/button'
import { Calendar } from '@/components/ui/calendar'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

const model = defineModel<string>({ default: '' })

withDefaults(
  defineProps<{
    /** 空值时按钮占位文案 */
    placeholder?: string
    /** false = 仅选日期，不含时分 */
    withTime?: boolean
  }>(),
  {
    placeholder: '',
    withTime: true,
  },
)

const { t } = useI18n()
const open = ref(false)
const hourOptions = Array.from({ length: 24 }, (_, i) => String(i).padStart(2, '0'))
const minuteOptions = Array.from({ length: 60 }, (_, i) => String(i).padStart(2, '0'))

function pad2(value: number | string): string {
  return String(value).padStart(2, '0')
}

function parseLocalDateTime(value: string): { date: CalendarDate, hour: string, minute: string } | undefined {
  const match = value.match(/^(\d{4})-(\d{2})-(\d{2})(?:T(\d{2}):(\d{2}))?/)
  if (!match)
    return undefined
  return {
    date: new CalendarDate(Number(match[1]), Number(match[2]), Number(match[3])),
    hour: match[4] ?? '00',
    minute: match[5] ?? '00',
  }
}

function formatDisplay(value: string, withTime: boolean, placeholder: string): string {
  const parsed = parseLocalDateTime(value)
  if (!parsed)
    return placeholder || t('comp.filterDateEmpty')
  const day = `${parsed.date.year}-${pad2(parsed.date.month)}-${pad2(parsed.date.day)}`
  return withTime ? `${day} ${parsed.hour}:${parsed.minute}` : day
}

function writeValue(date: CalendarDate, hour: string, minute: string, withTime: boolean) {
  const day = `${date.year}-${pad2(date.month)}-${pad2(date.day)}`
  model.value = withTime ? `${day}T${hour}:${minute}` : day
}

function currentDate(withTime: boolean): CalendarDate | undefined {
  return parseLocalDateTime(model.value)?.date
}

function currentTime(part: 'hour' | 'minute'): string {
  const parsed = parseLocalDateTime(model.value)
  if (!parsed)
    return '00'
  return part === 'hour' ? parsed.hour : parsed.minute
}

function setDate(value: CalendarDate | undefined, withTime: boolean) {
  if (!value) {
    model.value = ''
    return
  }
  const parsed = parseLocalDateTime(model.value)
  writeValue(value, parsed?.hour ?? '00', parsed?.minute ?? '00', withTime)
  if (!withTime)
    open.value = false
}

function setTime(part: 'hour' | 'minute', value: AcceptableValue, withTime: boolean) {
  const next = value == null ? '00' : String(value)
  const parsed = parseLocalDateTime(model.value)
  const date = parsed?.date ?? today(getLocalTimeZone())
  const hour = part === 'hour' ? next : (parsed?.hour ?? '00')
  const minute = part === 'minute' ? next : (parsed?.minute ?? '00')
  writeValue(date, hour, minute, withTime)
}

function clear() {
  model.value = ''
}
</script>

<template>
  <Popover v-model:open="open">
    <PopoverTrigger as-child>
      <button type="button" :class="filterDateButtonClass">
        <span :class="model ? 'text-foreground' : filterMutedTextClass">
          {{ formatDisplay(model, withTime, placeholder) }}
        </span>
        <CalendarIcon class="size-4 shrink-0" :class="filterMutedTextClass" />
      </button>
    </PopoverTrigger>
    <PopoverContent
      class="w-auto p-0"
      :align="filterPopupAlign"
      :side-offset="filterPopupSideOffset"
    >
      <Calendar
        :model-value="currentDate(withTime)"
        locale="zh-CN"
        initial-focus
        @update:model-value="value => setDate(value as CalendarDate | undefined, withTime)"
      />
      <div v-if="withTime" class="border-border flex items-center gap-2 border-t p-3">
        <Select :model-value="currentTime('hour')" @update:model-value="value => setTime('hour', value, withTime)">
          <SelectTrigger class="h-9 w-20">
            <SelectValue />
          </SelectTrigger>
          <SelectContent class="max-h-64">
            <SelectItem v-for="hour in hourOptions" :key="hour" :value="hour">
              {{ hour }}
            </SelectItem>
          </SelectContent>
        </Select>
        <span class="text-muted-foreground">:</span>
        <Select :model-value="currentTime('minute')" @update:model-value="value => setTime('minute', value, withTime)">
          <SelectTrigger class="h-9 w-20">
            <SelectValue />
          </SelectTrigger>
          <SelectContent class="max-h-64">
            <SelectItem v-for="minute in minuteOptions" :key="minute" :value="minute">
              {{ minute }}
            </SelectItem>
          </SelectContent>
        </Select>
        <Button variant="ghost" size="sm" class="ml-2 h-9" @click="clear">
          {{ t('common.clear') }}
        </Button>
        <Button size="sm" class="h-9" @click="open = false">
          {{ t('common.confirm') }}
        </Button>
      </div>
      <div v-else class="border-border flex justify-end gap-2 border-t p-3">
        <Button variant="ghost" size="sm" class="h-9" @click="clear">
          {{ t('common.clear') }}
        </Button>
        <Button size="sm" class="h-9" @click="open = false">
          {{ t('common.confirm') }}
        </Button>
      </div>
    </PopoverContent>
  </Popover>
</template>
