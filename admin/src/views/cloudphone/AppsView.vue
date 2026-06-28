<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { MarketApp } from '@/types/app'
import { AppWindow, Trash, Upload } from 'lucide-vue-next'
import { computed, h, onMounted, reactive, ref } from 'vue'
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
import { fmtBytes } from '@/utils/bytes'
import { formatDateTime } from '@/utils/date'

const { t } = useI18n()
const data = ref<MarketApp[]>([])
const loading = ref(false)
const uploading = ref(false)
const uploadProgress = ref(0)
const fileInput = ref<HTMLInputElement | null>(null)
const filters = reactive({ q: '' })
useQuerySync(filters, { q: '' })
const selectedIds = ref<Set<number>>(new Set())

async function load() {
  loading.value = true
  try {
    const res = await appApi.marketList()
    data.value = res.data ?? []
    selectedIds.value = new Set()
  }
  finally {
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

const columns = computed<ColumnDef<MarketApp>[]>(() => [
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
  { accessorKey: 'app_name', id: 'app_name', header: t('storeApps.colName'), meta: { label: 'storeApps.colName' } },
  { accessorKey: 'package_name', id: 'package_name', header: t('storeApps.colPackage'), meta: { label: 'storeApps.colPackage' } },
  { accessorKey: 'version', id: 'version', header: t('storeApps.colVersion'), meta: { label: 'storeApps.colVersion' } },
  { accessorKey: 'size_bytes', id: 'size_bytes', header: t('storeApps.colSize'), meta: { label: 'storeApps.colSize' } },
  { accessorKey: 'parse_status', id: 'parse_status', header: t('storeApps.colStatus'), meta: { label: 'storeApps.colStatus' } },
  { accessorKey: 'created_at', id: 'created_at', header: t('storeApps.colUploadTime'), meta: { label: 'storeApps.colUploadTime' } },
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
    await appApi.marketUpload(file, p => (uploadProgress.value = p))
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

async function removeOne(row: MarketApp) {
  try {
    await appApi.marketBatchDelete([row.id])
    toast.success(t('storeApps.deleteOk'))
  }
  catch {
    toast.error(t('storeApps.deleteFail'))
  }
  finally {
    load()
  }
}
async function removeSelected() {
  const ids = [...selectedIds.value]
  if (!ids.length)
    return
  try {
    await appApi.marketBatchDelete(ids)
    toast.success(t('storeApps.deleteOk'))
  }
  catch {
    toast.error(t('storeApps.deleteFail'))
  }
  finally {
    load()
  }
}

onMounted(() => load())
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
          <span v-auth="'app:manage'" class="contents">
            <Popconfirm tone="danger" :title="t('storeApps.batchDeleteConfirm', { n: selectedIds.size })" @confirm="removeSelected">
              <Button size="sm" variant="outline" :disabled="!selectedIds.size">
                <Trash class="size-4" /> {{ t('storeApps.batchDelete') }}<span v-if="selectedIds.size">（{{ selectedIds.size }}）</span>
              </Button>
            </Popconfirm>
          </span>
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
        <template #cell-app_name="{ row }">
          <div class="flex items-center gap-2">
            <div class="bg-muted flex size-8 shrink-0 items-center justify-center overflow-hidden rounded-md">
              <img v-if="row.icon_url" :src="row.icon_url" :alt="row.app_name" class="size-full object-cover">
              <AppWindow v-else class="text-muted-foreground size-4" />
            </div>
            <span class="font-medium">{{ row.app_name || '-' }}</span>
          </div>
        </template>
        <template #cell-package_name="{ row }">
          <span class="font-mono text-xs">{{ row.package_name || '-' }}</span>
        </template>
        <template #cell-version="{ row }">
          <span class="tabular-nums">{{ row.version || '-' }}</span>
        </template>
        <template #cell-size_bytes="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ fmtBytes(row.size_bytes) }}</span>
        </template>
        <template #cell-parse_status="{ row }">
          <Badge v-if="row.parse_status === 'parsing'" variant="outline" class="animate-pulse border-amber-500 text-amber-600 dark:text-amber-400">
            {{ t('storeApps.statusParsing') }}
          </Badge>
          <Badge v-else-if="row.parse_status === 'failed'" variant="outline" class="border-destructive text-destructive" :title="row.parse_error || undefined">
            {{ t('storeApps.statusFailed') }}
          </Badge>
          <Badge v-else variant="default">
            {{ t('storeApps.statusReady') }}
          </Badge>
        </template>
        <template #cell-created_at="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.created_at) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <span v-auth="'app:manage'" class="contents">
            <Popconfirm tone="danger" :title="t('storeApps.deleteConfirm', { name: row.app_name })" @confirm="removeOne(row)">
              <Button size="sm" variant="ghost" class="text-destructive hover:text-destructive">
                <Trash class="size-4" /> {{ t('crud.delete') }}
              </Button>
            </Popconfirm>
          </span>
        </template>
      </DataTable>
    </CardContent>
  </Card>
</template>
