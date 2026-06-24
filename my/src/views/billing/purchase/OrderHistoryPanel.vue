<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { Order2, OrderItem2 } from '@/types/billing'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import billingApi from '@/api/modules/billing'
import DataTable from '@/components/DataTable.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { formatDateTime } from '@/utils/date'
import { fmtCents, fmtDiscountBps } from '@/utils/money'

// 默认 tab：全部订单 — 订单 ID / 类型 / 明细 / 金额 / 状态 / 时间；未支付可继续支付。
// 明细按订单项派生摘要（数量 + 时长），可展开看完整明细；支持按状态 + 创建时间区间过滤（服务端）。
const emit = defineEmits<{ paid: [] }>()
const { t } = useI18n()

const data = ref<Order2[]>([])
const loading = ref(false)
const payingId = ref<number | null>(null)
// status：服务端过滤；from/to：datetime-local 字符串（本地时区），查询时转 ISO；q：客户端搜索。
const filters = reactive({ status: 'all', from: '', to: '', q: '' })

// "2026-06-21T23:00"（本地）→ ISO；空串返回 undefined（不传该参数）。
function toIso(local: string): string | undefined {
  if (!local) return undefined
  const d = new Date(local)
  return Number.isNaN(d.getTime()) ? undefined : d.toISOString()
}

async function load() {
  loading.value = true
  try {
    const params: { page: number, size: number, status?: string, from?: string, to?: string } = { page: 1, size: 200 }
    if (filters.status !== 'all') params.status = filters.status
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
function resetTimeFilter() {
  filters.from = ''
  filters.to = ''
  load()
}
const hasTimeFilter = computed(() => !!filters.from || !!filters.to)

const statusOptions = ['all', 'unpaid', 'paid', 'expired'] as const

const columns = computed<ColumnDef<Order2>[]>(() => [
  { id: 'expander', header: '', meta: { label: 'billing.purchase2.colDetail' } },
  { accessorKey: 'id', id: 'id', header: t('billing.purchase2.colOrderId'), meta: { label: 'billing.purchase2.colOrderId' } },
  { accessorKey: 'biz_type', id: 'biz_type', header: t('billing.purchase2.colBizType'), meta: { label: 'billing.purchase2.colBizType' } },
  { id: 'detail', header: t('billing.purchase2.colDetail'), meta: { label: 'billing.purchase2.colDetail' } },
  { accessorKey: 'total_cents', id: 'total_cents', header: t('billing.purchase2.colAmount'), meta: { label: 'billing.purchase2.colAmount' } },
  { accessorKey: 'status', id: 'status', header: t('billing.purchase2.colStatus'), meta: { label: 'billing.purchase2.colStatus' } },
  { accessorKey: 'created_at', id: 'created_at', header: t('billing.purchase2.colCreatedAt'), meta: { label: 'billing.purchase2.colCreatedAt' } },
  { id: 'actions', header: '', meta: { label: 'billing.purchase2.colActions' } },
])

function statusVariant(s: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  if (s === 'paid') return 'default'
  if (s === 'unpaid') return 'secondary'
  return 'outline'
}

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
    :columns="columns"
    :data="data"
    :loading="loading"
    :search="false"
    expandable
    :get-row-id="(o: Order2) => String(o.id)"
  >
    <!-- 过滤栏：搜索 + 状态 + 创建时间区间（搜索框由本组件带 Label 渲染，故关闭 DataTable 内建搜索） -->
    <template #filters>
      <div class="flex flex-wrap items-end gap-2">
        <div class="flex flex-col gap-1">
          <label class="text-muted-foreground text-xs">{{ t('billing.purchase2.filterSearch') }}</label>
          <input
            v-model="filters.q"
            type="text"
            class="border-input bg-background h-9 w-64 rounded-md border px-2.5 text-sm"
            :placeholder="t('billing.purchase2.searchOrder')"
          >
        </div>
        <div class="flex flex-col gap-1">
          <label class="text-muted-foreground text-xs">{{ t('billing.purchase2.colStatus') }}</label>
          <Select v-model="filters.status">
            <SelectTrigger class="h-9 w-32">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="s in statusOptions" :key="s" :value="s">
                {{ s === 'all' ? t('billing.purchase2.filterAll') : t(`billing.purchase2.status_${s}`) }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="flex flex-col gap-1">
          <label class="text-muted-foreground text-xs">{{ t('billing.purchase2.filterFrom') }}</label>
          <input
            v-model="filters.from"
            type="datetime-local"
            class="border-input bg-background h-9 rounded-md border px-2.5 text-sm"
          >
        </div>
        <div class="flex flex-col gap-1">
          <label class="text-muted-foreground text-xs">{{ t('billing.purchase2.filterTo') }}</label>
          <input
            v-model="filters.to"
            type="datetime-local"
            class="border-input bg-background h-9 rounded-md border px-2.5 text-sm"
          >
        </div>
        <Button size="sm" class="h-9" @click="applyTimeFilter">
          {{ t('billing.purchase2.filterApply') }}
        </Button>
        <Button v-if="hasTimeFilter" variant="ghost" size="sm" class="h-9" @click="resetTimeFilter">
          {{ t('billing.purchase2.filterReset') }}
        </Button>
      </div>
    </template>

    <template #cell-id="{ row }">
      <span class="font-mono text-xs">#{{ row.id }}</span>
    </template>
    <template #cell-biz_type="{ row }">
      <span>{{ t(`billing.purchase2.biz_${row.biz_type}`) }}</span>
    </template>
    <template #cell-detail="{ row }">
      <span class="text-muted-foreground text-xs">{{ orderSummary(row) }}</span>
    </template>
    <template #cell-total_cents="{ row }">
      <div class="flex flex-col">
        <span class="tabular-nums">¥{{ fmtCents(row.total_cents) }}</span>
        <span v-if="row.fee_cents > 0" class="text-muted-foreground text-xs tabular-nums">
          {{ t('billing.purchase2.sumPayActual') }} ¥{{ fmtCents(row.total_cents + row.fee_cents) }}
        </span>
      </div>
    </template>
    <template #cell-status="{ row }">
      <Badge :variant="statusVariant(row.status)">
        {{ t(`billing.purchase2.status_${row.status}`) }}
      </Badge>
    </template>
    <template #cell-created_at="{ row }">
      <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.created_at) }}</span>
    </template>
    <template #cell-actions="{ row }">
      <Button
        v-if="row.status === 'unpaid'"
        size="sm"
        :disabled="payingId === row.id"
        @click="continuePay(row)"
      >
        {{ payingId === row.id ? t('billing.purchase2.submitting') : t('billing.purchase2.continuePay') }}
      </Button>
    </template>

    <!-- 行展开：完整订单项明细 + 赠送时长 -->
    <template #expanded="{ row }">
      <div class="px-4 py-3">
        <div class="overflow-hidden rounded-md border">
          <div class="bg-muted/50 text-muted-foreground grid grid-cols-[1.4fr_1fr_1fr_1fr_1fr] gap-2 px-3 py-2 text-xs font-medium">
            <span>{{ t('billing.purchase2.dColKind') }}</span>
            <span class="text-right">{{ t('billing.purchase2.dColQty') }}</span>
            <span class="text-right">{{ t('billing.purchase2.dColDuration') }}</span>
            <span class="text-right">{{ t('billing.purchase2.dColUnitPrice') }}</span>
            <span class="text-right">{{ t('billing.purchase2.dColAmount') }}</span>
          </div>
          <div
            v-for="it in (row.items ?? [])"
            :key="it.id"
            class="grid grid-cols-[1.4fr_1fr_1fr_1fr_1fr] items-center gap-2 border-t px-3 py-2 text-sm"
          >
            <span>{{ kindLabel(it.target_kind) }}</span>
            <span class="text-right tabular-nums">{{ it.quantity }}</span>
            <span class="text-right tabular-nums">
              <template v-if="it.duration_value">{{ it.duration_value }}{{ durUnit(it.duration_unit) }}</template>
              <span v-else class="text-muted-foreground">—</span>
            </span>
            <span class="text-right tabular-nums">¥{{ fmtCents(it.unit_price_cents) }}</span>
            <span class="text-right tabular-nums">
              <span v-if="it.qty_discount_bps < 10000 || it.duration_discount_bps < 10000" class="text-red-500 mr-1 text-xs">
                {{ fmtDiscountBps(it.qty_discount_bps) }}<span v-if="it.qty_discount_bps < 10000 && it.duration_discount_bps < 10000">×</span>{{ it.duration_discount_bps < 10000 ? fmtDiscountBps(it.duration_discount_bps) : '' }}
              </span>
              ¥{{ fmtCents(it.amount_cents) }}
            </span>
          </div>
        </div>
        <!-- 手续费 / 实付小结：仅有手续费时展示（历史/旧订单为 0 时隐藏）。 -->
        <div v-if="row.fee_cents > 0" class="mt-2 flex flex-col items-end gap-0.5 text-sm">
          <div class="flex w-full max-w-xs items-baseline justify-between">
            <span class="text-muted-foreground">{{ t('billing.purchase2.sumTotal') }}</span>
            <span class="tabular-nums">¥{{ fmtCents(row.total_cents) }}</span>
          </div>
          <div class="flex w-full max-w-xs items-baseline justify-between">
            <span class="text-muted-foreground">{{ t('billing.purchase2.sumFee') }}</span>
            <span class="tabular-nums">¥{{ fmtCents(row.fee_cents) }}</span>
          </div>
          <div class="flex w-full max-w-xs items-baseline justify-between">
            <span class="font-medium">{{ t('billing.purchase2.sumPayActual') }}</span>
            <span class="font-semibold tabular-nums text-red-600">¥{{ fmtCents(row.total_cents + row.fee_cents) }}</span>
          </div>
        </div>
        <div v-if="row.gift_runtime_minutes > 0" class="text-emerald-600 dark:text-emerald-400 mt-2 text-xs">
          {{ t('billing.purchase2.giftLine', { min: row.gift_runtime_minutes }) }}
        </div>
      </div>
    </template>
  </DataTable>
</template>
