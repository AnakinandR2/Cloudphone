<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import { CheckCircle2, Eye, Loader2, XCircle } from 'lucide-vue-next'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
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

interface Log { id: number, task: string, script: string, phone: string, trigger: string, start: string, duration: string, status: string }
const { t } = useI18n()

const search = ref('')
const statusFilter = ref('')
const logs: Log[] = [
  { id: 1, task: '每日签到 - 早 8 点', script: '每日签到', phone: 'cc-01', trigger: '定时', start: '2026-06-06 08:00:01', duration: '12s', status: 'success' },
  { id: 2, task: '每日签到 - 早 8 点', script: '每日签到', phone: 'cc-07', trigger: '定时', start: '2026-06-06 08:00:01', duration: '9s', status: 'success' },
  { id: 3, task: '养号 - 循环', script: '养号脚本', phone: 'cc-15', trigger: '周期', start: '2026-06-06 16:00:00', duration: '03:21', status: 'running' },
  { id: 4, task: '批量点赞 - 手动', script: '批量点赞', phone: 'cc-22', trigger: '手动', start: '2026-06-05 21:14:33', duration: '7s', status: 'failed' },
  { id: 5, task: '关键词采集 - 工作日', script: '关键词采集', phone: 'cc-03', trigger: '定时', start: '2026-06-05 09:30:02', duration: '41s', status: 'success' },
]
const columns: ColumnDef<Log>[] = [
  { accessorKey: 'task', id: 'task', header: t('taskLog.colTask'), meta: { label: 'taskLog.colTask' } },
  { accessorKey: 'script', id: 'script', header: t('taskLog.colScript'), meta: { label: 'taskLog.colScript' } },
  { accessorKey: 'phone', id: 'phone', header: t('taskLog.colPhone'), meta: { label: 'taskLog.colPhone' } },
  { accessorKey: 'trigger', id: 'trigger', header: t('taskLog.colTrigger'), meta: { label: 'taskLog.colTrigger' } },
  { accessorKey: 'start', id: 'start', header: t('taskLog.colStart'), meta: { label: 'taskLog.colStart' } },
  { accessorKey: 'duration', id: 'duration', header: t('taskLog.colDuration'), meta: { label: 'taskLog.colDuration' } },
  { accessorKey: 'status', id: 'status', header: t('taskLog.colStatus'), meta: { label: 'taskLog.colStatus' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
]

const statusMeta: Record<string, { icon: typeof CheckCircle2, variant: 'default' | 'destructive' | 'secondary', cls: string }> = {
  success: { icon: CheckCircle2, variant: 'default', cls: 'bg-green-100 text-green-700 dark:bg-green-950 dark:text-green-300' },
  failed: { icon: XCircle, variant: 'destructive', cls: '' },
  running: { icon: Loader2, variant: 'secondary', cls: '' },
}

function soon() {
  toast.info(t('taskLog.comingSoon'))
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
        v-model:search-value="search"
        :columns="columns"
        :data="logs"
        :get-row-id="(r) => String(r.id)"
        :search-placeholder="t('taskLog.searchPlaceholder')"
      >
        <template #filters>
          <NativeSelect v-model="statusFilter" class="h-9 w-32 text-xs">
            <NativeSelectOption value="">
              {{ t('taskLog.statusAll') }}
            </NativeSelectOption>
            <NativeSelectOption value="success">
              {{ t('taskLog.success') }}
            </NativeSelectOption>
            <NativeSelectOption value="failed">
              {{ t('taskLog.failed') }}
            </NativeSelectOption>
            <NativeSelectOption value="running">
              {{ t('taskLog.running') }}
            </NativeSelectOption>
          </NativeSelect>
        </template>
        <template #cell-task="{ row }">
          <span class="font-medium">{{ row.task }}</span>
        </template>
        <template #cell-script="{ row }">
          <span class="text-muted-foreground">{{ row.script }}</span>
        </template>
        <template #cell-phone="{ row }">
          <code class="font-mono text-xs">{{ row.phone }}</code>
        </template>
        <template #cell-trigger="{ row }">
          <span class="text-muted-foreground">{{ row.trigger }}</span>
        </template>
        <template #cell-start="{ row }">
          <span class="tabular-nums text-muted-foreground">{{ row.start }}</span>
        </template>
        <template #cell-duration="{ row }">
          <span class="tabular-nums">{{ row.duration }}</span>
        </template>
        <template #cell-status="{ row }">
          <Badge :variant="statusMeta[row.status].variant" :class="statusMeta[row.status].cls">
            <component :is="statusMeta[row.status].icon" class="mr-1 size-3" :class="row.status === 'running' && 'animate-spin'" />
            {{ t(`taskLog.${row.status}`) }}
          </Badge>
        </template>
        <template #cell-actions>
          <Button variant="ghost" size="sm" @click="soon">
            <Eye class="size-3.5" /> {{ t('taskLog.view') }}
          </Button>
        </template>
      </DataTable>
    </CardContent>
  </Card>
</template>
