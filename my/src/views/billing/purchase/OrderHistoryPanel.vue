<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { Order2, OrderItem2 } from '@/types/billing'
import { CalendarDate, getLocalTimeZone, today } from '@internationalized/date'
import { CalendarIcon } from 'lucide-vue-next'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import billingApi from '@/api/modules/billing'
import DataTable from '@/components/DataTable.vue'
import FilterBar from '@/components/FilterBar.vue'
import FilterField from '@/components/FilterField.vue'
import FilterSearchInput from '@/components/FilterSearchInput.vue'
import {
  filterDateButtonClass,
  filterMutedTextClass,
  filterPopupAlign,
  filterPopupSideOffset,
  filterSelectContentClass,
  filterSelectTriggerClass,
} from '@/components/filterField'
import TableExpanded from '@/components/TableExpanded.vue'
import TableExpandedTable from '@/components/TableExpandedTable.vue'
import { Badge } from '@/components/ui/badge'
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
import { ACTIONS_COLUMN_META } from '@/lib/table'
import { formatDateTime } from '@/utils/date'
import { fmtCents, fmtDiscountBps } from '@/utils/money'
import { orderStatusBadge, STATUS_BADGE_BASE } from '@/utils/statusBadge'

// 默认 tab：全部订单 — 订单 ID / 类型 / 明细 / 金额 / 状态 / 时间；未支付可继续支付。
// 明细按订单项派生摘要（数量 + 时长），可展开看完整明细；支持按状态 + 创建时间区间过滤（服务端）。
const emit = defineEmits<{ paid: [] }>()
const props = defineProps<{
  bizType?: string
  statusTone?: 'default' | 'primary'
}>()
const { t } = useI18n()

const data = ref<Order2[]>([])
const loading = ref(false)
const payingId = ref<number | null>(null)
// status：服务端过滤；from/to：本地时间字符串，查询时转 ISO；q：客户端搜索。
const filters = reactive({ status: 'all', from: '', to: '', q: '' })
const datePickerOpen = reactive({ from: false, to: false })
const hourOptions = Array.from({ length: 24 }, (_, i) => String(i).padStart(2, '0'))
const minuteOptions = Array.from({ length: 60 }, (_, i) => String(i).padStart(2, '0'))

type DateFilterKey = 'from' | 'to'

function pad2(value: number | string): string {
  return String(value).padStart(2, '0')
}

function parseLocalDateTime(value: string): { date: CalendarDate, hour: string, minute: string } | undefined {
  const match = value.match(/^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})/)
  if (!match) return undefined
  return {
    date: new CalendarDate(Number(match[1]), Number(match[2]), Number(match[3])),
    hour: match[4],
    minute: match[5],
  }
}

function getDateValue(key: DateFilterKey): CalendarDate | undefined {
  return parseLocalDateTime(filters[key])?.date
}

function getTimeValue(key: DateFilterKey, part: 'hour' | 'minute'): string {
  const parsed = parseLocalDateTime(filters[key])
  if (!parsed) return '00'
  return part === 'hour' ? parsed.hour : parsed.minute
}

function writeLocalDateTime(key: DateFilterKey, date: CalendarDate, hour: string, minute: string) {
  filters[key] = `${date.year}-${pad2(date.month)}-${pad2(date.day)}T${hour}:${minute}`
}

function setDateValue(key: DateFilterKey, value: CalendarDate | undefined) {
  if (!value) {
    filters[key] = ''
    return
  }
  const parsed = parseLocalDateTime(filters[key])
  writeLocalDateTime(key, value, parsed?.hour ?? '00', parsed?.minute ?? '00')
}

function setTimeValue(key: DateFilterKey, part: 'hour' | 'minute', value: string) {
  const parsed = parseLocalDateTime(filters[key])
  const date = parsed?.date ?? today(getLocalTimeZone())
  const hour = part === 'hour' ? value : (parsed?.hour ?? '00')
  const minute = part === 'minute' ? value : (parsed?.minute ?? '00')
  writeLocalDateTime(key, date, hour, minute)
}

function clearDateValue(key: DateFilterKey) {
  filters[key] = ''
}

function formatLocalDateTime(value: string): string {
  const parsed = parseLocalDateTime(value)
  if (!parsed) return '年/月/日 --:--'
  return `${parsed.date.year}-${pad2(parsed.date.month)}-${pad2(parsed.date.day)} ${parsed.hour}:${parsed.minute}`
}

// "2026-06-21T23:00"（本地）→ ISO；空串返回 undefined（不传该参数）。
function toIso(local: string): string | undefined {
  if (!local) return undefined
  const d = new Date(local)
  return Number.isNaN(d.getTime()) ? undefined : d.toISOString()
}

function badgeForStatus(status: string) {
  const base = orderStatusBadge(status)
  if (props.statusTone === 'primary' && status === 'paid') {
    return {
      ...base,
      class: `${STATUS_BADGE_BASE} bg-[var(--primary-light-bg)] text-primary dark:bg-primary/15 dark:text-primary`,
    }
  }
  return base
}

async function load() {
  loading.value = true
  try {
    const params: { page: number, size: number, status?: string, from?: string, to?: string, biz_type?: string } = { page: 1, size: 200 }
    if (filters.status !== 'all') params.status = filters.status
    if (props.bizType) params.biz_type = props.bizType
    params.from = toIso(filters.from)
    params.to = toIso(filters.to)
    const res = await billingApi.orders2(params)
    data.value = res.data.list
  }
  finally {
    loading.value = false
  }
}

onMounted(load)
// 状态切换即时生效；时间区间走「应用」按钮（避免半填触发）。
watch(() => filters.status, load)
defineExpose({ reload: load })

function applyTimeFilter() {
  load()
}
function resetFilters() {
  filters.q = ''
  filters.status = 'all'
  filters.from = ''
  filters.to = ''
  load()
}
const hasTimeFilter = computed(() => !!filters.from || !!filters.to)
const hasAnyFilter = computed(() => !!filters.q || filters.status !== 'all' || hasTimeFilter.value)
const displayData = computed(() => {
  if (!props.bizType) return data.value
  return data.value.filter(item => item.biz_type === props.bizType)
})

const statusOptions = ['all', 'unpaid', 'paid', 'expired'] as const

const columns = computed<ColumnDef<Order2>[]>(() => [
  { id: 'expander', header: '', meta: { label: 'billing.purchase2.colDetail' } },
  { accessorKey: 'id', id: 'id', header: t('billing.purchase2.colOrderId'), meta: { label: 'billing.purchase2.colOrderId' } },
  { accessorKey: 'biz_type', id: 'biz_type', header: t('billing.purchase2.colBizType'), meta: { label: 'billing.purchase2.colBizType' } },
  { id: 'detail', header: t('billing.purchase2.colDetail'), meta: { label: 'billing.purchase2.colDetail' } },
  { accessorKey: 'total_cents', id: 'total_cents', header: t('billing.purchase2.colAmount'), meta: { label: 'billing.purchase2.colAmount' } },
  { accessorKey: 'status', id: 'status', header: t('billing.purchase2.colStatus'), meta: { label: 'billing.purchase2.colStatus' } },
  { accessorKey: 'created_at', id: 'created_at', header: t('billing.purchase2.colCreatedAt'), meta: { label: 'billing.purchase2.colCreatedAt' } },
  { id: 'actions', header: t('crud.actions'), enableHiding: false, meta: ACTIONS_COLUMN_META },
])

function kindLabel(kind: string): string {
  return t(`billing.purchase2.kind_${kind}`)
}
function durUnit(unit: string): string {
  if (unit === 'month') return t('billing.purchase2.monthUnit')
  if (unit === 'day') return t('billing.purchase2.dayUnit')
  return ''
}

// 单个订单项摘要：席位/包月数=数量·时长；临时时长=分钟数；充值=金额。
function itemSummary(it: OrderItem2): string {
  if (it.target_kind === 'balance') {
    return `${kindLabel('balance')} ¥${fmtCents(it.amount_cents)}`
  }
  if (it.target_kind === 'runtime_minute') {
    return `${kindLabel('runtime_minute')} ×${it.quantity}${t('billing.purchase2.minuteUnit')}`
  }
  const dur = it.duration_value ? ` · ${it.duration_value}${durUnit(it.duration_unit)}` : ''
  return `${kindLabel(it.target_kind)} ×${it.quantity}${dur}`
}

// 整单摘要：各订单项以「；」连接；席位赠送时长追加在末尾。
function orderSummary(o: Order2): string {
  const parts = (o.items ?? []).map(itemSummary)
  let s = parts.join('；')
  if (o.gift_runtime_minutes > 0) {
    s += `${parts.length ? ' · ' : ''}${t('billing.purchase2.giftShort', { min: o.gift_runtime_minutes })}`
  }
  return s || '—'
}

async function continuePay(o: Order2) {
  payingId.value = o.id
  try {
    await billingApi.payOrder2(o.id)
    const { toast } = await import('vue-sonner')
    toast.success(t('billing.purchase2.orderOk'))
    await load()
    emit('paid')
  }
  catch {
    // 失败由响应拦截器统一提示
  }
  finally {
    payingId.value = null
  }
}
</script>

<template>
  <DataTable
    v-model:search-value="filters.q"
    pin-actions-column
    :columns="columns"
    :data="displayData"
    :loading="loading"
    :search="false"
    expandable
    :get-row-id="(o: Order2) => String(o.id)"
  >
    <!-- 过滤栏：搜索 + 状态 + 创建时间区间（仅调整视觉样式，筛选逻辑保持不变） -->
    <template #filters>
      <FilterBar class="min-w-0 flex-1">
        <FilterField :label="t('billing.purchase2.filterSearch')">
          <FilterSearchInput v-model="filters.q" :placeholder="t('billing.purchase2.searchOrder')" />
        </FilterField>

        <FilterField :label="t('billing.purchase2.filterStatus')">
          <Select v-model="filters.status" class="flex h-full min-w-0 flex-1">
            <SelectTrigger :class="filterSelectTriggerClass">
              <SelectValue />
            </SelectTrigger>
            <SelectContent
              :align="filterPopupAlign"
              :side-offset="filterPopupSideOffset"
              :class="filterSelectContentClass"
            >
              <SelectItem v-for="s in statusOptions" :key="s" :value="s">
                {{ s === 'all' ? t('billing.purchase2.filterAll') : t(`billing.purchase2.status_${s}`) }}
              </SelectItem>
            </SelectContent>
          </Select>
        </FilterField>

        <FilterField :label="t('billing.purchase2.filterFrom')">
          <Popover v-model:open="datePickerOpen.from">
            <PopoverTrigger as-child>
              <button type="button" :class="filterDateButtonClass">
                <span :class="filters.from ? 'text-foreground' : filterMutedTextClass">{{ formatLocalDateTime(filters.from) }}</span>
                <CalendarIcon class="size-4 shrink-0" :class="filterMutedTextClass" />
              </button>
            </PopoverTrigger>
            <PopoverContent
              class="w-auto p-0"
              :align="filterPopupAlign"
              :side-offset="filterPopupSideOffset"
            >
              <Calendar
                :model-value="getDateValue('from')"
                locale="zh-CN"
                initial-focus
                @update:model-value="value => setDateValue('from', value as CalendarDate | undefined)"
              />
              <div class="border-border flex items-center gap-2 border-t p-3">
                <Select :model-value="getTimeValue('from', 'hour')" @update:model-value="value => setTimeValue('from', 'hour', String(value))">
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
                <Select :model-value="getTimeValue('from', 'minute')" @update:model-value="value => setTimeValue('from', 'minute', String(value))">
                  <SelectTrigger class="h-9 w-20">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent class="max-h-64">
                    <SelectItem v-for="minute in minuteOptions" :key="minute" :value="minute">
                      {{ minute }}
                    </SelectItem>
                  </SelectContent>
                </Select>
                <Button variant="ghost" size="sm" class="ml-2 h-9" @click="clearDateValue('from')">
                  清除
                </Button>
                <Button size="sm" class="h-9" @click="datePickerOpen.from = false">
                  确定
                </Button>
              </div>
            </PopoverContent>
          </Popover>
        </FilterField>

        <FilterField :label="t('billing.purchase2.filterTo')">
          <Popover v-model:open="datePickerOpen.to">
            <PopoverTrigger as-child>
              <button type="button" :class="filterDateButtonClass">
                <span :class="filters.to ? 'text-foreground' : filterMutedTextClass">{{ formatLocalDateTime(filters.to) }}</span>
                <CalendarIcon class="size-4 shrink-0" :class="filterMutedTextClass" />
              </button>
            </PopoverTrigger>
            <PopoverContent
              class="w-auto p-0"
              :align="filterPopupAlign"
              :side-offset="filterPopupSideOffset"
            >
              <Calendar
                :model-value="getDateValue('to')"
                locale="zh-CN"
                initial-focus
                @update:model-value="value => setDateValue('to', value as CalendarDate | undefined)"
              />
              <div class="border-border flex items-center gap-2 border-t p-3">
                <Select :model-value="getTimeValue('to', 'hour')" @update:model-value="value => setTimeValue('to', 'hour', String(value))">
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
                <Select :model-value="getTimeValue('to', 'minute')" @update:model-value="value => setTimeValue('to', 'minute', String(value))">
                  <SelectTrigger class="h-9 w-20">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent class="max-h-64">
                    <SelectItem v-for="minute in minuteOptions" :key="minute" :value="minute">
                      {{ minute }}
                    </SelectItem>
                  </SelectContent>
                </Select>
                <Button variant="ghost" size="sm" class="ml-2 h-9" @click="clearDateValue('to')">
                  清除
                </Button>
                <Button size="sm" class="h-9" @click="datePickerOpen.to = false">
                  确定
                </Button>
              </div>
            </PopoverContent>
          </Popover>
        </FilterField>
      </FilterBar>
    </template>

    <template #actions>
      <Button size="sm" class="h-10 min-w-18 rounded-md px-5" @click="applyTimeFilter">
        {{ t('billing.purchase2.filterApply') }}
      </Button>
      <Button
        variant="outline"
        size="sm"
        class="h-10 min-w-18 rounded-md px-5"
        :disabled="!hasAnyFilter"
        @click="resetFilters"
      >
        {{ t('billing.purchase2.filterReset') }}
      </Button>
    </template>

    <template #cell-id="{ row }">
      <span class="font-mono text-sm">#{{ row.id }}</span>
    </template>
    <template #cell-biz_type="{ row }">
      <span>{{ t(`billing.purchase2.biz_${row.biz_type}`) }}</span>
    </template>
    <template #cell-detail="{ row }">
      <span class="text-muted-foreground text-sm">{{ orderSummary(row) }}</span>
    </template>
    <template #cell-total_cents="{ row }">
      <div class="flex flex-col">
        <span class="tabular-nums">¥{{ fmtCents(row.total_cents) }}</span>
        <span v-if="row.fee_cents > 0" class="text-muted-foreground text-sm tabular-nums">
          {{ t('billing.purchase2.sumPayActual') }} ¥{{ fmtCents(row.total_cents + row.fee_cents) }}
        </span>
      </div>
    </template>
    <template #cell-status="{ row }">
      <Badge v-bind="badgeForStatus(row.status)" class="text-sm leading-5">
        {{ t(`billing.purchase2.status_${row.status}`) }}
      </Badge>
    </template>
    <template #cell-created_at="{ row }">
      <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.created_at) }}</span>
    </template>
    <template #cell-actions="{ row }">
      <div class="flex items-center justify-end gap-2">
        <Button
          v-if="row.status === 'unpaid'"
          size="sm"
          :disabled="payingId === row.id"
          @click="continuePay(row)"
        >
          {{ payingId === row.id ? t('billing.purchase2.submitting') : t('billing.purchase2.continuePay') }}
        </Button>
      </div>
    </template>

    <!-- 行展开：完整订单项明细 + 赠送时长（图2 嵌套横线表） -->
    <template #expanded="{ row }">
      <TableExpanded>
        <TableExpandedTable>
          <template #head>
            <th class="py-2.5 pr-6 font-normal whitespace-nowrap">
              {{ t('billing.purchase2.dColKind') }}
            </th>
            <th class="py-2.5 pr-6 text-right font-normal whitespace-nowrap">
              {{ t('billing.purchase2.dColQty') }}
            </th>
            <th class="py-2.5 pr-6 text-right font-normal whitespace-nowrap">
              {{ t('billing.purchase2.dColDuration') }}
            </th>
            <th class="py-2.5 pr-6 text-right font-normal whitespace-nowrap">
              {{ t('billing.purchase2.dColUnitPrice') }}
            </th>
            <th class="py-2.5 text-right font-normal whitespace-nowrap">
              {{ t('billing.purchase2.dColAmount') }}
            </th>
          </template>
          <tr
            v-for="it in (row.items ?? [])"
            :key="it.id"
          >
            <td class="py-3 pr-6 whitespace-nowrap">
              {{ kindLabel(it.target_kind) }}
            </td>
            <td class="py-3 pr-6 text-right tabular-nums whitespace-nowrap">
              {{ it.quantity }}
            </td>
            <td class="py-3 pr-6 text-right tabular-nums whitespace-nowrap">
              <template v-if="it.duration_value">{{ it.duration_value }}{{ durUnit(it.duration_unit) }}</template>
              <span v-else class="text-[#8f959e]">—</span>
            </td>
            <td class="py-3 pr-6 text-right tabular-nums whitespace-nowrap">
              ¥{{ fmtCents(it.unit_price_cents) }}
            </td>
            <td class="py-3 text-right tabular-nums whitespace-nowrap">
              <span v-if="it.qty_discount_bps < 10000 || it.duration_discount_bps < 10000" class="mr-1 text-xs text-red-500">
                {{ fmtDiscountBps(it.qty_discount_bps) }}<span v-if="it.qty_discount_bps < 10000 && it.duration_discount_bps < 10000">×</span>{{ it.duration_discount_bps < 10000 ? fmtDiscountBps(it.duration_discount_bps) : '' }}
              </span>
              ¥{{ fmtCents(it.amount_cents) }}
            </td>
          </tr>
        </TableExpandedTable>
        <!-- 手续费 / 实付小结：仅有手续费时展示（历史/旧订单为 0 时隐藏）。 -->
        <div v-if="row.fee_cents > 0" class="mt-3 flex flex-col items-end gap-0.5 text-sm">
          <div class="flex w-full max-w-xs items-baseline justify-between">
            <span class="text-[#8f959e]">{{ t('billing.purchase2.sumTotal') }}</span>
            <span class="tabular-nums">¥{{ fmtCents(row.total_cents) }}</span>
          </div>
          <div class="flex w-full max-w-xs items-baseline justify-between">
            <span class="text-[#8f959e]">{{ t('billing.purchase2.sumFee') }}</span>
            <span class="tabular-nums">¥{{ fmtCents(row.fee_cents) }}</span>
          </div>
          <div class="flex w-full max-w-xs items-baseline justify-between">
            <span class="font-medium">{{ t('billing.purchase2.sumPayActual') }}</span>
            <span class="font-semibold tabular-nums text-red-600">¥{{ fmtCents(row.total_cents + row.fee_cents) }}</span>
          </div>
        </div>
        <div v-if="row.gift_runtime_minutes > 0" class="mt-2 text-xs text-emerald-600 dark:text-emerald-400">
          {{ t('billing.purchase2.giftLine', { min: row.gift_runtime_minutes }) }}
        </div>
      </TableExpanded>
    </template>
  </DataTable>
</template>
