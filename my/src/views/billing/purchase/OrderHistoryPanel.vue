<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { Order2 } from '@/types/billing'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import billingApi from '@/api/modules/billing'
import DataTable from '@/components/DataTable.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { formatDateTime } from '@/utils/date'
import { fmtCents } from '@/utils/money'

// 默认 tab：全部订单 — 订单 ID / 时间 / 金额 / 状态；未支付可继续支付。
const emit = defineEmits<{ paid: [] }>()
const { t } = useI18n()

const data = ref<Order2[]>([])
const loading = ref(false)
const payingId = ref<number | null>(null)
const filters = reactive({ status: 'all', q: '' })

async function load() {
  loading.value = true
  try {
    const params: { page: number, size: number, status?: string } = { page: 1, size: 200 }
    if (filters.status !== 'all') params.status = filters.status
    const res = await billingApi.orders2(params)
    data.value = res.data.list
  }
  finally {
    loading.value = false
  }
}

onMounted(load)
watch(() => filters.status, load)
defineExpose({ reload: load })

const columns = computed<ColumnDef<Order2>[]>(() => [
  { accessorKey: 'id', id: 'id', header: t('billing.purchase2.colOrderId'), meta: { label: 'billing.purchase2.colOrderId' } },
  { accessorKey: 'biz_type', id: 'biz_type', header: t('billing.purchase2.colBizType'), meta: { label: 'billing.purchase2.colBizType' } },
  { accessorKey: 'total_cents', id: 'total_cents', header: t('billing.purchase2.colAmount'), meta: { label: 'billing.purchase2.colAmount' } },
  { accessorKey: 'pay_method', id: 'pay_method', header: t('billing.purchase2.colPay'), meta: { label: 'billing.purchase2.colPay' } },
  { accessorKey: 'status', id: 'status', header: t('billing.purchase2.colStatus'), meta: { label: 'billing.purchase2.colStatus' } },
  { accessorKey: 'created_at', id: 'created_at', header: t('billing.purchase2.colCreatedAt'), meta: { label: 'billing.purchase2.colCreatedAt' } },
  { id: 'actions', header: '', meta: { label: 'billing.purchase2.colActions' } },
])

function statusVariant(s: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  if (s === 'paid') return 'default'
  if (s === 'unpaid') return 'secondary'
  return 'outline'
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
    :search-placeholder="t('billing.purchase2.searchOrder')"
  >
    <template #cell-id="{ row }">
      <span class="font-mono text-xs">#{{ row.id }}</span>
    </template>
    <template #cell-biz_type="{ row }">
      <span>{{ t(`billing.purchase2.biz_${row.biz_type}`) }}</span>
    </template>
    <template #cell-total_cents="{ row }">
      <span class="tabular-nums">¥{{ fmtCents(row.total_cents) }}</span>
    </template>
    <template #cell-pay_method="{ row }">
      <span class="text-muted-foreground">{{ t(`billing.purchase2.pay_${row.pay_method}`) }}</span>
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
  </DataTable>
</template>
