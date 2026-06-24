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
import { fmtCents } from '@/utils/money'

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

// 订单项明细：按需懒加载（展开行时拉 biz-orders/:id）。
const itemsCache = reactive<Record<number, BillingOrderItem[]>>({})
const itemsLoading = reactive<Record<number, boolean>>({})

async function ensureDetail(id: number) {
  if (itemsCache[id] || itemsLoading[id]) return
  itemsLoading[id] = true
  try {
    const res = await billingApi.billingOrderDetail(id)
    itemsCache[id] = res.data.items ?? []
  }
  catch {
    toast.error(t('billing.loadFail'))
  }
  finally {
    itemsLoading[id] = false
  }
}

function fmtBps(bps: number): string {
  // 10000=无折扣；展示为「X 折」（8500→8.5折）。无折扣显示 -。
  if (bps >= 10000) return '-'
  return `${(bps / 1000).toFixed(1).replace(/\.0$/, '')}${t('billing.discountUnit')}`
}

const columns = computed<ColumnDef<BillingOrder>[]>(() => [
  { id: 'expander', header: '', enableHiding: false, meta: { label: '' } },
  { accessorKey: 'id', id: 'id', header: t('billing.colOrderNo'), meta: { label: 'billing.colOrderNo' } },
  { accessorKey: 'user_id', id: 'user_id', header: t('billing.colUserId'), meta: { label: 'billing.colUserId' } },
  { accessorKey: 'biz_type', id: 'biz_type', header: t('billing.colBizType'), meta: { label: 'billing.colBizType' } },
  { accessorKey: 'pay_method', id: 'pay_method', header: t('billing.colPay'), meta: { label: 'billing.colPay' } },
  { accessorKey: 'total_cents', id: 'total_cents', header: t('billing.colAmount'), meta: { label: 'billing.colAmount' } },
  { accessorKey: 'fee_cents', id: 'fee_cents', header: t('billing.colFee'), meta: { label: 'billing.colFee' } },
  { id: 'pay_actual', header: t('billing.colPayActual'), meta: { label: 'billing.colPayActual' } },
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
    await billingApi.billingMarkPaid(row.id)
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

// 手续费构成提示，如「2% + ¥1」「2%」「¥1」（仅展示已配置的部分）。
function feeHint(row: BillingOrder): string {
  const parts: string[] = []
  if (row.fee_percent_bps > 0) parts.push(`${row.fee_percent_bps / 100}%`)
  if (row.fee_fixed_cents > 0) parts.push(`¥${fmtCents(row.fee_fixed_cents)}`)
  return parts.join(' + ')
}

function onFilter() {
  page.value = 1
  load()
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
              <SelectItem value="all">
                {{ t('billing.allStatus') }}
              </SelectItem>
              <SelectItem value="unpaid">
                {{ t('billing.status_unpaid') }}
              </SelectItem>
              <SelectItem value="paid">
                {{ t('billing.status_paid') }}
              </SelectItem>
              <SelectItem value="expired">
                {{ t('billing.status_expired') }}
              </SelectItem>
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
          <Badge variant="outline" class="text-xs">
            {{ bizLabel(row.biz_type) }}
          </Badge>
        </template>
        <template #cell-pay_method="{ row }">
          <span class="text-muted-foreground">{{ t(`billing.pay_${row.pay_method}`) }}</span>
        </template>
        <template #cell-total_cents="{ row }">
          <span class="tabular-nums">¥{{ fmtCents(row.total_cents) }}</span>
        </template>
        <template #cell-fee_cents="{ row }">
          <span v-if="row.fee_cents > 0" class="tabular-nums text-muted-foreground">¥{{ fmtCents(row.fee_cents) }}</span>
          <span v-else class="text-muted-foreground">-</span>
        </template>
        <template #cell-pay_actual="{ row }">
          <span class="tabular-nums font-medium">¥{{ fmtCents(row.total_cents + row.fee_cents) }}</span>
        </template>
        <template #cell-status="{ row }">
          <Badge :variant="statusVariant(row.status)">
            {{ t(`billing.status_${row.status}`) }}
          </Badge>
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

        <!-- 展开行：订单项明细（懒加载 biz-orders/:id） -->
        <template #expanded="{ row }">
          <div class="p-4">
            <!-- 费用小结：商品/充值金额、手续费、实付（实付 = total + fee） -->
            <div class="mb-3 flex flex-wrap gap-x-8 gap-y-1 text-sm">
              <span class="text-muted-foreground">
                {{ t('billing.colAmount') }}：<span class="tabular-nums text-foreground">¥{{ fmtCents(row.total_cents) }}</span>
              </span>
              <span class="text-muted-foreground">
                {{ t('billing.colFee') }}：<span class="tabular-nums text-foreground">¥{{ fmtCents(row.fee_cents) }}</span>
                <span v-if="row.fee_percent_bps || row.fee_fixed_cents" class="ml-1 text-xs text-muted-foreground">({{ feeHint(row) }})</span>
              </span>
              <span class="text-muted-foreground">
                {{ t('billing.colPayActual') }}：<span class="tabular-nums font-medium text-foreground">¥{{ fmtCents(row.total_cents + row.fee_cents) }}</span>
              </span>
            </div>
            <p class="mb-2 text-sm font-medium">
              {{ t('billing.orderItemsTitle') }}
            </p>
            <!-- 触发懒加载（首帧调用，已加载则 no-op） -->
            {{ (ensureDetail(row.id), '') }}
            <p v-if="itemsLoading[row.id]" class="text-muted-foreground text-xs">
              {{ t('common.loading') }}
            </p>
            <p v-else-if="!itemsCache[row.id]?.length" class="text-muted-foreground text-xs">
              {{ t('billing.orderItemsEmpty') }}
            </p>
            <div v-else class="overflow-auto rounded-md border">
              <table class="w-full text-xs">
                <thead>
                  <tr class="bg-muted/40 border-b text-muted-foreground">
                    <th class="px-3 py-2 text-left font-medium">
                      {{ t('billing.colTargetKind') }}
                    </th>
                    <th class="px-3 py-2 text-right font-medium">
                      {{ t('billing.colQty') }}
                    </th>
                    <th class="px-3 py-2 text-right font-medium">
                      {{ t('billing.colDuration') }}
                    </th>
                    <th class="px-3 py-2 text-right font-medium">
                      {{ t('billing.colUnitPrice') }}
                    </th>
                    <th class="px-3 py-2 text-right font-medium">
                      {{ t('billing.colQtyDiscount') }}
                    </th>
                    <th class="px-3 py-2 text-right font-medium">
                      {{ t('billing.colDurDiscount') }}
                    </th>
                    <th class="px-3 py-2 text-right font-medium">
                      {{ t('billing.colAmount') }}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(it, i) in itemsCache[row.id]" :key="i" class="border-b last:border-0">
                    <td class="px-3 py-2">
                      <Badge variant="outline" class="text-[10px]">
                        {{ t(`billing.kind_${it.target_kind}`, it.target_kind) }}
                      </Badge>
                    </td>
                    <td class="px-3 py-2 text-right tabular-nums">
                      {{ it.quantity }}
                    </td>
                    <td class="px-3 py-2 text-right tabular-nums">
                      <template v-if="it.duration_value">
                        {{ it.duration_value }}{{ it.duration_unit === 'day' ? t('billing.durDay') : t('billing.durMonth') }}
                      </template>
                      <span v-else class="text-muted-foreground">-</span>
                    </td>
                    <td class="px-3 py-2 text-right tabular-nums">
                      ¥{{ fmtCents(it.unit_price_cents) }}
                    </td>
                    <td class="px-3 py-2 text-right tabular-nums">
                      {{ fmtBps(it.qty_discount_bps) }}
                    </td>
                    <td class="px-3 py-2 text-right tabular-nums">
                      {{ fmtBps(it.duration_discount_bps) }}
                    </td>
                    <td class="px-3 py-2 text-right font-medium tabular-nums">
                      ¥{{ fmtCents(it.amount_cents) }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </template>
      </DataTable>
    </CardContent>
  </Card>
</template>
