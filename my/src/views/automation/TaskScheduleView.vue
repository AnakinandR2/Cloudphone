<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { Timer } from 'lucide-vue-next'
import { CalendarClock, Pause, Pencil, Play, Plus, Repeat, Trash2, Zap } from 'lucide-vue-next'
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

interface Task { id: number, name: string, script: string, targets: number, trigger: string, schedule: string, next: string, status: string }
const { t } = useI18n()

const search = ref('')
const tasks: Task[] = [
  { id: 1, name: '每日签到 - 早 8 点', script: '每日签到', targets: 12, trigger: 'cron', schedule: '每天 08:00', next: '2026-06-07 08:00', status: 'enabled' },
  { id: 2, name: '养号 - 循环', script: '养号脚本', targets: 30, trigger: 'interval', schedule: '每 4 小时', next: '2026-06-06 20:00', status: 'enabled' },
  { id: 3, name: '批量点赞 - 手动', script: '批量点赞', targets: 5, trigger: 'manual', schedule: '手动触发', next: '—', status: 'paused' },
  { id: 4, name: '关键词采集 - 工作日', script: '关键词采集', targets: 8, trigger: 'cron', schedule: '工作日 09:30', next: '2026-06-08 09:30', status: 'enabled' },
]
const columns: ColumnDef<Task>[] = [
  { accessorKey: 'name', id: 'name', header: t('taskSchedule.colName'), meta: { label: 'taskSchedule.colName' } },
  { accessorKey: 'script', id: 'script', header: t('taskSchedule.colScript'), meta: { label: 'taskSchedule.colScript' } },
  { accessorKey: 'targets', id: 'targets', header: t('taskSchedule.colTargets'), meta: { label: 'taskSchedule.colTargets' } },
  { accessorKey: 'trigger', id: 'trigger', header: t('taskSchedule.colTrigger'), meta: { label: 'taskSchedule.colTrigger' } },
  { accessorKey: 'schedule', id: 'schedule', header: t('taskSchedule.colSchedule'), meta: { label: 'taskSchedule.colSchedule' } },
  { accessorKey: 'next', id: 'next', header: t('taskSchedule.colNext'), meta: { label: 'taskSchedule.colNext' } },
  { accessorKey: 'status', id: 'status', header: t('taskSchedule.colStatus'), meta: { label: 'taskSchedule.colStatus' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
]

const triggerMeta: Record<string, { icon: typeof Timer, key: string }> = {
  cron: { icon: CalendarClock, key: 'triggerCron' },
  interval: { icon: Repeat, key: 'triggerInterval' },
  manual: { icon: Zap, key: 'triggerManual' },
}

function soon() {
  toast.info(t('taskSchedule.comingSoon'))
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
        <Button size="sm" class="shrink-0" @click="soon">
          <Plus class="size-4" /> {{ t('taskSchedule.newTask') }}
        </Button>
      </div>
    </CardHeader>
    <CardContent>
      <DataTable
        v-model:search-value="search"
        :columns="columns"
        :data="tasks"
        :get-row-id="(r) => String(r.id)"
        :search-placeholder="t('taskSchedule.searchPlaceholder')"
      >
        <template #cell-name="{ row }">
          <span class="font-medium">{{ row.name }}</span>
        </template>
        <template #cell-script="{ row }">
          <span class="text-muted-foreground">{{ row.script }}</span>
        </template>
        <template #cell-targets="{ row }">
          <span class="tabular-nums">{{ t('taskSchedule.targetsN', { n: row.targets }) }}</span>
        </template>
        <template #cell-trigger="{ row }">
          <Badge variant="secondary" class="font-normal">
            <component :is="triggerMeta[row.trigger].icon" class="mr-1 size-3" />
            {{ t(`taskSchedule.${triggerMeta[row.trigger].key}`) }}
          </Badge>
        </template>
        <template #cell-schedule="{ row }">
          <span class="text-muted-foreground">{{ row.schedule }}</span>
        </template>
        <template #cell-next="{ row }">
          <span class="tabular-nums text-muted-foreground">{{ row.next }}</span>
        </template>
        <template #cell-status="{ row }">
          <Badge :variant="row.status === 'enabled' ? 'default' : 'secondary'">
            {{ row.status === 'enabled' ? t('taskSchedule.enabled') : t('taskSchedule.paused') }}
          </Badge>
        </template>
        <template #cell-actions="{ row }">
          <Button variant="ghost" size="icon" class="size-7" :title="t('taskSchedule.runNow')" @click="soon">
            <Play class="size-3.5" />
          </Button>
          <Button variant="ghost" size="icon" class="size-7" :title="row.status === 'enabled' ? t('taskSchedule.pause') : t('taskSchedule.resume')" @click="soon">
            <Pause class="size-3.5" />
          </Button>
          <Button variant="ghost" size="icon" class="size-7" :title="t('crud.edit')" @click="soon">
            <Pencil class="size-3.5" />
          </Button>
          <Button variant="ghost" size="icon" class="size-7 text-destructive" :title="t('crud.delete')" @click="soon">
            <Trash2 class="size-3.5" />
          </Button>
        </template>
      </DataTable>
    </CardContent>
  </Card>
</template>
