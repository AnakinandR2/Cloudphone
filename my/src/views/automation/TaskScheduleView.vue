<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { AutomationPlan } from '@/types/automation'
import { CalendarClock, Pause, Play, Plus, Repeat, Trash2 } from 'lucide-vue-next'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import automationApi from '@/api/modules/automation'
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
import { useQuerySync } from '@/composables/useQuerySync'
import { ACTIONS_COLUMN_META } from '@/lib/table'
import { planStatusBadge } from '@/utils/statusBadge'
import TaskCreateDialog from './TaskCreateDialog.vue'
import TaskReportDialog from './TaskReportDialog.vue'

const { t } = useI18n()

const data = ref<AutomationPlan[]>([])
const loading = ref(false)
const filters = reactive({ q: '' })
useQuerySync(filters, { q: '' })
const createDialog = ref(false)
const reportDialog = ref<{ open: boolean, midTaskId: number | null }>({ open: false, midTaskId: null })

const columns = computed<ColumnDef<AutomationPlan>[]>(() => [
  { accessorKey: 'name', id: 'name', header: t('taskSchedule.colName'), meta: { label: 'taskSchedule.colName' } },
  { accessorKey: 'scriptName', id: 'scriptName', header: t('taskSchedule.colScript'), meta: { label: 'taskSchedule.colScript' } },
  { id: 'targets', header: t('taskSchedule.colTargets'), meta: { label: 'taskSchedule.colTargets' } },
  { id: 'schedule', header: t('taskSchedule.colSchedule'), meta: { label: 'taskSchedule.colSchedule' } },
  { accessorKey: 'status', id: 'status', header: t('taskSchedule.colStatus'), meta: { label: 'taskSchedule.colStatus' } },
  { id: 'actions', header: t('crud.actions'), enableHiding: false, meta: ACTIONS_COLUMN_META },
])

async function load() {
  loading.value = true
  try {
    const { data: d } = await automationApi.listPlans()
    data.value = d ?? []
  }
  finally {
    loading.value = false
  }
}
onMounted(load)

function scheduleText(p: AutomationPlan) {
  if (p.frequency === 'INTERVAL')
    return t('taskSchedule.everyMin', { n: p.intervalValue })
  return t('taskSchedule.everyDay', { time: (p.executionTime || '').slice(0, 5) })
}

async function start(p: AutomationPlan) {
  await automationApi.startPlan(p.id)
  toast.success(t('taskSchedule.startOk'))
  load()
}
async function pause(p: AutomationPlan) {
  await automationApi.pausePlan(p.id)
  toast.success(t('taskSchedule.pauseOk'))
  load()
}
async function remove(p: AutomationPlan) {
  await automationApi.deletePlan(p.id)
  toast.success(t('crud.deleteOk'))
  load()
}
function onCreated(firstTaskId: number | null) {
  load()
  if (firstTaskId != null)
    reportDialog.value = { open: true, midTaskId: firstTaskId }
}
</script>

<template>
  <Card>
    <CardHeader>
      <div class="flex items-start justify-between gap-4">
        <div class="space-y-1.5">
          <CardTitle>{{ t('taskSchedule.title') }}</CardTitle>
          <CardDescription>{{ t('taskSchedule.desc') }}</CardDescription>
        </div>
        <Button size="sm" class="shrink-0" @click="createDialog = true">
          <Plus class="size-4" /> {{ t('taskSchedule.newTask') }}
        </Button>
      </div>
    </CardHeader>
    <CardContent>
      <DataTable
        v-model:search-value="filters.q"
        pin-actions-column
        :columns="columns"
        :data="data"
        :loading="loading"
        :get-row-id="(r) => String(r.id)"
        :search-label="t('taskSchedule.searchLabel')"
        :search-placeholder="t('taskSchedule.searchPlaceholder')"
      >
        <template #cell-name="{ row }">
          <span class="font-medium">{{ row.name }}</span>
        </template>
        <template #cell-scriptName="{ row }">
          <span class="text-muted-foreground">{{ row.scriptName }}</span>
        </template>
        <template #cell-targets="{ row }">
          <span class="tabular-nums">{{ t('taskSchedule.targetsN', { n: row.cpIds.length }) }}</span>
        </template>
        <template #cell-schedule="{ row }">
          <Badge variant="secondary" class="font-normal">
            <component :is="row.frequency === 'INTERVAL' ? Repeat : CalendarClock" class="mr-1 size-3" />
            {{ scheduleText(row) }}
          </Badge>
        </template>
        <template #cell-status="{ row }">
          <Badge v-bind="planStatusBadge(row.status)">{{ t(`taskSchedule.planStatus_${row.status}`) }}</Badge>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex items-center justify-end gap-2">
            <Button v-if="row.status !== 'ENABLING'" variant="ghost" size="icon" class="size-7" :title="t('taskSchedule.start')" @click="start(row)">
              <Play class="size-3.5" />
            </Button>
            <Button v-else variant="ghost" size="icon" class="size-7" :title="t('taskSchedule.pause')" @click="pause(row)">
              <Pause class="size-3.5" />
            </Button>
            <Popconfirm :title="t('taskSchedule.delConfirm', { name: row.name })" tone="danger" @confirm="remove(row)">
              <Button variant="ghost" size="icon" class="size-7 text-destructive hover:text-destructive" :title="t('crud.delete')">
                <Trash2 class="size-3.5" />
              </Button>
            </Popconfirm>
          </div>
        </template>
      </DataTable>
    </CardContent>

    <TaskCreateDialog v-model="createDialog" @created="onCreated" />
    <TaskReportDialog v-model="reportDialog.open" :mid-task-id="reportDialog.midTaskId" />
  </Card>
</template>
