<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { AppItem } from '@/types/app'
import { Package, Trash, Upload } from 'lucide-vue-next'
import { computed, h, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'

import appApi from '@/api/modules/app'
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
import { formatDateTime } from '@/utils/date'
import UploadAppDialog from './UploadAppDialog.vue'

const { t } = useI18n()
const data = ref<AppItem[]>([])
const loading = ref(false)
const uploadOpen = ref(false)
const search = ref('')
const selectedIds = ref<Set<number>>(new Set())

// 存在「创建中」时静默轮询，等中台异步就绪后状态收敛到「正常」。
let pollTimer: ReturnType<typeof setInterval> | null = null
function syncPolling() {
  const hasCreating = data.value.some(a => a.status === 'CREATING')
  if (hasCreating && !pollTimer) {
    pollTimer = setInterval(load, 4000, true)
  }
  else if (!hasCreating && pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

async function load(silent = false) {
  if (!silent)
    loading.value = true
  try {
    const res = await appApi.list()
    data.value = res.data ?? []
    if (!silent)
      selectedIds.value = new Set()
    syncPolling()
  }
  finally {
    if (!silent)
      loading.value = false
  }
}

function toggleSelect(id: number, checked: boolean) {
  const next = new Set(selectedIds.value)
  if (checked)
    next.add(id)
  else next.delete(id)
  selectedIds.value = next
}
const allSelected = computed(() => data.value.length > 0 && data.value.every(a => selectedIds.value.has(a.id)))
function toggleAll(checked: boolean) {
  selectedIds.value = checked ? new Set(data.value.map(a => a.id)) : new Set()
}

const columns = computed<ColumnDef<AppItem>[]>(() => [
  {
    id: 'select',
    enableHiding: false,
    header: () => h('input', {
      type: 'checkbox',
      class: 'size-3.5 align-middle accent-primary',
      checked: allSelected.value,
      onChange: (e: Event) => toggleAll((e.target as HTMLInputElement).checked),
    }),
    meta: { cellClass: 'w-8' },
  },
  { accessorKey: 'appName', id: 'appName', header: t('app.colName'), meta: { label: 'app.colName' } },
  { accessorKey: 'packageName', id: 'packageName', header: t('app.colPackage'), meta: { label: 'app.colPackage' } },
  { accessorKey: 'version', id: 'version', header: t('app.colVersion'), meta: { label: 'app.colVersion' } },
  { accessorKey: 'fileSize', id: 'fileSize', header: t('app.colSize'), meta: { label: 'app.colSize' } },
  { accessorKey: 'status', id: 'status', header: t('app.colStatus'), meta: { label: 'app.colStatus' } },
  { accessorKey: 'createTime', id: 'createTime', header: t('app.colUploadTime'), meta: { label: 'app.colUploadTime' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
])

async function removeOne(row: AppItem) {
  await appApi.batchDelete([row.id])
  toast.success(t('app.deleteOk'))
  load()
}
async function removeSelected() {
  const ids = [...selectedIds.value]
  if (!ids.length)
    return
  await appApi.batchDelete(ids)
  toast.success(t('app.deleteOk'))
  load()
}

onMounted(() => load())
onUnmounted(() => {
  if (pollTimer)
    clearInterval(pollTimer)
})
</script>

<template>
  <Card>
    <CardHeader>
      <div class="flex items-start justify-between gap-4">
        <div class="space-y-1.5">
          <CardTitle>{{ t('app.title') }}</CardTitle>
          <CardDescription>{{ t('app.desc') }}</CardDescription>
        </div>
        <div class="flex shrink-0 items-center gap-2">
          <Popconfirm tone="danger" :title="t('app.batchDeleteConfirm', { n: selectedIds.size })" @confirm="removeSelected">
            <Button size="sm" variant="outline" :disabled="!selectedIds.size">
              <Trash class="size-4" /> {{ t('app.batchDelete') }}<span v-if="selectedIds.size">（{{ selectedIds.size }}）</span>
            </Button>
          </Popconfirm>
          <Button size="sm" @click="uploadOpen = true">
            <Upload class="size-4" /> {{ t('app.upload') }}
          </Button>
        </div>
      </div>
    </CardHeader>
    <CardContent>
      <DataTable
        v-model:search-value="search"
        :columns="columns"
        :data="data"
        :loading="loading"
        :get-row-id="(r) => String(r.id)"
        :search-placeholder="t('app.searchPlaceholder')"
      >
        <template #cell-select="{ row }">
          <input
            type="checkbox"
            class="size-3.5 accent-primary"
            :checked="selectedIds.has(row.id)"
            @change="toggleSelect(row.id, ($event.target as HTMLInputElement).checked)"
          >
        </template>
        <template #cell-appName="{ row }">
          <div class="flex items-center gap-2">
            <img v-if="row.iconPath" :src="row.iconPath" class="size-7 shrink-0 rounded" alt="">
            <span v-else class="flex size-7 shrink-0 items-center justify-center rounded bg-muted">
              <Package class="size-4 text-muted-foreground" />
            </span>
            <span class="font-medium">{{ row.appName }}</span>
          </div>
        </template>
        <template #cell-packageName="{ row }">
          <span class="text-muted-foreground">{{ row.packageName || '-' }}</span>
        </template>
        <template #cell-version="{ row }">
          <span class="tabular-nums">{{ row.version || '-' }}</span>
        </template>
        <template #cell-fileSize="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ row.fileSize || '-' }}</span>
        </template>
        <template #cell-status="{ row }">
          <Badge v-if="row.status === 'CREATING'" variant="outline" class="animate-pulse border-amber-500 text-amber-600 dark:text-amber-400">
            {{ t('app.statusCreating') }}
          </Badge>
          <Badge v-else variant="default">
            {{ t('app.statusNormal') }}
          </Badge>
        </template>
        <template #cell-createTime="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.createTime) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <Popconfirm tone="danger" :title="t('app.deleteConfirm', { name: row.appName })" @confirm="removeOne(row)">
            <Button size="sm" variant="ghost" class="text-destructive hover:text-destructive">
              <Trash class="size-4" /> {{ t('crud.delete') }}
            </Button>
          </Popconfirm>
        </template>
      </DataTable>
    </CardContent>
    <UploadAppDialog v-model:open="uploadOpen" @success="load" />
  </Card>
</template>
