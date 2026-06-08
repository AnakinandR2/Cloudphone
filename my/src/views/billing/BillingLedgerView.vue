<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { LedgerEntry } from '@/types/billing'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import billingApi from '@/api/modules/billing'
import DataTable from '@/components/DataTable.vue'
import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
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
const data = ref<LedgerEntry[]>([])
const loading = ref(false)

// ——— 筛选 ———
const filters = reactive({ subject: 'all', type: 'all', q: '' })
useQuerySync(filters, { subject: 'all', type: 'all', q: '' })

// ——— 加载 ———
async function load() {
  loading.value = true
  try {
    const params: { page: number; size: number; subject?: string; type?: string } = { page: 1, size: 200 }
    if (filters.subject !== 'all') params.subject = filters.subject
    if (filters.type !== 'all') params.type = filters.type
    const res = await billingApi.ledger(params)
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
watch(() => [filters.subject, filters.type], load)

// ——— 单位感知格式化 ———
function fmtDelta(entry: LedgerEntry): string {
  if (entry.subject === 'balance') {
    const sign = entry.delta >= 0 ? '+' : ''
    return `${sign}¥${fmtCents(entry.delta)}`
  }
  const unit = subjectUnit(entry.subject)
  const sign = entry.delta >= 0 ? '+' : ''
  return `${sign}${entry.delta} ${unit}`
}

function fmtBalanceAfter(entry: LedgerEntry): string {
  if (entry.subject === 'balance') {
    return `¥${fmtCents(entry.balance_after)}`
  }
  const unit = subjectUnit(entry.subject)
  return `${entry.balance_after} ${unit}`
}

function subjectUnit(subject: string): string {
  if (subject === 'instance_seat' || subject === 'boot_seat') return t('billing.ledgerUnitTai')
  if (subject === 'runtime_minute') return t('billing.ledgerUnitMin')
  return ''
}

function deltaClass(entry: LedgerEntry): string {
  if (entry.delta > 0) return 'text-green-600 dark:text-green-400'
  if (entry.delta < 0) return 'text-red-600 dark:text-red-400'
  return 'text-muted-foreground'
}

// ——— 科目徽章变体 ———
function subjectVariant(s: string): 'default' | 'secondary' | 'outline' {
  if (s === 'balance') return 'default'
  if (s === 'runtime_minute') return 'secondary'
  return 'outline'
}

// ——— 列定义 ———
const columns = computed<ColumnDef<LedgerEntry>[]>(() => [
  {
    accessorKey: 'created_at',
    id: 'created_at',
    header: t('billing.colCreatedAt'),
    meta: { label: 'billing.colCreatedAt' },
  },
  {
    accessorKey: 'subject',
    id: 'subject',
    header: t('billing.colSubject'),
    meta: { label: 'billing.colSubject' },
  },
  {
    accessorKey: 'type',
    id: 'type',
    header: t('billing.colLedgerType'),
    meta: { label: 'billing.colLedgerType' },
  },
  {
    accessorKey: 'delta',
    id: 'delta',
    header: t('billing.colDelta'),
    meta: { label: 'billing.colDelta' },
  },
  {
    accessorKey: 'balance_after',
    id: 'balance_after',
    header: t('billing.colBalanceAfter'),
    meta: { label: 'billing.colBalanceAfter' },
  },
  {
    accessorKey: 'reason',
    id: 'reason',
    header: t('billing.colReason'),
    meta: { label: 'billing.colReason' },
  },
])

const SUBJECTS = ['balance', 'instance_seat', 'boot_seat', 'runtime_minute'] as const
const TYPES = ['topup', 'consume', 'adjust_grant', 'adjust_deduct', 'purchase', 'trial'] as const
</script>

<template>
  <div class="flex flex-col gap-6">
    <Card>
      <CardHeader>
        <CardTitle>{{ t('billing.ledgerTitle') }}</CardTitle>
        <CardDescription>{{ t('billing.ledgerDesc') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <DataTable
          v-model:search-value="filters.q"
          :columns="columns"
          :data="data"
          :loading="loading"
          :search-placeholder="t('billing.searchLedger')"
        >
          <template #filters>
            <!-- 科目筛选 -->
            <Select v-model="filters.subject">
              <SelectTrigger class="h-9 w-36">
                <SelectValue :placeholder="t('billing.allSubjects')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">
                  {{ t('billing.allSubjects') }}
                </SelectItem>
                <SelectItem v-for="s in SUBJECTS" :key="s" :value="s">
                  {{ t(`billing.subject_${s}`) }}
                </SelectItem>
              </SelectContent>
            </Select>
            <!-- 类型筛选 -->
            <Select v-model="filters.type">
              <SelectTrigger class="h-9 w-36">
                <SelectValue :placeholder="t('billing.allTypes')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">
                  {{ t('billing.allTypes') }}
                </SelectItem>
                <SelectItem v-for="tp in TYPES" :key="tp" :value="tp">
                  {{ t(`billing.ledgerType_${tp}`) }}
                </SelectItem>
              </SelectContent>
            </Select>
          </template>

          <template #cell-created_at="{ row }">
            <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.created_at) }}</span>
          </template>
          <template #cell-subject="{ row }">
            <Badge :variant="subjectVariant(row.subject)">{{ t(`billing.subject_${row.subject}`) }}</Badge>
          </template>
          <template #cell-type="{ row }">
            <span class="text-muted-foreground text-sm">{{ t(`billing.ledgerType_${row.type}`) }}</span>
          </template>
          <template #cell-delta="{ row }">
            <span class="tabular-nums font-medium" :class="deltaClass(row)">{{ fmtDelta(row) }}</span>
          </template>
          <template #cell-balance_after="{ row }">
            <span class="text-muted-foreground tabular-nums">{{ fmtBalanceAfter(row) }}</span>
          </template>
          <template #cell-reason="{ row }">
            <span class="text-muted-foreground max-w-[240px] truncate text-sm">{{ row.reason || '-' }}</span>
          </template>
        </DataTable>
      </CardContent>
    </Card>
  </div>
</template>
