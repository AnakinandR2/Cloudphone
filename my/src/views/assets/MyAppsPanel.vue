<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { UserApp } from '@/types/app'
import type { LibraryOverview } from '@/types/library'
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
  CardHeader,
} from '@/components/ui/card'
import { fmtBytes } from '@/utils/bytes'
import { formatDateTime } from '@/utils/date'
import UploadAppDialog from '@/views/app/UploadAppDialog.vue'

const props = defineProps<{ overview: LibraryOverview | null }>()
const emit = defineEmits<{ changed: [] }>()

const { t } = useI18n()
const data = ref<UserApp[]>([])
const loading = ref(false)
const uploadOpen = ref(false)
const search = ref('')
const selectedIds = ref<Set<number>>(new Set())

const locked = computed(() => props.overview?.usage.locked ?? false)

// 存在「解析中」时静默轮询，等服务端 finalize 收敛到 ready/failed。
let pollTimer: ReturnType<typeof setInterval> | null = null
function syncPolling() {
  const hasParsing = data.value.some(a => a.parse_status === 'parsing')
  if (hasParsing && !pollTimer) {
    pollTimer = setInterval(() => load(true), 4000)
  }
  else if (!hasParsing && pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

async function load(silent = false) {
  if (!silent)
    loading.value = true
  try {
    const res = await appApi.userList()
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
const allSelected = computed(() => data.value.length > 0 && data.value.every(a => selectedIds.value.has(a.file_id)))
function toggleAll(checked: boolean) {
  selectedIds.value = checked ? new Set(data.value.map(a => a.file_id)) : new Set()
}

const columns = computed<ColumnDef<UserApp>[]>(() => [
  {
    id: 'select',
    enableHiding: false,
    header: () => h('input', {
      'type': 'checkbox',
      'class': 'size-3.5 align-middle accent-primary',
      'checked': allSelected.value,
      'onChange': (e: Event) => toggleAll((e.target as HTMLInputElement).checked),
    }),
    meta: { cellClass: 'w-8' },
  },
  { accessorKey: 'app_name', id: 'app_name', header: t('app.colName'), meta: { label: 'app.colName' } },
  { accessorKey: 'package_name', id: 'package_name', header: t('app.colPackage'), meta: { label: 'app.colPackage' } },
  { accessorKey: 'version', id: 'version', header: t('app.colVersion'), meta: { label: 'app.colVersion' } },
  { accessorKey: 'size_bytes', id: 'size_bytes', header: t('app.colSize'), meta: { label: 'app.colSize' } },
  { accessorKey: 'parse_status', id: 'parse_status', header: t('app.colStatus'), meta: { label: 'app.colStatus' } },
  { accessorKey: 'created_at', id: 'created_at', header: t('app.colUploadTime'), meta: { label: 'app.colUploadTime' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
])

async function removeOne(row: UserApp) {
  await appApi.userBatchDelete([row.file_id])
  toast.success(t('app.deleteOk'))
  emit('changed')
  load()
}
async function removeSelected() {
  const ids = [...selectedIds.value]
  if (!ids.length)
    return
  await appApi.userBatchDelete(ids)
  toast.success(t('app.deleteOk'))
  emit('changed')
  load()
}

function openUpload() {
  if (locked.value) {
    toast.warning(t('library.lockedUploadHint'))
    return
  }
  uploadOpen.value = true
}

// 上传成功 → 通知容器刷新概览（用量变化）+ 列表。
function onUploaded() {
  emit('changed')
  load()
}

onMounted(load)
onUnmounted(() => {
  if (pollTimer)
    clearInterval(pollTimer)
})
</script>

<template>
  <Card>
    <CardHeader>
      <div class="flex items-start justify-between gap-4">
        <p class="text-muted-foreground text-sm">
          {{ t('app.desc') }}
        </p>
        <div class="flex shrink-0 items-center gap-2">
          <Popconfirm tone="danger" :title="t('app.batchDeleteConfirm', { n: selectedIds.size })" @confirm="removeSelected">
            <Button size="sm" variant="outline" :disabled="!selectedIds.size">
              <Trash class="size-4" /> {{ t('app.batchDelete') }}<span v-if="selectedIds.size">（{{ selectedIds.size }}）</span>
            </Button>
          </Popconfirm>
          <Button size="sm" :disabled="locked" @click="openUpload">
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
        :get-row-id="(r) => String(r.file_id)"
        :search-placeholder="t('app.searchPlaceholder')"
      >
        <template #cell-select="{ row }">
          <input
            type="checkbox"
            class="size-3.5 accent-primary"
            :checked="selectedIds.has(row.file_id)"
            @change="toggleSelect(row.file_id, ($event.target as HTMLInputElement).checked)"
          >
        </template>
        <template #cell-app_name="{ row }">
          <div class="flex items-center gap-2">
            <img v-if="row.icon_url" :src="row.icon_url" class="size-7 shrink-0 rounded" alt="">
            <span v-else class="flex size-7 shrink-0 items-center justify-center rounded bg-muted">
              <Package class="size-4 text-muted-foreground" />
            </span>
            <span class="font-medium">{{ row.app_name || '-' }}</span>
          </div>
        </template>
        <template #cell-package_name="{ row }">
          <span class="text-muted-foreground">{{ row.package_name || '-' }}</span>
        </template>
        <template #cell-version="{ row }">
          <span class="tabular-nums">{{ row.version || '-' }}</span>
        </template>
        <template #cell-size_bytes="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ fmtBytes(row.size_bytes) }}</span>
        </template>
        <template #cell-parse_status="{ row }">
          <Badge v-if="row.parse_status === 'parsing'" variant="outline" class="animate-pulse border-amber-500 text-amber-600 dark:text-amber-400">
            {{ t('app.statusParsing') }}
          </Badge>
          <Badge v-else-if="row.parse_status === 'failed'" variant="outline" class="border-red-500 text-red-600 dark:text-red-400" :title="row.parse_error || ''">
            {{ t('app.statusFailed') }}
          </Badge>
          <Badge v-else variant="default">
            {{ t('app.statusReady') }}
          </Badge>
        </template>
        <template #cell-created_at="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.created_at) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <Popconfirm tone="danger" :title="t('app.deleteConfirm', { name: row.app_name || row.package_name })" @confirm="removeOne(row)">
            <Button size="sm" variant="ghost" class="text-destructive hover:text-destructive">
              <Trash class="size-4" /> {{ t('crud.delete') }}
            </Button>
          </Popconfirm>
        </template>
      </DataTable>
    </CardContent>
  </Card>

  <UploadAppDialog v-model:open="uploadOpen" @success="onUploaded" />
</template>
