<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { Proxy } from '@/types/proxy'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { toast } from 'vue-sonner'
import proxyApi from '@/api/modules/proxy'
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
const data = ref<Proxy[]>([])
const loading = ref(false)
const filters = reactive({ q: '' })
useQuerySync(filters, { q: '' })

const columns = computed<ColumnDef<Proxy>[]>(() => [
  { accessorKey: 'id', id: 'id', header: t('table.id'), meta: { label: 'table.id' } },
  { accessorKey: 'user_id', id: 'user_id', header: t('proxy.colUserId'), meta: { label: 'proxy.colUserId' } },
  { accessorKey: 'name', id: 'name', header: t('proxy.colName'), meta: { label: 'proxy.colName' } },
  { id: 'address', header: t('proxy.colAddress'), meta: { label: 'proxy.colAddress' } },
  { accessorKey: 'username', id: 'username', header: t('proxy.colUsername'), meta: { label: 'proxy.colUsername' } },
  { accessorKey: 'region', id: 'region', header: t('proxy.colRegion'), meta: { label: 'proxy.colRegion' } },
  { accessorKey: 'status', id: 'status', header: t('table.status'), meta: { label: 'table.status' } },
  { accessorKey: 'latency', id: 'latency', header: t('proxy.colLatency'), meta: { label: 'proxy.colLatency' } },
  { accessorKey: 'egress_ip', id: 'egress_ip', header: t('proxy.colEgressIp'), meta: { label: 'proxy.colEgressIp' } },
  { accessorKey: 'created_at', id: 'created_at', header: t('proxy.colCreatedAt'), meta: { label: 'proxy.colCreatedAt' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
])

function statusVariant(status: Proxy['status']) {
  if (status === 'ok') return 'default'
  if (status === 'fail') return 'destructive'
  return 'secondary'
}

async function load() {
  loading.value = true
  try {
    const res = await proxyApi.adminList({ page: 1, size: 999, kw: filters.q })
    data.value = res.data.list
  }
  finally {
    loading.value = false
  }
}

async function remove(row: Proxy) {
  await proxyApi.adminDelete(row.id)
  toast.success(t('crud.deleteOk'))
  load()
}

onMounted(load)
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ t('proxy.title') }}</CardTitle>
      <CardDescription>{{ t('proxy.desc') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <DataTable
        v-model:search-value="filters.q"
        :columns="columns"
        :data="data"
        :loading="loading"
        :search-placeholder="t('proxy.searchPlaceholder')"
      >
        <template #cell-id="{ row }">
          <span class="text-muted-foreground">#{{ row.id }}</span>
        </template>
        <template #cell-user_id="{ row }">
          <span class="tabular-nums">{{ row.user_id }}</span>
        </template>
        <template #cell-name="{ row }">
          {{ row.name?.trim() || '-' }}
        </template>
        <template #cell-address="{ row }">
          <span class="font-mono text-xs">{{ row.protocol }}://{{ row.host }}:{{ row.port }}</span>
        </template>
        <template #cell-username="{ row }">
          {{ row.username?.trim() || '-' }}
        </template>
        <template #cell-region="{ row }">
          {{ row.region?.trim() || '-' }}
        </template>
        <template #cell-status="{ row }">
          <Badge :variant="statusVariant(row.status)">
            {{ t(`proxy.status_${row.status}`) }}
          </Badge>
        </template>
        <template #cell-latency="{ row }">
          <span class="tabular-nums">{{ row.latency }} ms</span>
        </template>
        <template #cell-egress_ip="{ row }">
          <span class="font-mono text-xs">{{ row.egress_ip?.trim() || '-' }}</span>
        </template>
        <template #cell-created_at="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.created_at) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <Popconfirm
            :title="t('proxy.deleteConfirm', { name: row.name || row.id })"
            @confirm="remove(row)"
          >
            <Button
              v-auth="'proxy:manage'"
              variant="ghost"
              size="sm"
              class="text-destructive hover:text-destructive"
            >
              {{ t('common.delete') }}
            </Button>
          </Popconfirm>
        </template>
      </DataTable>
    </CardContent>
  </Card>
</template>
