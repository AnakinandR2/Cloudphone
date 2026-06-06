<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import { computed, reactive } from 'vue'
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

const { t } = useI18n()

function money(n: number) {
  return n.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

const filters = reactive({ type: 'all', status: 'all', from: '', to: '', q: '' })

type OrderType = 'instance' | 'hours'
type OrderStatus = 'paid' | 'pending' | 'refunded' | 'cancelled'
interface Order {
  id: number
  orderNo: string
  user: string
  type: OrderType
  detail: string
  amount: number
  status: OrderStatus
  payMethod: string
  createdAt: string
  ts: number
}

const STATUSES: OrderStatus[] = ['paid', 'paid', 'pending', 'paid', 'refunded', 'paid', 'cancelled', 'paid']
const PAY = ['alipay', 'wechat', 'balance']
const USERS = ['138****8000', '139****2341', '136****7788', '137****5566', '135****9012']
const orders = computed<Order[]>(() => {
  const out: Order[] = []
  for (let i = 0; i < 28; i++) {
    const isInstance = i % 3 !== 0
    const type: OrderType = isInstance ? 'instance' : 'hours'
    const qtyOrHours = isInstance ? (i % 5) + 1 : ((i % 4) + 1) * 250
    const unitCycle = ['cycle_month', 'cycle_quarter', 'cycle_year'][i % 3]
    const detail = isInstance
      ? `${qtyOrHours} ${t('billing.unit')} · ${t(`billing.${unitCycle}`)}`
      : `${qtyOrHours} ${t('billing.hoursUnit')}`
    const amount = isInstance ? qtyOrHours * 30 * [1, 2.55, 8.4][i % 3] : qtyOrHours * 0.2 * 0.9
    const day = 28 - (i % 28)
    const createdAt = `2026-05-${String(day).padStart(2, '0')} ${String(8 + (i % 12)).padStart(2, '0')}:${String((i * 7) % 60).padStart(2, '0')}`
    out.push({
      id: i + 1,
      orderNo: `BIL2026${String(60000 - i * 7).padStart(6, '0')}`,
      user: USERS[i % USERS.length],
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
  { accessorKey: 'user', id: 'user', header: t('billing.colUser'), meta: { label: 'billing.colUser' } },
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
  <Card>
    <CardHeader>
      <CardTitle>{{ t('billing.ordersTitle') }}</CardTitle>
      <CardDescription>{{ t('billing.ordersDesc') }}</CardDescription>
    </CardHeader>
    <CardContent>
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
              <SelectItem value="all">{{ t('billing.allTypes') }}</SelectItem>
              <SelectItem value="instance">{{ t('billing.typeInstance') }}</SelectItem>
              <SelectItem value="hours">{{ t('billing.typeHours') }}</SelectItem>
            </SelectContent>
          </Select>
          <Select v-model="filters.status">
            <SelectTrigger class="h-9 w-32">
              <SelectValue :placeholder="t('billing.allStatus')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{{ t('billing.allStatus') }}</SelectItem>
              <SelectItem v-for="s in ['paid', 'pending', 'refunded', 'cancelled']" :key="s" :value="s">
                {{ t(`billing.status_${s}`) }}
              </SelectItem>
            </SelectContent>
          </Select>
          <div class="flex items-center gap-1.5">
            <Input v-model="filters.from" type="datetime-local" class="h-9 w-[200px]" :aria-label="t('billing.fromTime')" />
            <span class="text-muted-foreground text-sm">~</span>
            <Input v-model="filters.to" type="datetime-local" class="h-9 w-[200px]" :aria-label="t('billing.toTime')" />
          </div>
        </template>

        <template #cell-orderNo="{ row }">
          <span class="font-mono text-xs">{{ row.orderNo }}</span>
        </template>
        <template #cell-user="{ row }">
          <span class="tabular-nums">{{ row.user }}</span>
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
</template>
