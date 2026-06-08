<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { Order } from '@/types/billing'
import { CreditCard } from 'lucide-vue-next'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import billingApi from '@/api/modules/billing'
import DataTable from '@/components/DataTable.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useQuerySync } from '@/composables/useQuerySync'
import { formatDateTime } from '@/utils/date'
import { fmtCents } from '@/utils/money'

const { t } = useI18n()

// ——— 数据 ———
const data = ref<Order[]>([])
const loading = ref(false)

// ——— 筛选 ———
const filters = reactive({ status: 'all', q: '' })
useQuerySync(filters, { status: 'all', q: '' })

// ——— 加载订单 ———
async function load() {
  loading.value = true
  try {
    const params: { page: number; size: number; status?: string } = { page: 1, size: 200 }
    if (filters.status !== 'all') params.status = filters.status
    const res = await billingApi.orders(params)
    data.value = res.data.list
  }
  catch {
    toast.error(t('billing.loadFailed'))
  }
  finally {
    loading.value = false
  }
}

onMounted(load)

// ——— 状态变化时重新加载 ———
watch(() => filters.status, load)

// ——— 列定义 ———
const columns = computed<ColumnDef<Order>[]>(() => [
  {
    accessorKey: 'order_no',
    id: 'order_no',
    header: t('billing.colOrderNo'),
    meta: { label: 'billing.colOrderNo' },
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
])

// ——— 状态徽章变体 ———
function statusVariant(s: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  if (s === 'paid') return 'default'
  if (s === 'pending') return 'secondary'
  return 'destructive'
}

// ——— 充值对话框 ———
const topupOpen = ref(false)
const topupYuan = ref<number | undefined>(undefined)
const topupLoading = ref(false)

async function submitTopup() {
  const yuan = topupYuan.value
  if (!yuan || yuan <= 0) {
    toast.error(t('billing.topupAmountRequired'))
    return
  }
  const cents = Math.round(yuan * 100)
  topupLoading.value = true
  try {
    await billingApi.topup(cents)
    toast.success(t('billing.topupOk'))
    topupOpen.value = false
    topupYuan.value = undefined
    load()
  }
  catch {
    toast.error(t('billing.topupFailed'))
  }
  finally {
    topupLoading.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <Card>
      <CardHeader class="flex flex-row items-start justify-between gap-4">
        <div>
          <CardTitle>{{ t('billing.ordersTitle') }}</CardTitle>
          <CardDescription>{{ t('billing.ordersDesc') }}</CardDescription>
        </div>
        <Button variant="outline" size="sm" @click="topupOpen = true">
          <CreditCard class="mr-1.5 h-4 w-4" />
          {{ t('billing.topup') }}
        </Button>
      </CardHeader>
      <CardContent>
        <DataTable
          v-model:search-value="filters.q"
          :columns="columns"
          :data="data"
          :loading="loading"
          :search-placeholder="t('billing.searchOrder')"
        >
          <template #filters>
            <Select v-model="filters.status">
              <SelectTrigger class="h-9 w-36">
                <SelectValue :placeholder="t('billing.allStatus')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">
                  {{ t('billing.allStatus') }}
                </SelectItem>
                <SelectItem v-for="s in ['pending', 'paid', 'cancelled']" :key="s" :value="s">
                  {{ t(`billing.status_${s}`) }}
                </SelectItem>
              </SelectContent>
            </Select>
          </template>

          <template #cell-order_no="{ row }">
            <span class="font-mono text-xs">{{ row.order_no }}</span>
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
            <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.created_at) }}</span>
          </template>
        </DataTable>
      </CardContent>
    </Card>

    <!-- 充值对话框 -->
    <Dialog v-model:open="topupOpen">
      <DialogContent class="sm:max-w-[400px]">
        <DialogHeader>
          <DialogTitle>{{ t('billing.topupTitle') }}</DialogTitle>
          <DialogDescription>{{ t('billing.topupDesc') }}</DialogDescription>
        </DialogHeader>
        <div class="grid gap-4 py-4">
          <div class="grid grid-cols-4 items-center gap-4">
            <Label for="topup-amount" class="text-right">
              {{ t('billing.topupAmount') }}
            </Label>
            <div class="col-span-3 flex items-center gap-2">
              <span class="text-muted-foreground text-sm">¥</span>
              <Input
                id="topup-amount"
                v-model.number="topupYuan"
                type="number"
                min="0.01"
                step="0.01"
                :placeholder="t('billing.topupAmountPlaceholder')"
                class="flex-1"
                @keyup.enter="submitTopup"
              />
              <span class="text-muted-foreground text-sm">{{ t('billing.topupYuan') }}</span>
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="topupOpen = false">
            {{ t('crud.cancel') }}
          </Button>
          <Button :disabled="topupLoading" @click="submitTopup">
            {{ topupLoading ? t('billing.submitting') : t('billing.topupConfirm') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
