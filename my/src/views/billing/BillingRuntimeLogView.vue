<script setup lang="ts">
import type { RuntimeLogEntry, RuntimeLogSegment } from '@/types/billing'
import { ChevronRight, Info } from 'lucide-vue-next'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import billingApi from '@/api/modules/billing'
import FilterBar from '@/components/FilterBar.vue'
import FilterField from '@/components/FilterField.vue'
import { filterInputClass } from '@/components/filterField'
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { formatDateTime } from '@/utils/date'

// 费用日志页：聚合到「开机会话」的费用记录。
// 契约 §1.6：实例 / 开机时段（含运行中）/ 占用类型 / 扣临时时长合计；混合段可展开看 segments；
// 顶部提示当天 200 封顶规则。
const { t } = useI18n()

const items = ref<RuntimeLogEntry[]>([])
const total = ref(0)
const dailyCap = ref(200)
const loading = ref(false)
const page = ref(1)
const size = ref(20)
const expanded = ref<Set<string>>(new Set())

// 时间段筛选：datetime-local 字符串（本地时区），查询时转 ISO 传给后端。
const fromInput = ref('')
const toInput = ref('')

const sizeOptions = [20, 50, 100, 200]

function rowKey(e: RuntimeLogEntry) {
  return `${e.cp_id}-${e.power_on_at}`
}

// "2026-06-21T23:00"（本地）→ ISO；空串返回 undefined（不传该参数）。
function toIso(local: string): string | undefined {
  if (!local) return undefined
  const d = new Date(local)
  return Number.isNaN(d.getTime()) ? undefined : d.toISOString()
}

async function load(silent = false) {
  if (!silent) loading.value = true
  try {
    const res = await billingApi.runtimeLog({
      page: page.value,
      size: size.value,
      from: toIso(fromInput.value),
      to: toIso(toInput.value),
    })
    items.value = res.data.items
    total.value = res.data.total
    dailyCap.value = res.data.daily_cap_minutes
  }
  finally {
    loading.value = false
  }
}

onMounted(() => load())

// 应用筛选：回到第 1 页重新拉取。
function applyFilter() {
  page.value = 1
  load()
}
// 清空筛选条件并刷新。
function resetFilter() {
  fromInput.value = ''
  toInput.value = ''
  page.value = 1
  load()
}

const hasFilter = computed(() => !!fromInput.value || !!toInput.value)

function toggle(e: RuntimeLogEntry) {
  const k = rowKey(e)
  if (expanded.value.has(k)) expanded.value.delete(k)
  else expanded.value.add(k)
  // 触发响应式
  expanded.value = new Set(expanded.value)
}

function quotaVariant(q: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  if (q === 'boot_slot') return 'default'
  if (q === 'temp') return 'secondary'
  if (q === 'mixed') return 'outline'
  return 'outline' // capped_free
}

const pageCount = computed(() => Math.max(1, Math.ceil(total.value / size.value)))

function changeSize(v: unknown) {
  size.value = Number(v)
  page.value = 1
  load()
}
function prev() {
  if (page.value <= 1) return
  page.value--
  load()
}
function next() {
  if (page.value >= pageCount.value) return
  page.value++
  load()
}

// 混合段（中途名额释放或撞封顶）才有展开价值；segments 多于 1 段即可展开。
function hasSegments(e: RuntimeLogEntry) {
  return e.segments && e.segments.length > 1
}
function segMinutes(segs: RuntimeLogSegment[]) {
  return segs.reduce((s, x) => s + x.minutes, 0)
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <div>
      <h1 class="text-xl font-semibold tracking-tight">
        {{ t('billing.runtimeLog.title') }}
      </h1>
      <p class="text-muted-foreground mt-1 text-sm">
        {{ t('billing.runtimeLog.desc') }}
      </p>
    </div>

    <!-- 顶部封顶规则提示 -->
    <div class="bg-amber-500/10 text-amber-700 dark:text-amber-400 flex items-start gap-2.5 rounded-lg border border-amber-500/30 px-3.5 py-3 text-sm">
      <Info class="mt-0.5 size-4 shrink-0" />
      <p class="leading-relaxed">
        {{ t('billing.runtimeLog.capHint', { cap: dailyCap }) }}
      </p>
    </div>

    <Card>
      <CardHeader>
        <CardTitle>{{ t('billing.runtimeLog.sessionsTitle') }}</CardTitle>
        <CardDescription>{{ t('billing.runtimeLog.sessionsDesc') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <!-- 时间段筛选 -->
        <div class="mb-4 flex flex-wrap items-center gap-3">
          <FilterBar class="min-w-0 flex-1">
            <FilterField :label="t('billing.runtimeLog.filterFrom')">
              <input
                v-model="fromInput"
                type="datetime-local"
                :class="filterInputClass"
              >
            </FilterField>
            <FilterField :label="t('billing.runtimeLog.filterTo')">
              <input
                v-model="toInput"
                type="datetime-local"
                :class="filterInputClass"
              >
            </FilterField>
          </FilterBar>
          <Button size="sm" class="h-10" @click="applyFilter">
            {{ t('billing.runtimeLog.filterApply') }}
          </Button>
          <Button v-if="hasFilter" variant="outline" size="sm" class="h-10" @click="resetFilter">
            {{ t('billing.runtimeLog.filterReset') }}
          </Button>
        </div>

        <!-- 加载骨架 -->
        <div v-if="loading" class="space-y-2">
          <Skeleton v-for="i in 6" :key="i" class="h-12 w-full rounded-md" />
        </div>

        <div v-else-if="!items.length" class="text-muted-foreground py-16 text-center text-sm">
          {{ t('billing.runtimeLog.empty') }}
        </div>

        <div v-else class="overflow-hidden rounded-lg border">
          <!-- 表头 -->
          <div class="bg-muted/50 text-muted-foreground grid grid-cols-[1.4fr_2fr_1fr_1fr_2rem] gap-2 px-3 py-2 text-xs font-medium">
            <span>{{ t('billing.runtimeLog.colInstance') }}</span>
            <span>{{ t('billing.runtimeLog.colPeriod') }}</span>
            <span>{{ t('billing.runtimeLog.colQuota') }}</span>
            <span class="text-right">{{ t('billing.runtimeLog.colTempMinutes') }}</span>
            <span />
          </div>

          <template v-for="e in items" :key="rowKey(e)">
            <!-- 会话行 -->
            <div
              class="grid grid-cols-[1.4fr_2fr_1fr_1fr_2rem] items-center gap-2 border-t px-3 py-2.5 text-sm"
              :class="hasSegments(e) ? 'hover:bg-muted/40 cursor-pointer' : ''"
              @click="hasSegments(e) && toggle(e)"
            >
              <div class="min-w-0">
                <div class="truncate font-medium">
                  {{ e.instance_name }}
                </div>
                <div class="text-muted-foreground truncate font-mono text-xs">
                  {{ e.cp_id }}
                </div>
              </div>
              <div class="text-muted-foreground tabular-nums text-xs">
                {{ formatDateTime(e.power_on_at) }}
                <span class="mx-1">→</span>
                <span v-if="e.running" class="text-emerald-600 dark:text-emerald-400 font-medium">{{ t('billing.runtimeLog.running') }}</span>
                <span v-else>{{ formatDateTime(e.power_off_at) }}</span>
              </div>
              <div>
                <Badge :variant="quotaVariant(e.quota_type)" class="text-[10px]">
                  {{ t(`billing.runtimeLog.quota_${e.quota_type}`) }}
                </Badge>
              </div>
              <div class="text-right tabular-nums">
                {{ e.temp_minutes_charged }} <span class="text-muted-foreground text-xs">{{ t('billing.purchase2.minuteUnit') }}</span>
              </div>
              <div class="flex justify-center">
                <ChevronRight
                  v-if="hasSegments(e)"
                  class="text-muted-foreground size-4 transition-transform"
                  :class="expanded.has(rowKey(e)) && 'rotate-90'"
                />
              </div>
            </div>

            <!-- 展开：混合段分段明细 -->
            <div v-if="hasSegments(e) && expanded.has(rowKey(e))" class="bg-muted/30 border-t px-3 py-2">
              <div class="text-muted-foreground mb-1.5 text-xs font-medium">
                {{ t('billing.runtimeLog.segmentsTitle', { total: segMinutes(e.segments) }) }}
              </div>
              <div class="space-y-1">
                <div
                  v-for="(seg, i) in e.segments"
                  :key="i"
                  class="rounded-md bg-background px-2.5 py-1.5 text-xs"
                >
                  <div class="grid grid-cols-[1fr_2fr_1fr] items-center gap-2">
                    <Badge :variant="quotaVariant(seg.quota_type)" class="w-fit text-[10px]">
                      {{ t(`billing.runtimeLog.quota_${seg.quota_type}`) }}
                    </Badge>
                    <span class="text-muted-foreground tabular-nums">{{ formatDateTime(seg.from) }} → {{ formatDateTime(seg.to) }}</span>
                    <span class="text-right tabular-nums">{{ seg.minutes }} {{ t('billing.purchase2.minuteUnit') }}</span>
                  </div>
                  <!-- 该段为何这样计费：优先用后端落账时生成的具体原因（含名额数/封顶值等），
                       旧数据无 reason 时回退到按 quota_type 的通用说明。 -->
                  <p class="text-muted-foreground/80 mt-1 leading-snug">
                    {{ seg.reason || t(`billing.runtimeLog.reason_${seg.quota_type}`) }}
                  </p>
                </div>
              </div>
            </div>
          </template>
        </div>

        <!-- 分页 -->
        <div v-if="items.length" class="mt-4 flex items-center justify-between gap-3">
          <span class="text-muted-foreground text-sm">{{ t('billing.runtimeLog.totalCount', { total }) }}</span>
          <div class="flex items-center gap-2">
            <Select :model-value="String(size)" @update:model-value="changeSize">
              <SelectTrigger class="h-8 w-24">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="s in sizeOptions" :key="s" :value="String(s)">
                  {{ s }} / {{ t('billing.runtimeLog.perPage') }}
                </SelectItem>
              </SelectContent>
            </Select>
            <button type="button" class="cursor-pointer rounded-md border px-2.5 py-1 text-sm disabled:opacity-50" :disabled="page <= 1" @click="prev">
              {{ t('billing.runtimeLog.prev') }}
            </button>
            <span class="text-muted-foreground text-sm tabular-nums">{{ page }} / {{ pageCount }}</span>
            <button type="button" class="cursor-pointer rounded-md border px-2.5 py-1 text-sm disabled:opacity-50" :disabled="page >= pageCount" @click="next">
              {{ t('billing.runtimeLog.next') }}
            </button>
          </div>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
