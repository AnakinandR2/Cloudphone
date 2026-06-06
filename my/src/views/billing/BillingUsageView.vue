<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import { Activity, Clock, Smartphone } from 'lucide-vue-next'
import { computed, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import DataTable from '@/components/DataTable.vue'
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
import UsageBarChart from './UsageBarChart.vue'
import UsageLineChart from './UsageLineChart.vue'

const { t } = useI18n()

// 确定性伪随机（按下标），保证图表/表格在重渲染间稳定，不依赖 Math.random。
function seeded(i: number, salt: number) {
  const x = Math.sin((i + 1) * 12.9898 + salt * 78.233) * 43758.5453
  return x - Math.floor(x)
}

// range = 图表/数据窗口；from/to = 时长消耗明细的独立时间段过滤。
const filters = reactive({ range: '30', instance: 'all', from: '', to: '', q: '' })
useQuerySync(filters, { range: '30', instance: 'all', from: '', to: '', q: '' })

const ranges = [
  { key: '7', days: 7 },
  { key: '30', days: 30 },
  { key: '90', days: 90 },
  { key: '365', days: 365 },
]
const rangeDays = computed(() => ranges.find(r => r.key === filters.range)?.days ?? 30)

const days = computed(() => {
  const n = rangeDays.value
  const today = new Date()
  const out: { label: string, date: Date }[] = []
  for (let k = n - 1; k >= 0; k--) {
    const d = new Date(today)
    d.setDate(today.getDate() - k)
    d.setHours(0, 0, 0, 0)
    out.push({ label: `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`, date: d })
  }
  return out
})

const instanceSeries = computed(() => days.value.map((d, i) => ({ label: d.label, value: Math.round(seeded(i, 1) * 6) })))
const hoursSeries = computed(() => days.value.map((d, i) => ({ label: d.label, value: Math.round(20 + seeded(i, 2) * 180) })))

const totalInstances = computed(() => instanceSeries.value.reduce((s, d) => s + d.value, 0))
const totalHours = computed(() => hoursSeries.value.reduce((s, d) => s + d.value, 0))
// 时长是预购的时长包，消耗不再产生费用；此处给「日均开机时长」而非金额。
const avgHours = computed(() => Math.round(totalHours.value / Math.max(1, rangeDays.value)))

// —— 时长消耗明细（原型确定性数据）。时长包消耗按小时扣减，无单条费用。——
const INSTANCES = ['cp-7f3a', 'cp-9b21', 'cp-1c08', 'cp-22e5', 'cp-4d70', 'cp-8a13']
interface UsageRow { id: number, instance: string, period: string, hours: number, ts: number }
const allRecords = computed<UsageRow[]>(() => {
  const out: UsageRow[] = []
  let id = 0
  for (let i = 0; i < rangeDays.value; i++) {
    const d = days.value[i]
    const cnt = Math.floor(seeded(i, 3) * 3) + 1
    for (let j = 0; j < cnt; j++) {
      const inst = INSTANCES[Math.floor(seeded(i * 7 + j, 4) * INSTANCES.length)]
      const hours = +(1 + seeded(i * 5 + j, 5) * 11).toFixed(1)
      const startH = Math.floor(seeded(i * 3 + j, 6) * 12)
      const endH = Math.min(23, startH + Math.ceil(hours))
      const start = new Date(d.date)
      start.setHours(startH, 0, 0, 0)
      out.push({
        id: ++id,
        instance: inst,
        period: `${d.label} ${String(startH).padStart(2, '0')}:00 ~ ${String(endH).padStart(2, '0')}:00`,
        hours,
        ts: start.getTime(),
      })
    }
  }
  return out.reverse()
})

const fromTs = computed(() => (filters.from ? new Date(filters.from).getTime() : null))
const toTs = computed(() => (filters.to ? new Date(filters.to).getTime() : null))
const records = computed(() => allRecords.value.filter(r =>
  (filters.instance === 'all' || r.instance === filters.instance)
  && (fromTs.value === null || r.ts >= fromTs.value)
  && (toTs.value === null || r.ts <= toTs.value),
))

const columns = computed<ColumnDef<UsageRow>[]>(() => [
  { accessorKey: 'instance', id: 'instance', header: t('billing.colInstance'), meta: { label: 'billing.colInstance' } },
  { accessorKey: 'period', id: 'period', header: t('billing.colPeriod'), meta: { label: 'billing.colPeriod' } },
  { accessorKey: 'hours', id: 'hours', header: t('billing.colHours'), meta: { label: 'billing.colHours' } },
])
</script>

<template>
  <div class="flex flex-col gap-6">
    <!-- 标题 + 说明 + 时间选择 -->
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 class="text-xl font-semibold tracking-tight">{{ t('billing.usageTitle') }}</h1>
        <p class="text-muted-foreground mt-1 text-sm">{{ t('billing.usageDesc') }}</p>
      </div>
      <Select v-model="filters.range">
        <SelectTrigger class="h-9 w-32">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem v-for="r in ranges" :key="r.key" :value="r.key">
            {{ t(`billing.range_${r.key}`) }}
          </SelectItem>
        </SelectContent>
      </Select>
    </div>

    <!-- 区间汇总 -->
    <div class="grid gap-4 sm:grid-cols-3">
      <Card>
        <CardContent class="flex items-center gap-3 py-4">
          <div class="flex size-9 items-center justify-center rounded-lg bg-blue-500/10 text-blue-600 dark:text-blue-400">
            <Smartphone class="size-4" />
          </div>
          <div>
            <div class="text-muted-foreground text-xs">{{ t('billing.sumInstances') }}</div>
            <div class="text-lg font-semibold tabular-nums">{{ totalInstances }} {{ t('billing.unit') }}</div>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="flex items-center gap-3 py-4">
          <div class="flex size-9 items-center justify-center rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400">
            <Clock class="size-4" />
          </div>
          <div>
            <div class="text-muted-foreground text-xs">{{ t('billing.sumHours') }}</div>
            <div class="text-lg font-semibold tabular-nums">{{ totalHours }} {{ t('billing.hoursUnit') }}</div>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="flex items-center gap-3 py-4">
          <div class="bg-primary/10 text-primary flex size-9 items-center justify-center rounded-lg">
            <Activity class="size-4" />
          </div>
          <div>
            <div class="text-muted-foreground text-xs">{{ t('billing.avgHours') }}</div>
            <div class="text-lg font-semibold tabular-nums">{{ avgHours }} {{ t('billing.hoursUnit') }}</div>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- 图表 -->
    <div class="grid gap-6 xl:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle class="text-base">{{ t('billing.chartInstances') }}</CardTitle>
          <CardDescription>{{ t('billing.chartInstancesDesc') }}</CardDescription>
        </CardHeader>
        <CardContent>
          <UsageLineChart :data="instanceSeries" :unit="t('billing.unit')" />
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle class="text-base">{{ t('billing.chartHours') }}</CardTitle>
          <CardDescription>{{ t('billing.chartHoursDesc') }}</CardDescription>
        </CardHeader>
        <CardContent>
          <UsageBarChart :data="hoursSeries" :unit="t('billing.hoursUnit')" />
        </CardContent>
      </Card>
    </div>

    <!-- 时长消耗明细 -->
    <Card>
      <CardHeader>
        <CardTitle class="text-base">{{ t('billing.tableTitle') }}</CardTitle>
        <CardDescription>{{ t('billing.tableDesc') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <DataTable
          v-model:search-value="filters.q"
          :columns="columns"
          :data="records"
          :search-placeholder="t('billing.searchInstance')"
        >
          <template #filters>
            <Select v-model="filters.instance">
              <SelectTrigger class="h-9 w-40">
                <SelectValue :placeholder="t('billing.allInstances')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">
                  {{ t('billing.allInstances') }}
                </SelectItem>
                <SelectItem v-for="ins in INSTANCES" :key="ins" :value="ins">
                  {{ ins }}
                </SelectItem>
              </SelectContent>
            </Select>
            <!-- 时间段：独立的日期时间范围过滤 -->
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
          <template #cell-instance="{ row }">
            <span class="font-mono text-xs">{{ row.instance }}</span>
          </template>
          <template #cell-period="{ row }">
            <span class="text-muted-foreground tabular-nums">{{ row.period }}</span>
          </template>
          <template #cell-hours="{ row }">
            <span class="tabular-nums">{{ row.hours }} {{ t('billing.hoursUnit') }}</span>
          </template>
        </DataTable>
      </CardContent>
    </Card>
  </div>
</template>
