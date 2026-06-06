<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { AdminAppItem } from '@/types/app'
import { AppWindow, Trash } from 'lucide-vue-next'
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
const data = ref<AdminAppItem[]>([])
const loading = ref(false)
const filters = reactive({ q: '' })
useQuerySync(filters, { q: '' })
const selectedIds = ref<Set<number>>(new Set())

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

const columns = computed<ColumnDef<AdminAppItem>[]>(() => [
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
  { accessorKey: 'appName', id: 'appName', header: t('opsApps.colName'), meta: { label: 'opsApps.colName' } },
  { accessorKey: 'packageName', id: 'packageName', header: t('opsApps.colPackage'), meta: { label: 'opsApps.colPackage' } },
  { accessorKey: 'version', id: 'version', header: t('opsApps.colVersion'), meta: { label: 'opsApps.colVersion' } },
  { accessorKey: 'fileSize', id: 'fileSize', header: t('opsApps.colSize'), meta: { label: 'opsApps.colSize' } },
  { accessorKey: 'status', id: 'status', header: t('opsApps.colStatus'), meta: { label: 'opsApps.colStatus' } },
  { accessorKey: 'userPhone', id: 'user', header: t('opsApps.colUser'), meta: { label: 'opsApps.colUser' } },
  { accessorKey: 'createTime', id: 'createTime', header: t('opsApps.colUploadTime'), meta: { label: 'opsApps.colUploadTime' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
])

async function removeOne(row: AdminAppItem) {
  await appApi.batchDelete([row.id])
  toast.success(t('opsApps.deleteOk'))
  load()
}
async function removeSelected() {
  const ids = [...selectedIds.value]
  if (!ids.length)
    return
  await appApi.batchDelete(ids)
  toast.success(t('opsApps.deleteOk'))
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
          <CardTitle>{{ t('opsApps.title') }}</CardTitle>
          <CardDescription>{{ t('opsApps.desc') }}</CardDescription>
        </div>
        <Popconfirm v-auth="'app:manage'" tone="danger" :title="t('opsApps.batchDeleteConfirm', { n: selectedIds.size })" @confirm="removeSelected">
          <Button size="sm" variant="outline" :disabled="!selectedIds.size">
            <Trash class="size-4" /> {{ t('opsApps.batchDelete') }}<span v-if="selectedIds.size">（{{ selectedIds.size }}）</span>
          </Button>
        </Popconfirm>
      </div>
    </CardHeader>
    <CardContent>
      <DataTable
        v-model:search-value="filters.q"
        :columns="columns"
        :data="data"
        :loading="loading"
        :get-row-id="(r) => String(r.id)"
        :search-placeholder="t('opsApps.searchPlaceholder')"
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
            {{ t('opsApps.statusCreating') }}
          </Badge>
          <Badge v-else variant="default">
            {{ t('opsApps.statusNormal') }}
          </Badge>
        </template>
        <template #cell-user="{ row }">
          <div class="leading-tight">
            <div>{{ row.userNickname || '-' }}</div>
            <div class="text-muted-foreground text-xs tabular-nums">
              {{ row.userPhone || '-' }}
            </div>
          </div>
        </template>
        <template #cell-createTime="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.createTime) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <Popconfirm v-auth="'app:manage'" tone="danger" :title="t('opsApps.deleteConfirm', { name: row.appName })" @confirm="removeOne(row)">
            <Button size="sm" variant="ghost" class="text-destructive hover:text-destructive">
              <Trash class="size-4" /> {{ t('crud.delete') }}
            </Button>
          </Popconfirm>
        </template>
      </DataTable>
    </CardContent>
  </Card>
</template>
