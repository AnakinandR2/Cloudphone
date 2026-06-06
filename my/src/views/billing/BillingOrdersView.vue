<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import DataTable from '@/components/DataTable.vue'
import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useQuerySync } from '@/composables/useQuerySync'
import UsageLineChart from './UsageLineChart.vue'

const { t } = useI18n()

function money(n: number) {
  return n.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}
// 确定性伪随机，保证趋势图重渲染稳定。
function seeded(i: number, salt: number) {
  const x = Math.sin((i + 1) * 12.9898 + salt * 78.233) * 43758.5453
  return x - Math.floor(x)
}
// 近 1 年消费趋势，可按月 / 按天切换，默认按月。
const trendGran = ref<'month' | 'day'>('month')
// 原型确定性日数据（部分天为 0 表示当天无购买）。
const dailyRaw = computed(() => {
  const n = 365
  const today = new Date()
  const out: { d: Date, v: number }[] = []
  for (let k = n - 1; k >= 0; k--) {
    const d = new Date(today)
    d.setDate(today.getDate() - k)
    const idx = n - 1 - k
    out.push({ d, v: seeded(idx, 9) < 0.45 ? 0 : Math.round(50 + seeded(idx, 10) * 760) })
  }
  return out
})
const spendSeries = computed(() => {
  if (trendGran.value === 'day') {
    return dailyRaw.value.map(({ d, v }) => ({
      label: `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`,
      value: v,
    }))
  }
  // 按月聚合（近 12 个月）
  const map = new Map<string, number>()
  for (const { d, v } of dailyRaw.value) {
    const key = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
    map.set(key, (map.get(key) ?? 0) + v)
  }
  return [...map.entries()].map(([key, value]) => ({ label: key.slice(5), value }))
})

const filters = reactive({ type: 'all', status: 'all', from: '', to: '', q: '' })
useQuerySync(filters, { type: 'all', status: 'all', from: '', to: '', q: '' })

type OrderType = 'instance' | 'hours'
type OrderStatus = 'paid' | 'pending' | 'refunded' | 'cancelled'
interface Order {
  id: number
  orderNo: string
  type: OrderType
  detail: string
  amount: number
  status: OrderStatus
  payMethod: string
  createdAt: string
  ts: number
}

// 原型：确定性静态订单（不依赖随机数，重渲染稳定）。
const STATUSES: OrderStatus[] = ['paid', 'paid', 'pending', 'paid', 'refunded', 'paid', 'cancelled', 'paid']
const PAY = ['alipay', 'wechat', 'balance']
const orders = computed<Order[]>(() => {
  const out: Order[] = []
  for (let i = 0; i < 24; i++) {
    const isInstance = i % 3 !== 0
    const type: OrderType = isInstance ? 'instance' : 'hours'
    const qtyOrHours = isInstance ? (i % 5) + 1 : ((i % 4) + 1) * 250
    const unitCycle = ['cycle_month', 'cycle_quarter', 'cycle_year'][i % 3]
    const detail = isInstance
      ? `${qtyOrHours} ${t('billing.unit')} · ${t(`billing.${unitCycle}`)}`
      : `${qtyOrHours} ${t('billing.hoursUnit')}`
    const amount = isInstance ? qtyOrHours * 30 * [1, 2.55, 8.4][i % 3] : qtyOrHours * 0.2 * 0.9
    const day = 28 - i
    const createdAt = `2026-05-${String(day).padStart(2, '0')} ${String(8 + (i % 12)).padStart(2, '0')}:${String((i * 7) % 60).padStart(2, '0')}`
    out.push({
      id: i + 1,
      orderNo: `BIL2026${String(60000 - i * 7).padStart(6, '0')}`,
      type,
      detail,
      amount: +amount.toFixed(2),
      status: STATUSES[i % STATUSES.length],
      payMethod: PAY[i % PAY.length],
      createdAt,
      ts: new Date(createdAt.replace(' ', 'T')).getTime(),
    })
  }
  return out
})

const fromTs = computed(() => (filters.from ? new Date(filters.from).getTime() : null))
const toTs = computed(() => (filters.to ? new Date(filters.to).getTime() : null))
const filtered = computed(() => orders.value.filter(o =>
  (filters.type === 'all' || o.type === filters.type)
  && (filters.status === 'all' || o.status === filters.status)
  && (fromTs.value === null || o.ts >= fromTs.value)
  && (toTs.value === null || o.ts <= toTs.value),
))

const columns = computed<ColumnDef<Order>[]>(() => [
  { accessorKey: 'orderNo', id: 'orderNo', header: t('billing.colOrderNo'), meta: { label: 'billing.colOrderNo' } },
  { accessorKey: 'type', id: 'type', header: t('billing.colType'), meta: { label: 'billing.colType' } },
  { accessorKey: 'detail', id: 'detail', header: t('billing.colDetail'), meta: { label: 'billing.colDetail' } },
  { accessorKey: 'amount', id: 'amount', header: t('billing.colAmount'), meta: { label: 'billing.colAmount' } },
  { accessorKey: 'status', id: 'status', header: t('billing.colStatus'), meta: { label: 'billing.colStatus' } },
  { accessorKey: 'payMethod', id: 'payMethod', header: t('billing.colPay'), meta: { label: 'billing.colPay' } },
  { accessorKey: 'createdAt', id: 'createdAt', header: t('billing.colCreatedAt'), meta: { label: 'billing.colCreatedAt' } },
])

function statusVariant(s: OrderStatus): 'default' | 'secondary' | 'destructive' | 'outline' {
  if (s === 'paid') return 'default'
  if (s === 'pending') return 'secondary'
  if (s === 'refunded') return 'outline'
  return 'destructive'
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <Card>
      <CardHeader>
        <CardTitle>{{ t('billing.ordersTitle') }}</CardTitle>
        <CardDescription>{{ t('billing.ordersDesc') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <!-- 近 1 年消费趋势（可按月 / 按天，默认按月）-->
        <div class="mb-6 flex flex-col gap-2">
          <div class="flex items-center justify-between">
            <span class="text-muted-foreground text-sm">{{ t('billing.spendTrend') }}</span>
            <div class="bg-muted inline-flex rounded-md p-0.5 text-xs">
              <button
                type="button"
                class="cursor-pointer rounded px-2.5 py-1 font-medium transition-colors"
                :class="trendGran === 'month' ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground'"
                @click="trendGran = 'month'"
              >
                {{ t('billing.byMonth') }}
              </button>
              <button
                type="button"
                class="cursor-pointer rounded px-2.5 py-1 font-medium transition-colors"
                :class="trendGran === 'day' ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground'"
                @click="trendGran = 'day'"
              >
                {{ t('billing.byDay') }}
              </button>
            </div>
          </div>
          <UsageLineChart :data="spendSeries" :height="180" :unit="t('billing.yuan')" />
        </div>

        <DataTable
          v-model:search-value="filters.q"
          :columns="columns"
          :data="filtered"
          :search-placeholder="t('billing.searchOrder')"
        >
          <template #filters>
            <Select v-model="filters.type">
              <SelectTrigger class="h-9 w-32">
                <SelectValue :placeholder="t('billing.allTypes')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">
                  {{ t('billing.allTypes') }}
                </SelectItem>
                <SelectItem value="instance">
                  {{ t('billing.typeInstance') }}
                </SelectItem>
                <SelectItem value="hours">
                  {{ t('billing.typeHours') }}
                </SelectItem>
              </SelectContent>
            </Select>
            <Select v-model="filters.status">
              <SelectTrigger class="h-9 w-32">
                <SelectValue :placeholder="t('billing.allStatus')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">
                  {{ t('billing.allStatus') }}
                </SelectItem>
                <SelectItem v-for="s in ['paid', 'pending', 'refunded', 'cancelled']" :key="s" :value="s">
                  {{ t(`billing.status_${s}`) }}
                </SelectItem>
              </SelectContent>
            </Select>
            <!-- 下单时间：日期时间范围过滤 -->
            <div class="flex items-center gap-1.5">
              <Input
                v-model="filters.from"
                type="datetime-local"
                class="h-9 w-[200px]"
                :aria-label="t('billing.fromTime')"
              />
              <span class="text-muted-foreground text-sm">~</span>
              <Input
                v-model="filters.to"
                type="datetime-local"
                class="h-9 w-[200px]"
                :aria-label="t('billing.toTime')"
              />
            </div>
          </template>

          <template #cell-orderNo="{ row }">
            <span class="font-mono text-xs">{{ row.orderNo }}</span>
          </template>
          <template #cell-type="{ row }">
            <Badge variant="outline">{{ t(`billing.type${row.type === 'instance' ? 'Instance' : 'Hours'}`) }}</Badge>
          </template>
          <template #cell-amount="{ row }">
            <span class="tabular-nums">¥{{ money(row.amount) }}</span>
          </template>
          <template #cell-status="{ row }">
            <Badge :variant="statusVariant(row.status)">{{ t(`billing.status_${row.status}`) }}</Badge>
          </template>
          <template #cell-payMethod="{ row }">
            <span class="text-muted-foreground">{{ t(`billing.pay_${row.payMethod}`) }}</span>
          </template>
          <template #cell-createdAt="{ row }">
            <span class="text-muted-foreground tabular-nums">{{ row.createdAt }}</span>
          </template>
        </DataTable>
      </CardContent>
    </Card>
  </div>
</template>
