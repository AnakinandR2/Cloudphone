<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { AccessLog } from '@/types/access_log'
import { computed, onMounted, reactive, ref } from 'vue'

import { useI18n } from 'vue-i18n'
import accessLogApi from '@/api/modules/access_log'
import DataTable from '@/components/DataTable.vue'
import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useQuerySync } from '@/composables/useQuerySync'
import { formatDateTimeFull } from '@/utils/date'
import AccessLogDetailPanel from './AccessLogDetailPanel.vue'
import { methodClass, statusVariant } from './helpers'

const { t } = useI18n()
const all = ref<AccessLog[]>([])

const search = reactive({
  username: '',
  method: 'all',
  path: '',
  status_group: 'all',
  start_time: '',
  end_time: '',
})
useQuerySync(search, {
  username: '',
  method: 'all',
  path: '',
  status_group: 'all',
  start_time: '',
  end_time: '',
})

const filtered = computed(() => {
  const s = search
  return all.value.filter((l) => {
    if (s.username && !l.username.includes(s.username)) return false
    if (s.method !== 'all' && l.method !== s.method) return false
    if (s.path && !l.path.includes(s.path)) return false
    if (s.status_group !== 'all') {
      const c = l.status_code
      if (s.status_group === '2xx' && !(c >= 200 && c < 300)) return false
      if (s.status_group === '4xx' && !(c >= 400 && c < 500)) return false
      if (s.status_group === '5xx' && c < 500) return false
    }
    if (s.start_time && l.created_at < s.start_time) return false
    if (s.end_time && l.created_at > `${s.end_time} 23:59:59`) return false
    return true
  })
})

const columns = computed<ColumnDef<AccessLog>[]>(() => [
  { id: 'expander', header: '', enableHiding: false, meta: { headClass: 'w-10' } },
  { accessorKey: 'id', id: 'id', header: t('table.id'), meta: { label: 'table.id' } },
  { accessorKey: 'created_at', id: 'created_at', header: t('accessLog.colTime'), meta: { label: 'accessLog.colTime' } },
  { accessorKey: 'username', id: 'username', header: t('accessLog.colUser'), meta: { label: 'accessLog.colUser' } },
  { accessorKey: 'method', id: 'method', header: t('accessLog.method'), meta: { label: 'accessLog.method' } },
  { accessorKey: 'path', id: 'path', header: t('accessLog.path'), meta: { label: 'accessLog.path' } },
  { accessorKey: 'status_code', id: 'status', header: t('accessLog.colStatus'), meta: { label: 'accessLog.colStatus' } },
  { accessorKey: 'latency_ms', id: 'latency', header: t('accessLog.colLatency'), meta: { label: 'accessLog.colLatency', headClass: 'text-right', cellClass: 'text-right' } },
  { accessorKey: 'client_ip', id: 'ip', header: t('accessLog.colIp'), meta: { label: 'accessLog.colIp' } },
])

const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const res = await accessLogApi.list({ page: 1, size: 999 })
    all.value = res.data.list
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
      <CardTitle>{{ t('accessLog.title') }}</CardTitle>
      <CardDescription>{{ t('accessLog.desc') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <DataTable :columns="columns" :data="filtered" :search="false" :loading="loading" expandable>
        <!-- 结构化筛选 -->
        <template #filters>
          <Input v-model="search.username" class="h-9 w-36" :placeholder="t('accessLog.searchUserPlaceholder')" />
          <Select v-model="search.method">
            <SelectTrigger class="h-9 w-24">
              <SelectValue :placeholder="t('accessLog.method')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">
                {{ t('accessLog.all') }}
              </SelectItem>
              <SelectItem v-for="m in ['GET', 'POST', 'PUT', 'DELETE', 'PATCH']" :key="m" :value="m">
                {{ m }}
              </SelectItem>
            </SelectContent>
          </Select>
          <Input v-model="search.path" class="h-9 w-44" :placeholder="t('accessLog.pathPlaceholder')" />
          <Select v-model="search.status_group">
            <SelectTrigger class="h-9 w-28">
              <SelectValue :placeholder="t('accessLog.statusGroup')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">
                {{ t('accessLog.all') }}
              </SelectItem>
              <SelectItem value="2xx">
                {{ t('accessLog.s2xx') }}
              </SelectItem>
              <SelectItem value="4xx">
                {{ t('accessLog.s4xx') }}
              </SelectItem>
              <SelectItem value="5xx">
                {{ t('accessLog.s5xx') }}
              </SelectItem>
            </SelectContent>
          </Select>
          <Input v-model="search.start_time" type="date" class="h-9 w-40" />
          <Input v-model="search.end_time" type="date" class="h-9 w-40" />
        </template>

        <template #cell-id="{ row }">
          <span class="text-muted-foreground">#{{ row.id }}</span>
        </template>
        <template #cell-created_at="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ formatDateTimeFull(row.created_at) }}</span>
        </template>
        <template #cell-username="{ row }">
          {{ row.username || '—' }}
        </template>
        <template #cell-method="{ row }">
          <span :class="methodClass(row.method)" class="font-semibold">{{ row.method }}</span>
        </template>
        <template #cell-path="{ row }">
          <code class="font-mono text-xs">{{ row.path }}</code>
        </template>
        <template #cell-status="{ row }">
          <Badge :variant="statusVariant(row.status_code)">
            {{ row.status_code }}
          </Badge>
        </template>
        <template #cell-latency="{ row }">
          <span class="tabular-nums">{{ row.latency_ms }}ms</span>
        </template>
        <template #cell-ip="{ row }">
          <span class="tabular-nums">{{ row.client_ip }}</span>
        </template>

        <!-- 行展开：调用详情接口获取请求/响应头与体 -->
        <template #expanded="{ row }">
          <AccessLogDetailPanel :id="row.id" />
        </template>
      </DataTable>
    </CardContent>
  </Card>
</template>
