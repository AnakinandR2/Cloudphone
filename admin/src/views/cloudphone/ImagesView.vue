<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { ImageInfo } from '@/types/cloudphone'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import cloudphoneApi from '@/api/modules/cloudphone'
import DataTable from '@/components/DataTable.vue'
import { Badge } from '@/components/ui/badge'
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
const data = ref<ImageInfo[]>([])
const loading = ref(false)

const filters = reactive({ q: '' })
useQuerySync(filters, { q: '' })

const columns = computed<ColumnDef<ImageInfo>[]>(() => [
  { accessorKey: 'imageId', id: 'imageId', header: t('cloudphone.imageId'), meta: { label: 'cloudphone.imageId' } },
  { accessorKey: 'imageName', id: 'imageName', header: t('cloudphone.imageName'), meta: { label: 'cloudphone.imageName' } },
  { accessorKey: 'imageVersion', id: 'imageVersion', header: t('cloudphone.imageVersion'), meta: { label: 'cloudphone.imageVersion' } },
  { accessorKey: 'isForbid', id: 'isForbid', header: t('table.status'), meta: { label: 'table.status' } },
  { accessorKey: 'downloadUrl', id: 'downloadUrl', header: t('cloudphone.downloadUrl'), meta: { label: 'cloudphone.downloadUrl' } },
  { accessorKey: 'createTime', id: 'createTime', header: t('cloudphone.createTime'), meta: { label: 'cloudphone.createTime' } },
  { accessorKey: 'updateTime', id: 'updateTime', header: t('cloudphone.updateTime'), meta: { label: 'cloudphone.updateTime' } },
])

async function load() {
  loading.value = true
  try {
    const res = await cloudphoneApi.listImages()
    data.value = res.data
  }
  finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ t('cloudphone.imagesTitle') }}</CardTitle>
      <CardDescription>{{ t('cloudphone.imagesDesc') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <DataTable
        v-model:search-value="filters.q"
        :columns="columns"
        :data="data"
        :loading="loading"
        :search-placeholder="t('cloudphone.searchImage')"
      >
        <template #cell-imageId="{ row }">
          <span class="font-mono text-xs">{{ row.imageId }}</span>
        </template>
        <template #cell-imageName="{ row }">
          <span class="font-medium">{{ row.imageName }}</span>
        </template>
        <template #cell-isForbid="{ row }">
          <Badge :variant="row.isForbid === 0 ? 'default' : 'secondary'">
            {{ row.isForbid === 0 ? t('table.enabled') : t('table.disabled') }}
          </Badge>
        </template>
        <template #cell-downloadUrl="{ row }">
          <a
            v-if="row.downloadUrl"
            :href="row.downloadUrl"
            target="_blank"
            rel="noopener"
            class="text-primary block max-w-[260px] truncate hover:underline"
            :title="row.downloadUrl"
          >
            {{ row.downloadUrl }}
          </a>
          <span v-else class="text-muted-foreground">-</span>
        </template>
        <template #cell-createTime="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.createTime) }}</span>
        </template>
        <template #cell-updateTime="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.updateTime) }}</span>
        </template>
      </DataTable>
    </CardContent>
  </Card>
</template>
