<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { AutomationTask } from '@/types/automation'
import { CheckCircle2, Eye, Loader2, XCircle } from 'lucide-vue-next'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import automationApi from '@/api/modules/automation'
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
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { useQuerySync } from '@/composables/useQuerySync'
import { formatDateTime } from '@/utils/date'
import TaskReportDialog from './TaskReportDialog.vue'

const { t } = useI18n()

const data = ref<AutomationTask[]>([])
const loading = ref(false)
const filters = reactive({ q: '', status: '' })
useQuerySync(filters, { q: '', status: '' })
const reportDialog = ref<{ open: boolean, midTaskId: number | null }>({ open: false, midTaskId: null })

const columns = computed<ColumnDef<AutomationTask>[]>(() => [
  { accessorKey: 'taskName', id: 'taskName', header: t('taskLog.colTask'), meta: { label: 'taskLog.colTask' } },
  { accessorKey: 'scriptName', id: 'scriptName', header: t('taskLog.colScript'), meta: { label: 'taskLog.colScript' } },
  { accessorKey: 'cpId', id: 'cpId', header: t('taskLog.colPhone'), meta: { label: 'taskLog.colPhone' } },
  { id: 'trigger', header: t('taskLog.colTrigger'), meta: { label: 'taskLog.colTrigger' } },
  { accessorKey: 'runStart', id: 'runStart', header: t('taskLog.colStart'), meta: { label: 'taskLog.colStart' } },
  { accessorKey: 'status', id: 'status', header: t('taskLog.colStatus'), meta: { label: 'taskLog.colStatus' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
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
  success: { icon: CheckCircle2, cls: 'bg-green-100 text-green-700 dark:bg-green-950 dark:text-green-300' },
  failed: { icon: XCircle, cls: 'bg-destructive/10 text-destructive' },
  running: { icon: Loader2, cls: 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300' },
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
        :columns="columns"
        :data="data"
        :loading="loading"
        :get-row-id="(r) => String(r.id)"
        :search-placeholder="t('taskLog.searchPlaceholder')"
      >
        <template #filters>
          <NativeSelect v-model="filters.status" class="h-9 w-36 text-xs">
            <NativeSelectOption value="">
              {{ t('taskLog.statusAll') }}
            </NativeSelectOption>
            <NativeSelectOption value="COMPLETED">
              {{ t('taskLog.success') }}
            </NativeSelectOption>
            <NativeSelectOption value="FAILED">
              {{ t('taskLog.failed') }}
            </NativeSelectOption>
            <NativeSelectOption value="EXECUTING">
              {{ t('taskLog.running') }}
            </NativeSelectOption>
          </NativeSelect>
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
          <Badge variant="secondary" :class="statusMeta[statusKind(row.status)].cls">
            <component :is="statusMeta[statusKind(row.status)].icon" class="mr-1 size-3" :class="statusKind(row.status) === 'running' && 'animate-spin'" />
            {{ row.status }}
          </Badge>
        </template>
        <template #cell-actions="{ row }">
          <Button variant="ghost" size="sm" @click="view(row)">
            <Eye class="size-3.5" /> {{ t('taskLog.view') }}
          </Button>
        </template>
      </DataTable>
    </CardContent>

    <TaskReportDialog v-model="reportDialog.open" :mid-task-id="reportDialog.midTaskId" />
  </Card>
</template>
