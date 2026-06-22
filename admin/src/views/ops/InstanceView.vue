<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { CloudPhone } from '@/types/instance'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { toast } from 'vue-sonner'
import instanceApi from '@/api/modules/instance'
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
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { useQuerySync } from '@/composables/useQuerySync'
import { formatDateTime } from '@/utils/date'
import { tagClass } from '@/utils/tagColor'

const { t } = useI18n()
const data = ref<CloudPhone[]>([])
const loading = ref(false)
const filters = reactive({ q: '' })
useQuerySync(filters, { q: '' })

// 状态/标签过滤下推后端；选项用固定枚举 + 全用户标签接口，避免被结果集裁剪。
const statusFilter = ref('')
const statusOptions = ['RUNNING', 'STOPPED', 'STARTING', 'STOPPING', 'CREATING', 'CREATE_FAILED', 'DESTROYING', 'CREATED']
const tagFilter = ref('')
const tagOptions = ref<string[]>([])
async function loadTags() {
  try {
    tagOptions.value = (await instanceApi.tags()).data?.map(tg => tg.name) ?? []
  }
  catch {
    tagOptions.value = []
  }
}

const columns = computed<ColumnDef<CloudPhone>[]>(() => [
  { accessorKey: 'id', id: 'id', header: t('table.id'), meta: { label: 'table.id' } },
  { accessorKey: 'user_id', id: 'user_id', header: t('instance.colUserId'), meta: { label: 'instance.colUserId' } },
  { accessorKey: 'name', id: 'name', header: t('instance.colName'), meta: { label: 'instance.colName' } },
  { accessorKey: 'cp_id', id: 'cp_id', header: t('instance.colCpId'), meta: { label: 'instance.colCpId' } },
  { accessorKey: 'status', id: 'status', header: t('table.status'), meta: { label: 'table.status' } },
  { accessorKey: 'vm_id', id: 'vm_id', header: t('instance.colVmId'), meta: { label: 'instance.colVmId' } },
  { accessorKey: 'image_id', id: 'image_id', header: t('instance.colImageId'), meta: { label: 'instance.colImageId' } },
  { accessorKey: 'proxy_id', id: 'proxy_id', header: t('instance.colProxyId'), meta: { label: 'instance.colProxyId' } },
  { id: 'tags', header: t('instance.colTags'), enableHiding: true, meta: { label: 'instance.colTags' } },
  { accessorKey: 'created_at', id: 'created_at', header: t('instance.colCreatedAt'), meta: { label: 'instance.colCreatedAt' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
])

function statusVariant(status: CloudPhone['status']) {
  if (status === 'RUNNING') return 'default'
  if (status === 'CREATE_FAILED') return 'destructive'
  return 'secondary'
}

// 过渡态（创建中/开机中）描边脉冲；销毁中红脉冲；已创建蓝；停止灰。
function statusClass(status: CloudPhone['status']) {
  if (status === 'CREATING' || status === 'STARTING' || status === 'STOPPING')
    return 'border-amber-500 text-amber-600 dark:text-amber-400 animate-pulse'
  if (status === 'DESTROYING')
    return 'border-destructive text-destructive animate-pulse'
  if (status === 'CREATED')
    return 'border-blue-500 text-blue-600 dark:text-blue-400'
  return ''
}

const proxyMap = ref<Map<number, string>>(new Map())
async function loadProxies() {
  try {
    const res = await proxyApi.adminList({ page: 1, size: 999 })
    const m = new Map<number, string>()
    for (const p of res.data.list)
      m.set(p.id, `${p.protocol}://${p.host}:${p.port}${p.region ? ` · ${p.region}` : ''}`)
    proxyMap.value = m
  }
  catch {
    proxyMap.value = new Map()
  }
}

async function load() {
  loading.value = true
  try {
    const res = await instanceApi.adminList({
      page: 1,
      size: 999,
      status: statusFilter.value || undefined,
      tag: tagFilter.value || undefined,
    })
    data.value = res.data.list
  }
  finally {
    loading.value = false
  }
}

async function remove(row: CloudPhone) {
  await instanceApi.adminDelete(row.id)
  toast.success(t('crud.deleteOk'))
  load()
}

watch([statusFilter, tagFilter], () => load())
onMounted(() => {
  load()
  loadProxies()
  loadTags()
})
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ t('instance.title') }}</CardTitle>
      <CardDescription>{{ t('instance.desc') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <DataTable
        v-model:search-value="filters.q"
        :columns="columns"
        :data="data"
        :loading="loading"
        :search-placeholder="t('instance.searchPlaceholder')"
      >
        <template #filters>
          <NativeSelect v-model="statusFilter" class="h-8 w-32 text-xs">
            <NativeSelectOption value="">
              {{ t('instance.statusAll') }}
            </NativeSelectOption>
            <NativeSelectOption v-for="st in statusOptions" :key="st" :value="st">
              {{ t(`instance.status_${st}`, st) }}
            </NativeSelectOption>
          </NativeSelect>
          <NativeSelect v-model="tagFilter" class="h-8 w-32 text-xs">
            <NativeSelectOption value="">
              {{ t('instance.tagAll') }}
            </NativeSelectOption>
            <NativeSelectOption v-for="tg in tagOptions" :key="tg" :value="tg">
              {{ tg }}
            </NativeSelectOption>
          </NativeSelect>
        </template>
        <template #cell-id="{ row }">
          <span class="text-muted-foreground">#{{ row.id }}</span>
        </template>
        <template #cell-user_id="{ row }">
          <span class="tabular-nums">{{ row.user_id }}</span>
        </template>
        <template #cell-name="{ row }">
          {{ row.name?.trim() || '-' }}
        </template>
        <template #cell-cp_id="{ row }">
          <span class="font-mono text-xs">{{ row.cp_id?.trim() || '-' }}</span>
        </template>
        <template #cell-status="{ row }">
          <Badge :variant="statusVariant(row.status)" :class="statusClass(row.status)">
            {{ t(`instance.status_${row.status}`, row.status) }}
          </Badge>
        </template>
        <template #cell-vm_id="{ row }">
          <span class="font-mono text-xs">{{ row.vm_id?.trim() || '-' }}</span>
        </template>
        <template #cell-image_id="{ row }">
          <span class="font-mono text-xs">{{ row.image_id?.trim() || '-' }}</span>
        </template>
        <template #cell-proxy_id="{ row }">
          <span v-if="row.proxy_id" class="font-mono text-xs">{{ proxyMap.get(row.proxy_id) ?? `#${row.proxy_id}` }}</span>
          <span v-else class="text-muted-foreground">{{ t('instance.proxyUnbound') }}</span>
        </template>
        <template #cell-tags="{ row }">
          <div class="flex flex-wrap gap-1">
            <Badge v-for="tg in row.tags" :key="tg.name" variant="outline" class="text-[10px]" :class="tagClass(tg.color)">
              {{ tg.name }}
            </Badge>
            <span v-if="!row.tags?.length" class="text-muted-foreground">-</span>
          </div>
        </template>
        <template #cell-created_at="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.created_at) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <Popconfirm
            :title="t('instance.deleteConfirm', { name: row.name || row.id })"
            @confirm="remove(row)"
          >
            <Button
              v-auth="'phone:manage'"
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
