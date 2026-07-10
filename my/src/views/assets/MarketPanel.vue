<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { MarketApp } from '@/types/app'
import { Info, Package } from 'lucide-vue-next'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import appApi from '@/api/modules/app'
import DataTable from '@/components/DataTable.vue'
import { Badge } from '@/components/ui/badge'
import { fmtBytes } from '@/utils/bytes'
import { formatDateTime } from '@/utils/date'
import { parseStatusBadge } from '@/utils/statusBadge'

const { t } = useI18n()
const data = ref<MarketApp[]>([])
const loading = ref(false)
const search = ref('')

async function load() {
  loading.value = true
  try {
    const res = await appApi.market()
    data.value = res.data ?? []
  }
  finally {
    loading.value = false
  }
}

const columns = computed<ColumnDef<MarketApp>[]>(() => [
  { accessorKey: 'app_name', id: 'app_name', header: t('app.colName'), meta: { label: 'app.colName' } },
  { accessorKey: 'package_name', id: 'package_name', header: t('app.colPackage'), meta: { label: 'app.colPackage' } },
  { accessorKey: 'version', id: 'version', header: t('app.colVersion'), meta: { label: 'app.colVersion' } },
  { accessorKey: 'size_bytes', id: 'size_bytes', header: t('app.colSize'), meta: { label: 'app.colSize' } },
  { accessorKey: 'parse_status', id: 'parse_status', header: t('app.colStatus'), meta: { label: 'app.colStatus' } },
  { accessorKey: 'created_at', id: 'created_at', header: t('app.colUploadTime'), meta: { label: 'app.colUploadTime' } },
])

onMounted(load)
</script>

<template>
  <div class="flex flex-col gap-3">
    <!-- 应用市场不占用容量提示 -->
    <div class="flex items-center gap-2 rounded-lg border border-primary/30 bg-primary/5 px-3 py-2 text-sm text-primary">
      <Info class="size-4 shrink-0" />
      <span>{{ t('assets.marketNoQuota') }}</span>
    </div>

    <DataTable
      v-model:search-value="search"
      class="w-full"
      :columns="columns"
      :data="data"
      :loading="loading"
      :get-row-id="(r) => String(r.id)"
      :search-label="t('app.searchLabel')"
      :search-placeholder="t('app.searchPlaceholder')"
    >
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
          <Badge v-bind="parseStatusBadge(row.parse_status)">
            {{ row.parse_status === 'parsing' ? t('app.statusParsing') : row.parse_status === 'failed' ? t('app.statusFailed') : t('app.statusReady') }}
          </Badge>
        </template>
        <template #cell-created_at="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.created_at) }}</span>
        </template>
      </DataTable>
  </div>
</template>
