<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { AutomationTask } from '@/types/automation'
import { CheckCircle2, Eye, Loader2, XCircle } from 'lucide-vue-next'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import automationApi from '@/api/modules/automation'
import DataTable from '@/components/DataTable.vue'
import FilterBar from '@/components/FilterBar.vue'
import FilterField from '@/components/FilterField.vue'
import FilterSearchInput from '@/components/FilterSearchInput.vue'
import {
  filterPopupAlign,
  filterPopupSideOffset,
  filterSelectContentClass,
  filterSelectTriggerClass,
} from '@/components/filterField'
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
import { useQuerySync } from '@/composables/useQuerySync'
import { ACTIONS_COLUMN_META } from '@/lib/table'
import { formatDateTime } from '@/utils/date'
import { taskLogStatusBadge } from '@/utils/statusBadge'
import TaskReportDialog from './TaskReportDialog.vue'

const { t } = useI18n()

const data = ref<AutomationTask[]>([])
const loading = ref(false)
const filters = reactive({ q: '', status: '' })
useQuerySync(filters, { q: '', status: '' })
const ALL_STATUS_VALUE = '__all__'
const statusFilterModel = computed({
  get: () => filters.status || ALL_STATUS_VALUE,
  set: value => (filters.status = value === ALL_STATUS_VALUE ? '' : value),
})
const reportDialog = ref<{ open: boolean, midTaskId: number | null }>({ open: false, midTaskId: null })

const columns = computed<ColumnDef<AutomationTask>[]>(() => [
  { accessorKey: 'taskName', id: 'taskName', header: t('taskLog.colTask'), meta: { label: 'taskLog.colTask' } },
  { accessorKey: 'scriptName', id: 'scriptName', header: t('taskLog.colScript'), meta: { label: 'taskLog.colScript' } },
  { accessorKey: 'cpId', id: 'cpId', header: t('taskLog.colPhone'), meta: { label: 'taskLog.colPhone' } },
  { id: 'trigger', header: t('taskLog.colTrigger'), meta: { label: 'taskLog.colTrigger' } },
  { accessorKey: 'runStart', id: 'runStart', header: t('taskLog.colStart'), meta: { label: 'taskLog.colStart' } },
  { accessorKey: 'status', id: 'status', header: t('taskLog.colStatus'), meta: { label: 'taskLog.colStatus' } },
  { id: 'actions', header: t('crud.actions'), enableHiding: false, meta: ACTIONS_COLUMN_META },
])

async function load() {
  loading.value = true
  try {
    const { data: d } = await automationApi.listTasks({ page: 1, size: 200, status: filters.status || undefined })
    data.value = d.list ?? []
  }
  finally {
    loading.value = false
  }
}
onMounted(load)
watch(() => filters.status, load)

function statusKind(s: string): 'success' | 'failed' | 'running' {
  if (s === 'COMPLETED')
    return 'success'
  if (s === 'FAILED' || s === 'CANCELLED')
    return 'failed'
  return 'running'
}
const statusMeta = {
  success: { icon: CheckCircle2 },
  failed: { icon: XCircle },
  running: { icon: Loader2 },
} as const

function view(row: AutomationTask) {
  reportDialog.value = { open: true, midTaskId: row.midTaskId }
}
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ t('taskLog.title') }}</CardTitle>
      <CardDescription>{{ t('taskLog.desc') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <DataTable
        v-model:search-value="filters.q"
        pin-actions-column
        :columns="columns"
        :data="data"
        :loading="loading"
        :search="false"
        :get-row-id="(r) => String(r.id)"
      >
        <template #filters>
          <FilterBar class="min-w-0 flex-1">
            <FilterField :label="t('taskLog.searchLabel')">
              <FilterSearchInput v-model="filters.q" :placeholder="t('taskLog.searchPlaceholder')" />
            </FilterField>
            <FilterField :label="t('taskLog.filterStatus')">
              <Select v-model="statusFilterModel" class="flex h-full min-w-0 flex-1">
                <SelectTrigger :class="filterSelectTriggerClass">
                  <SelectValue :placeholder="t('taskLog.statusAll')" />
                </SelectTrigger>
                <SelectContent
                  :align="filterPopupAlign"
                  :side-offset="filterPopupSideOffset"
                  :class="filterSelectContentClass"
                >
                  <SelectItem :value="ALL_STATUS_VALUE">
                    {{ t('taskLog.statusAll') }}
                  </SelectItem>
                  <SelectItem value="COMPLETED">
                    {{ t('taskLog.success') }}
                  </SelectItem>
                  <SelectItem value="FAILED">
                    {{ t('taskLog.failed') }}
                  </SelectItem>
                  <SelectItem value="EXECUTING">
                    {{ t('taskLog.running') }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </FilterField>
          </FilterBar>
        </template>

        <template #cell-taskName="{ row }">
          <span class="font-medium">{{ row.taskName }}</span>
        </template>
        <template #cell-scriptName="{ row }">
          <span class="text-muted-foreground">{{ row.scriptName }}</span>
        </template>
        <template #cell-cpId="{ row }">
          <code class="font-mono text-xs">{{ row.cpId }}</code>
        </template>
        <template #cell-trigger="{ row }">
          <span class="text-muted-foreground">{{ row.trigger === 'plan' ? t('taskLog.triggerPlan') : t('taskLog.triggerManual') }}</span>
        </template>
        <template #cell-runStart="{ row }">
          <span class="tabular-nums text-muted-foreground">{{ row.runStart ? formatDateTime(row.runStart) : '—' }}</span>
        </template>
        <template #cell-status="{ row }">
          <Badge v-bind="taskLogStatusBadge(statusKind(row.status))">
            <component :is="statusMeta[statusKind(row.status)].icon" class="mr-1 size-3" :class="statusKind(row.status) === 'running' && 'animate-spin'" />
            {{ row.status }}
          </Badge>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex items-center justify-end gap-2">
            <Button variant="ghost" size="sm" @click="view(row)">
              <Eye class="size-3.5" /> {{ t('taskLog.view') }}
            </Button>
          </div>
        </template>
      </DataTable>
    </CardContent>

    <TaskReportDialog v-model="reportDialog.open" :mid-task-id="reportDialog.midTaskId" />
  </Card>
</template>
