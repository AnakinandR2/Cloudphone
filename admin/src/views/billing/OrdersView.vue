<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { Order } from '@/types/billing'
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

const data = ref<Order[]>([])
const total = ref(0)
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)

const filters = reactive({
  userId: '' as string,
  status: 'all',
})

const columns = computed<ColumnDef<Order>[]>(() => [
  {
    accessorKey: 'order_no',
    id: 'order_no',
    header: t('billing.colOrderNo'),
    meta: { label: 'billing.colOrderNo' },
  },
  {
    accessorKey: 'user_id',
    id: 'user_id',
    header: t('billing.colUserId'),
    meta: { label: 'billing.colUserId' },
  },
  {
    accessorKey: 'pay_method',
    id: 'pay_method',
    header: t('billing.colPay'),
    meta: { label: 'billing.colPay' },
  },
  {
    accessorKey: 'total_cents',
    id: 'total_cents',
    header: t('billing.colAmount'),
    meta: { label: 'billing.colAmount' },
  },
  {
    accessorKey: 'status',
    id: 'status',
    header: t('billing.colStatus'),
    meta: { label: 'billing.colStatus' },
  },
  {
    accessorKey: 'created_at',
    id: 'created_at',
    header: t('billing.colCreatedAt'),
    meta: { label: 'billing.colCreatedAt' },
  },
  {
    id: 'actions',
    header: '',
    enableHiding: false,
    meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' },
  },
])

async function load() {
  loading.value = true
  try {
    const params: { page: number, size: number, userId?: number, status?: string } = {
      page: page.value,
      size: pageSize.value,
    }
    if (filters.userId.trim()) {
      params.userId = Number(filters.userId.trim())
    }
    if (filters.status !== 'all') {
      params.status = filters.status
    }
    const res = await billingApi.orders(params)
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

async function markPaid(row: Order) {
  try {
    await billingApi.markPaid(row.id)
    toast.success(t('billing.markPaidOk'))
    load()
  }
  catch {
    toast.error(t('billing.markPaidFail'))
  }
}

function statusVariant(s: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  if (s === 'paid') return 'default'
  if (s === 'pending') return 'secondary'
  if (s === 'cancelled') return 'destructive'
  return 'outline'
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
              <SelectItem value="pending">{{ t('billing.status_pending') }}</SelectItem>
              <SelectItem value="paid">{{ t('billing.status_paid') }}</SelectItem>
              <SelectItem value="cancelled">{{ t('billing.status_cancelled') }}</SelectItem>
            </SelectContent>
          </Select>
          <Button variant="outline" size="sm" @click="onFilter">
            {{ t('common.search') }}
          </Button>
        </template>

        <template #cell-order_no="{ row }">
          <span class="font-mono text-xs">{{ row.order_no }}</span>
        </template>
        <template #cell-user_id="{ row }">
          <span class="tabular-nums text-muted-foreground">{{ row.user_id }}</span>
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
            v-if="row.status === 'pending'"
            :title="t('billing.markPaidConfirm')"
            @confirm="markPaid(row)"
          >
            <Button v-auth="'billing:manage'" variant="outline" size="sm">
              {{ t('billing.markPaid') }}
            </Button>
          </Popconfirm>
        </template>
      </DataTable>
    </CardContent>
  </Card>
</template>
