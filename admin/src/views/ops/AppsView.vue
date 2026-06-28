<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { OpsUserApp } from '@/types/app'
import { AppWindow, Trash } from 'lucide-vue-next'
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
const data = ref<OpsUserApp[]>([])
const loading = ref(false)
const filters = reactive({ q: '' })
useQuerySync(filters, { q: '' })
const selectedIds = ref<Set<number>>(new Set())

async function load() {
  loading.value = true
  try {
    const res = await appApi.opsList()
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
const allSelected = computed(() => data.value.length > 0 && data.value.every(a => selectedIds.value.has(a.file_id)))
function toggleAll(checked: boolean) {
  selectedIds.value = checked ? new Set(data.value.map(a => a.file_id)) : new Set()
}

const columns = computed<ColumnDef<OpsUserApp>[]>(() => [
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
  { accessorKey: 'app_name', id: 'app_name', header: t('opsApps.colName'), meta: { label: 'opsApps.colName' } },
  { accessorKey: 'package_name', id: 'package_name', header: t('opsApps.colPackage'), meta: { label: 'opsApps.colPackage' } },
  { accessorKey: 'version', id: 'version', header: t('opsApps.colVersion'), meta: { label: 'opsApps.colVersion' } },
  { accessorKey: 'size_bytes', id: 'size_bytes', header: t('opsApps.colSize'), meta: { label: 'opsApps.colSize' } },
  { accessorKey: 'parse_status', id: 'parse_status', header: t('opsApps.colStatus'), meta: { label: 'opsApps.colStatus' } },
  { accessorKey: 'user_phone', id: 'user', header: t('opsApps.colUser'), meta: { label: 'opsApps.colUser' } },
  { accessorKey: 'created_at', id: 'created_at', header: t('opsApps.colUploadTime'), meta: { label: 'opsApps.colUploadTime' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
])

async function removeOne(row: OpsUserApp) {
  try {
    await appApi.opsBatchDelete([row.file_id])
    toast.success(t('opsApps.deleteOk'))
  }
  catch {
    toast.error(t('opsApps.deleteFail'))
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
    await appApi.opsBatchDelete(ids)
    toast.success(t('opsApps.deleteOk'))
  }
  catch {
    toast.error(t('opsApps.deleteFail'))
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
          <CardTitle>{{ t('opsApps.title') }}</CardTitle>
          <CardDescription>{{ t('opsApps.desc') }}</CardDescription>
        </div>
        <span v-auth="'app:manage'" class="contents">
          <Popconfirm tone="danger" :title="t('opsApps.batchDeleteConfirm', { n: selectedIds.size })" @confirm="removeSelected">
            <Button size="sm" variant="outline" :disabled="!selectedIds.size">
              <Trash class="size-4" /> {{ t('opsApps.batchDelete') }}<span v-if="selectedIds.size">（{{ selectedIds.size }}）</span>
            </Button>
          </Popconfirm>
        </span>
      </div>
    </CardHeader>
    <CardContent>
      <DataTable
        v-model:search-value="filters.q"
        :columns="columns"
        :data="data"
        :loading="loading"
        :get-row-id="(r) => String(r.file_id)"
        :search-placeholder="t('opsApps.searchPlaceholder')"
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
            {{ t('opsApps.statusParsing') }}
          </Badge>
          <Badge v-else-if="row.parse_status === 'failed'" variant="outline" class="border-destructive text-destructive" :title="row.parse_error || undefined">
            {{ t('opsApps.statusFailed') }}
          </Badge>
          <Badge v-else variant="default">
            {{ t('opsApps.statusReady') }}
          </Badge>
        </template>
        <template #cell-user="{ row }">
          <div class="leading-tight">
            <div>{{ row.user_nickname || '-' }}</div>
            <div class="text-muted-foreground text-xs tabular-nums">
              {{ row.user_phone || '-' }}
            </div>
          </div>
        </template>
        <template #cell-created_at="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.created_at) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <span v-auth="'app:manage'" class="contents">
            <Popconfirm tone="danger" :title="t('opsApps.deleteConfirm', { name: row.app_name })" @confirm="removeOne(row)">
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
