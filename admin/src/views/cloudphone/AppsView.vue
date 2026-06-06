<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { StoreAppItem } from '@/types/app'
import { AppWindow, Trash, Upload } from 'lucide-vue-next'
import { computed, h, onMounted, onUnmounted, reactive, ref } from 'vue'
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
import { useQuerySync } from '@/composables/useQuerySync'
import { formatDateTime } from '@/utils/date'

const { t } = useI18n()
const data = ref<StoreAppItem[]>([])
const loading = ref(false)
const uploading = ref(false)
const uploadProgress = ref(0)
const fileInput = ref<HTMLInputElement | null>(null)
const filters = reactive({ q: '' })
useQuerySync(filters, { q: '' })
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
    const res = await appApi.storeList()
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

const columns = computed<ColumnDef<StoreAppItem>[]>(() => [
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
  { accessorKey: 'appName', id: 'appName', header: t('storeApps.colName'), meta: { label: 'storeApps.colName' } },
  { accessorKey: 'packageName', id: 'packageName', header: t('storeApps.colPackage'), meta: { label: 'storeApps.colPackage' } },
  { accessorKey: 'version', id: 'version', header: t('storeApps.colVersion'), meta: { label: 'storeApps.colVersion' } },
  { accessorKey: 'fileSize', id: 'fileSize', header: t('storeApps.colSize'), meta: { label: 'storeApps.colSize' } },
  { accessorKey: 'status', id: 'status', header: t('storeApps.colStatus'), meta: { label: 'storeApps.colStatus' } },
  { accessorKey: 'createTime', id: 'createTime', header: t('storeApps.colUploadTime'), meta: { label: 'storeApps.colUploadTime' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
])

function pickFile() {
  fileInput.value?.click()
}
async function onPicked(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file)
    return
  uploading.value = true
  uploadProgress.value = 0
  try {
    await appApi.storeUpload(file, undefined, undefined, p => (uploadProgress.value = p))
    toast.success(t('storeApps.uploadOk'), { description: file.name })
    await load()
  }
  catch {
    toast.error(t('storeApps.uploadFail'))
  }
  finally {
    uploading.value = false
  }
}

async function removeOne(row: StoreAppItem) {
  await appApi.storeBatchDelete([row.id])
  toast.success(t('storeApps.deleteOk'))
  load()
}
async function removeSelected() {
  const ids = [...selectedIds.value]
  if (!ids.length)
    return
  await appApi.storeBatchDelete(ids)
  toast.success(t('storeApps.deleteOk'))
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
          <CardTitle>{{ t('storeApps.title') }}</CardTitle>
          <CardDescription>{{ t('storeApps.desc') }}</CardDescription>
        </div>
        <div class="flex shrink-0 items-center gap-2">
          <Popconfirm v-auth="'app:manage'" tone="danger" :title="t('storeApps.batchDeleteConfirm', { n: selectedIds.size })" @confirm="removeSelected">
            <Button size="sm" variant="outline" :disabled="!selectedIds.size">
              <Trash class="size-4" /> {{ t('storeApps.batchDelete') }}<span v-if="selectedIds.size">（{{ selectedIds.size }}）</span>
            </Button>
          </Popconfirm>
          <Button v-auth="'app:manage'" size="sm" :disabled="uploading" @click="pickFile">
            <template v-if="uploading">
              <svg viewBox="0 0 36 36" class="size-4 -rotate-90">
                <circle cx="18" cy="18" r="16" fill="none" stroke="currentColor" stroke-opacity="0.3" stroke-width="4" />
                <circle
                  cx="18" cy="18" r="16" fill="none" stroke="currentColor" stroke-width="4"
                  stroke-linecap="round" pathLength="100" stroke-dasharray="100"
                  :stroke-dashoffset="100 - uploadProgress" class="transition-all duration-200"
                />
              </svg>
              {{ t('storeApps.uploading', { n: uploadProgress }) }}
            </template>
            <template v-else>
              <Upload class="size-4" /> {{ t('storeApps.upload') }}
            </template>
          </Button>
          <input ref="fileInput" type="file" accept=".apk,.xapk" class="hidden" @change="onPicked">
        </div>
      </div>
    </CardHeader>
    <CardContent>
      <DataTable
        v-model:search-value="filters.q"
        :columns="columns"
        :data="data"
        :loading="loading"
        :get-row-id="(r) => String(r.id)"
        :search-placeholder="t('storeApps.searchPlaceholder')"
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
            <div class="bg-muted flex size-8 shrink-0 items-center justify-center overflow-hidden rounded-md">
              <img v-if="row.iconPath" :src="row.iconPath" :alt="row.appName" class="size-full object-cover">
              <AppWindow v-else class="text-muted-foreground size-4" />
            </div>
            <span class="font-medium">{{ row.appName }}</span>
          </div>
        </template>
        <template #cell-packageName="{ row }">
          <span class="font-mono text-xs">{{ row.packageName || '-' }}</span>
        </template>
        <template #cell-version="{ row }">
          <span class="tabular-nums">{{ row.version || '-' }}</span>
        </template>
        <template #cell-fileSize="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ row.fileSize || '-' }}</span>
        </template>
        <template #cell-status="{ row }">
          <Badge v-if="row.status === 'CREATING'" variant="outline" class="animate-pulse border-amber-500 text-amber-600 dark:text-amber-400">
            {{ t('storeApps.statusCreating') }}
          </Badge>
          <Badge v-else variant="default">
            {{ t('storeApps.statusNormal') }}
          </Badge>
        </template>
        <template #cell-createTime="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.createTime) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <Popconfirm v-auth="'app:manage'" tone="danger" :title="t('storeApps.deleteConfirm', { name: row.appName })" @confirm="removeOne(row)">
            <Button size="sm" variant="ghost" class="text-destructive hover:text-destructive">
              <Trash class="size-4" /> {{ t('crud.delete') }}
            </Button>
          </Popconfirm>
        </template>
      </DataTable>
    </CardContent>
  </Card>
</template>
