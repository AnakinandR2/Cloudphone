<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { BillingAccount, EntitlementBatch, EntitlementsResult, RuntimeUsageSlice } from '@/types/billing'
import { ChevronLeft, ChevronRight, Clock, Cpu, Smartphone, Wallet } from 'lucide-vue-next'
import { computed, onMounted, ref } from 'vue'
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
import { Skeleton } from '@/components/ui/skeleton'
import { formatDate, formatDateTime } from '@/utils/date'
import { fmtCents } from '@/utils/money'

const { t } = useI18n()

// ——— 加载状态 ———
const loading = ref(false)
const accountData = ref<BillingAccount | null>(null)
const entitlementsData = ref<EntitlementsResult | null>(null)

// ——— 时长余额 min → h+min ———
function fmtMinutes(min: number): string {
  const h = Math.floor(min / 60)
  const m = min % 60
  if (h > 0 && m > 0) return `${h}h ${m}min`
  if (h > 0) return `${h}h`
  return `${m}min`
}

// ——— 科目 badge 配色 ———
function subjectVariant(subject: string): 'default' | 'secondary' | 'outline' | 'destructive' {
  if (subject === 'instance_seat') return 'default'
  if (subject === 'boot_seat') return 'secondary'
  if (subject === 'runtime_minute') return 'outline'
  return 'outline'
}

// ——— 科目单位 ———
function subjectUnit(subject: string): string {
  if (subject === 'instance_seat') return t('billing.unitTai')
  if (subject === 'boot_seat') return t('billing.unitTai')
  if (subject === 'runtime_minute') return t('billing.unitMin')
  return ''
}

// ——— 数据加载 ———
async function load() {
  loading.value = true
  try {
    const [accRes, entRes] = await Promise.all([
      billingApi.account(),
      billingApi.entitlements(),
    ])
    accountData.value = accRes.data
    entitlementsData.value = entRes.data
  }
  catch {
    toast.error(t('billing.loadFailed'))
  }
  finally {
    loading.value = false
  }
}

onMounted(load)

// ——— 运行用量（时长费切片，倒序分页）———
const RT_SIZE = 10
const runtimeSlices = ref<RuntimeUsageSlice[]>([])
const runtimeTotal = ref(0)
const runtimePage = ref(1)
const runtimeLoading = ref(false)
const runtimePages = computed(() => Math.max(1, Math.ceil(runtimeTotal.value / RT_SIZE)))
async function loadRuntime() {
  runtimeLoading.value = true
  try {
    const { data } = await billingApi.runtimeUsage({ page: runtimePage.value, size: RT_SIZE })
    runtimeSlices.value = data.list ?? []
    runtimeTotal.value = data.total ?? 0
  }
  catch { /* 静默：无数据不打扰 */ }
  finally { runtimeLoading.value = false }
}
function goRuntime(p: number) {
  if (p < 1 || p > runtimePages.value || runtimeLoading.value)
    return
  runtimePage.value = p
  loadRuntime()
}
onMounted(loadRuntime)

// ——— 汇总卡片 ———
const balance = computed(() => accountData.value ? fmtCents(accountData.value.balance_cents) : '-')
const instanceSeat = computed(() => entitlementsData.value?.capacities.instance_seat ?? '-')
const bootSeat = computed(() => entitlementsData.value?.capacities.boot_seat ?? '-')
const runtimeMinute = computed(() => {
  const m = entitlementsData.value?.capacities.runtime_minute
  if (m === undefined || m === null) return '-'
  return fmtMinutes(m)
})

// ——— 批次明细表列 ———
const batches = computed<EntitlementBatch[]>(() => entitlementsData.value?.batches ?? [])

const columns = computed<ColumnDef<EntitlementBatch>[]>(() => [
  {
    accessorKey: 'subject',
    id: 'subject',
    header: t('billing.colBatchSubject'),
    meta: { label: 'billing.colBatchSubject' },
  },
  {
    accessorKey: 'quantity',
    id: 'quantity',
    header: t('billing.colBatchQuantity'),
    meta: { label: 'billing.colBatchQuantity' },
  },
  {
    accessorKey: 'used',
    id: 'used',
    header: t('billing.colBatchUsed'),
    meta: { label: 'billing.colBatchUsed' },
  },
  {
    id: 'remaining',
    header: t('billing.colBatchRemaining'),
    meta: { label: 'billing.colBatchRemaining' },
    accessorFn: (row) => row.quantity - row.used,
  },
  {
    accessorKey: 'expire_at',
    id: 'expire_at',
    header: t('billing.colBatchExpire'),
    meta: { label: 'billing.colBatchExpire' },
  },
  {
    accessorKey: 'source',
    id: 'source',
    header: t('billing.colBatchSource'),
    meta: { label: 'billing.colBatchSource' },
  },
  {
    accessorKey: 'source_ref',
    id: 'source_ref',
    header: t('billing.colBatchSourceRef'),
    meta: { label: 'billing.colBatchSourceRef' },
  },
  {
    accessorKey: 'created_at',
    id: 'created_at',
    header: t('billing.colBatchCreatedAt'),
    meta: { label: 'billing.colBatchCreatedAt' },
  },
])
</script>

<template>
  <div class="flex flex-col gap-6">
    <!-- 标题 -->
    <div>
      <h1 class="text-xl font-semibold tracking-tight">{{ t('billing.accountResourceTitle') }}</h1>
      <p class="text-muted-foreground mt-1 text-sm">{{ t('billing.accountResourceDesc') }}</p>
    </div>

    <!-- 汇总卡片骨架屏 -->
    <div v-if="loading" class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <Card v-for="i in 4" :key="i">
        <CardContent class="flex items-center gap-3 py-4">
          <Skeleton class="size-9 rounded-lg" />
          <div class="flex flex-col gap-1.5">
            <Skeleton class="h-3 w-20" />
            <Skeleton class="h-5 w-28" />
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- 汇总卡片 -->
    <div v-else class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <!-- 余额 -->
      <Card>
        <CardContent class="flex items-center gap-3 py-4">
          <div class="flex size-9 items-center justify-center rounded-lg bg-amber-500/10 text-amber-600 dark:text-amber-400">
            <Wallet class="size-4" />
          </div>
          <div>
            <div class="text-muted-foreground text-xs">{{ t('billing.overviewBalance') }}</div>
            <div class="text-lg font-semibold tabular-nums">¥{{ balance }}</div>
          </div>
        </CardContent>
      </Card>

      <!-- 实例席位 -->
      <Card>
        <CardContent class="flex items-center gap-3 py-4">
          <div class="flex size-9 items-center justify-center rounded-lg bg-blue-500/10 text-blue-600 dark:text-blue-400">
            <Smartphone class="size-4" />
          </div>
          <div>
            <div class="text-muted-foreground text-xs">{{ t('billing.overviewInstanceSeat') }}</div>
            <div class="text-lg font-semibold tabular-nums">{{ instanceSeat }} <span class="text-muted-foreground text-sm font-normal">{{ t('billing.unitTai') }}</span></div>
          </div>
        </CardContent>
      </Card>

      <!-- 开机席位 -->
      <Card>
        <CardContent class="flex items-center gap-3 py-4">
          <div class="flex size-9 items-center justify-center rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400">
            <Cpu class="size-4" />
          </div>
          <div>
            <div class="text-muted-foreground text-xs">{{ t('billing.overviewBootSeat') }}</div>
            <div class="text-lg font-semibold tabular-nums">{{ bootSeat }} <span class="text-muted-foreground text-sm font-normal">{{ t('billing.unitTai') }}</span></div>
          </div>
        </CardContent>
      </Card>

      <!-- 时长余额 -->
      <Card>
        <CardContent class="flex items-center gap-3 py-4">
          <div class="bg-primary/10 text-primary flex size-9 items-center justify-center rounded-lg">
            <Clock class="size-4" />
          </div>
          <div>
            <div class="text-muted-foreground text-xs">{{ t('billing.overviewRuntime') }}</div>
            <div class="text-lg font-semibold tabular-nums">{{ runtimeMinute }}</div>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- 资源批次明细 -->
    <Card>
      <CardHeader>
        <CardTitle class="text-base">{{ t('billing.batchTitle') }}</CardTitle>
        <CardDescription>{{ t('billing.batchDesc') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <DataTable
          :columns="columns"
          :data="batches"
          :loading="loading"
        >
          <template #cell-subject="{ row }">
            <Badge :variant="subjectVariant(row.subject)">
              {{ t(`billing.subject_${row.subject}`) }}
            </Badge>
          </template>
          <template #cell-quantity="{ row }">
            <span class="tabular-nums">{{ row.quantity }} {{ subjectUnit(row.subject) }}</span>
          </template>
          <template #cell-used="{ row }">
            <span class="tabular-nums">{{ row.used }} {{ subjectUnit(row.subject) }}</span>
          </template>
          <template #cell-remaining="{ row }">
            <span class="tabular-nums font-medium">{{ row.quantity - row.used }} {{ subjectUnit(row.subject) }}</span>
          </template>
          <template #cell-expire_at="{ row }">
            <span class="text-muted-foreground tabular-nums">{{ row.expire_at ? formatDate(row.expire_at) : t('billing.permanent') }}</span>
          </template>
          <template #cell-source="{ row }">
            <span class="text-muted-foreground">{{ row.source }}</span>
          </template>
          <template #cell-source_ref="{ row }">
            <span class="font-mono text-xs text-muted-foreground">{{ row.source_ref || '-' }}</span>
          </template>
          <template #cell-created_at="{ row }">
            <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.created_at) }}</span>
          </template>
        </DataTable>
      </CardContent>
    </Card>

    <!-- 运行用量（时长费） -->
    <Card>
      <CardHeader>
        <CardTitle>{{ t('billing.runtimeUsageTitle') }}</CardTitle>
        <CardDescription>{{ t('billing.runtimeUsageDesc') }}</CardDescription>
      </CardHeader>
      <CardContent class="px-0">
        <div v-if="runtimeLoading" class="flex items-center justify-center p-8 text-sm text-muted-foreground">
          {{ t('common.loading', '加载中…') }}
        </div>
        <div v-else-if="!runtimeSlices.length" class="p-8 text-center text-sm text-muted-foreground">
          {{ t('billing.runtimeUsageEmpty') }}
        </div>
        <table v-else class="w-full text-sm">
          <thead class="bg-muted/50 text-xs text-muted-foreground">
            <tr>
              <th class="px-4 py-2 text-left font-medium">{{ t('billing.rtWindow') }}</th>
              <th class="px-4 py-2 text-right font-medium">{{ t('billing.rtBillable') }}</th>
              <th class="px-4 py-2 text-right font-medium">{{ t('billing.rtCovered') }}</th>
              <th class="px-4 py-2 text-right font-medium">{{ t('billing.rtPack') }}</th>
              <th class="px-4 py-2 text-right font-medium">{{ t('billing.rtBalance') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="s in runtimeSlices" :key="s.id" class="border-t">
              <td class="px-4 py-2 tabular-nums text-muted-foreground">{{ formatDateTime(s.window_start) }} ~ {{ formatDateTime(s.window_end) }}</td>
              <td class="px-4 py-2 text-right tabular-nums">{{ s.billable_unit_minutes }}</td>
              <td class="px-4 py-2 text-right tabular-nums text-green-600 dark:text-green-400">{{ s.covered_seat_minutes }}</td>
              <td class="px-4 py-2 text-right tabular-nums">{{ s.charged_pack_minutes }}</td>
              <td class="px-4 py-2 text-right tabular-nums">{{ fmtCents(s.charged_balance_cents) }}</td>
            </tr>
          </tbody>
        </table>
        <div v-if="runtimeTotal > RT_SIZE" class="flex items-center justify-end gap-2 px-4 pt-3 text-xs text-muted-foreground">
          <Button variant="outline" size="icon" class="size-7" :disabled="runtimePage <= 1 || runtimeLoading" @click="goRuntime(runtimePage - 1)">
            <ChevronLeft class="size-4" />
          </Button>
          <span class="tabular-nums">{{ runtimePage }} / {{ runtimePages }}</span>
          <Button variant="outline" size="icon" class="size-7" :disabled="runtimePage >= runtimePages || runtimeLoading" @click="goRuntime(runtimePage + 1)">
            <ChevronRight class="size-4" />
          </Button>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
