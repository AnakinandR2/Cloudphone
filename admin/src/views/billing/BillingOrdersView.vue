<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { BillingOrder, BillingOrderItem } from '@/types/billing'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import billingApi from '@/api/modules/billing'
import DataTable from '@/components/DataTable.vue'
import Popconfirm from '@/components/Popconfirm.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
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
import { formatDateTime } from '@/utils/date'
import { fmtCents, fmtDiscountBps } from '@/utils/money'

const { t } = useI18n()

const data = ref<BillingOrder[]>([])
const total = ref(0)
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)

const filters = reactive({
  userId: '' as string,
  status: 'all',
})

// 明细缓存（按订单 id）。展开行时按需拉 mark-paid 不合适——明细随订单返回；
// 这里改为从 mark-paid 详情或单独缓存。订单列表不带 items，展开时显示已知字段。
const itemsCache = reactive<Record<number, BillingOrderItem[]>>({})

const columns = computed<ColumnDef<BillingOrder>[]>(() => [
  { id: 'expander', header: '', enableHiding: false, meta: { label: '' } },
  { accessorKey: 'id', id: 'id', header: t('billing.colOrderNo'), meta: { label: 'billing.colOrderNo' } },
  { accessorKey: 'user_id', id: 'user_id', header: t('billing.colUserId'), meta: { label: 'billing.colUserId' } },
  { accessorKey: 'biz_type', id: 'biz_type', header: t('billing.colBizType'), meta: { label: 'billing.colBizType' } },
  { accessorKey: 'pay_method', id: 'pay_method', header: t('billing.colPay'), meta: { label: 'billing.colPay' } },
  { accessorKey: 'total_cents', id: 'total_cents', header: t('billing.colAmount'), meta: { label: 'billing.colAmount' } },
  { accessorKey: 'status', id: 'status', header: t('billing.colStatus'), meta: { label: 'billing.colStatus' } },
  { accessorKey: 'created_at', id: 'created_at', header: t('billing.colCreatedAt'), meta: { label: 'billing.colCreatedAt' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
])

async function load() {
  loading.value = true
  try {
    const params: { page: number, size: number, userId?: number, status?: string } = {
      page: page.value,
      size: pageSize.value,
    }
    if (filters.userId.trim()) params.userId = Number(filters.userId.trim())
    if (filters.status !== 'all') params.status = filters.status
    const res = await billingApi.billingOrders(params)
    data.value = res.data.list
    total.value = res.data.total
  }
  catch {
    toast.error(t('billing.loadFail'))
  }
  finally {
    loading.value = false
  }
}

async function markPaid(row: BillingOrder) {
  try {
    const res = await billingApi.billingMarkPaid(row.id)
    itemsCache[row.id] = res.data.items
    toast.success(t('billing.markPaidOk'))
    load()
  }
  catch {
    toast.error(t('billing.markPaidFail'))
  }
}

function statusVariant(s: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  if (s === 'paid') return 'default'
  if (s === 'unpaid') return 'secondary'
  if (s === 'expired') return 'destructive'
  return 'outline'
}

function bizLabel(b: string): string {
  return t(`billing.biz_${b}`)
}

function onFilter() {
  page.value = 1
  load()
}

function kindLabel(k: string): string {
  if (k === 'seat') return t('billing.kind_seat')
  if (k === 'boot_slot') return t('billing.kind_boot_slot')
  if (k === 'runtime_pack') return t('billing.kind_runtime_pack')
  return k
}

onMounted(load)
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ t('billing.ordersTitle') }}</CardTitle>
      <CardDescription>{{ t('billing.ordersDesc') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <DataTable
        :columns="columns"
        :data="data"
        :loading="loading"
        expandable
      >
        <template #filters>
          <Input
            v-model="filters.userId"
            type="number"
            class="h-9 w-32"
            :placeholder="t('billing.filterUserId')"
            @change="onFilter"
          />
          <Select v-model="filters.status" @update:model-value="onFilter">
            <SelectTrigger class="h-9 w-36">
              <SelectValue :placeholder="t('billing.allStatus')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{{ t('billing.allStatus') }}</SelectItem>
              <SelectItem value="unpaid">{{ t('billing.status_unpaid') }}</SelectItem>
              <SelectItem value="paid">{{ t('billing.status_paid') }}</SelectItem>
              <SelectItem value="expired">{{ t('billing.status_expired') }}</SelectItem>
            </SelectContent>
          </Select>
          <Button variant="outline" size="sm" @click="onFilter">
            {{ t('common.search') }}
          </Button>
        </template>

        <template #cell-id="{ row }">
          <span class="font-mono text-xs">#{{ row.id }}</span>
        </template>
        <template #cell-user_id="{ row }">
          <span class="tabular-nums text-muted-foreground">{{ row.user_id }}</span>
        </template>
        <template #cell-biz_type="{ row }">
          <Badge variant="outline" class="text-xs">{{ bizLabel(row.biz_type) }}</Badge>
        </template>
        <template #cell-pay_method="{ row }">
          <span class="text-muted-foreground">{{ t(`billing.pay_${row.pay_method}`) }}</span>
        </template>
        <template #cell-total_cents="{ row }">
          <span class="tabular-nums">¥{{ fmtCents(row.total_cents) }}</span>
        </template>
        <template #cell-status="{ row }">
          <Badge :variant="statusVariant(row.status)">{{ t(`billing.status_${row.status}`) }}</Badge>
        </template>
        <template #cell-created_at="{ row }">
          <span class="tabular-nums text-muted-foreground">{{ formatDateTime(row.created_at) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <Popconfirm
            v-if="row.status === 'unpaid'"
            :title="t('billing.markPaidConfirm')"
            @confirm="markPaid(row)"
          >
            <Button v-auth="'billing:manage'" variant="outline" size="sm">
              {{ t('billing.markPaid') }}
            </Button>
          </Popconfirm>
        </template>

        <template #expanded="{ row }">
          <div class="space-y-2 p-4">
            <p class="text-sm font-medium">{{ t('billing.orderItemsTitle') }}</p>
            <template v-if="itemsCache[row.id]?.length">
              <div class="overflow-auto rounded-md border">
                <table class="w-full text-xs">
                  <thead>
                    <tr class="bg-muted/40 border-b">
                      <th class="px-3 py-2 text-left font-medium">{{ t('billing.colTargetKind') }}</th>
                      <th class="px-3 py-2 text-left font-medium">{{ t('billing.colQty') }}</th>
                      <th class="px-3 py-2 text-left font-medium">{{ t('billing.colDuration') }}</th>
                      <th class="px-3 py-2 text-left font-medium">{{ t('billing.colUnitPrice') }}</th>
                      <th class="px-3 py-2 text-left font-medium">{{ t('billing.colDiscount') }}</th>
                      <th class="px-3 py-2 text-left font-medium">{{ t('billing.colAmount') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="it in itemsCache[row.id]" :key="it.id" class="border-b last:border-0">
                      <td class="px-3 py-1.5">{{ kindLabel(it.target_kind) }}</td>
                      <td class="px-3 py-1.5 tabular-nums">{{ it.quantity }}</td>
                      <td class="px-3 py-1.5 tabular-nums">{{ it.duration_value || '—' }} {{ it.duration_unit }}</td>
                      <td class="px-3 py-1.5 tabular-nums">¥{{ fmtCents(it.unit_price_cents) }}</td>
                      <td class="text-muted-foreground px-3 py-1.5">
                        {{ fmtDiscountBps(it.qty_discount_bps) || t('billing.fullPrice') }}
                        × {{ fmtDiscountBps(it.duration_discount_bps) || t('billing.fullPrice') }}
                      </td>
                      <td class="px-3 py-1.5 tabular-nums">¥{{ fmtCents(it.amount_cents) }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </template>
            <p v-else class="text-muted-foreground text-xs">{{ t('billing.orderItemsHint') }}</p>
          </div>
        </template>
      </DataTable>
    </CardContent>
  </Card>
</template>
