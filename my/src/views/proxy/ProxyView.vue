<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { Proxy } from '@/types/proxy'
import { Add2 as Plus } from 'reicon-vue'
import { Activity, Loader2, SquarePen, Trash2, Upload } from 'lucide-vue-next'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'

import proxyApi from '@/api/modules/proxy'
import DataTable from '@/components/DataTable.vue'
import Popconfirm from '@/components/Popconfirm.vue'
import TableExpanded from '@/components/TableExpanded.vue'
import TableExpandedTable from '@/components/TableExpandedTable.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useQuerySync } from '@/composables/useQuerySync'
import { ACTIONS_COLUMN_META } from '@/lib/table'
import { formatDateTime } from '@/utils/date'
import { proxyStatusBadge } from '@/utils/statusBadge'
import PartnerRecommendPanel from './PartnerRecommendPanel.vue'
import ProxyFormDialog from './ProxyFormDialog.vue'
import ProxyImportDialog from './ProxyImportDialog.vue'

const { t } = useI18n()
const data = ref<Proxy[]>([])
const loading = ref(false)
const testingId = ref(0) // 正在测试的代理 id（用于按钮 loading）
const filters = reactive({ q: '' })
useQuerySync(filters, { q: '' })

// 主表只保留关键列；用户名/地区/ASN/公司/最近检测等次要信息走「行展开」。
const columns = computed<ColumnDef<Proxy>[]>(() => [
  { id: 'expander', header: '', enableHiding: false, meta: { headClass: 'w-10' } },
  { accessorKey: 'id', id: 'id', header: t('table.id'), meta: { label: 'table.id' } },
  { accessorKey: 'name', id: 'name', header: t('proxy.colName'), meta: { label: 'proxy.colName' } },
  { accessorKey: 'host', id: 'address', header: t('proxy.colAddress'), meta: { label: 'proxy.colAddress' } },
  { accessorKey: 'status', id: 'status', header: t('proxy.colStatus'), meta: { label: 'proxy.colStatus' } },
  { accessorKey: 'latency', id: 'latency', header: t('proxy.colLatency'), meta: { label: 'proxy.colLatency' } },
  { accessorKey: 'egress_ip', id: 'egress_ip', header: t('proxy.colEgressIp'), meta: { label: 'proxy.colEgressIp' } },
  { accessorKey: 'remark', id: 'remark', header: t('proxy.fRemark'), meta: { label: 'proxy.fRemark' } },
  { accessorKey: 'created_at', id: 'created_at', header: t('table.createdAt'), meta: { label: 'table.createdAt' } },
  { id: 'actions', header: t('crud.actions'), enableHiding: false, meta: ACTIONS_COLUMN_META },
])

const dialog = ref({ open: false, id: 0, mode: 'create' as 'create' | 'edit' | 'view' })
const importOpen = ref(false)

function asnText(row: Proxy): string {
  if (!row.asn)
    return '-'
  return row.asn_name ? `${row.asn} ${row.asn_name}` : row.asn
}

async function load() {
  loading.value = true
  try {
    const res = await proxyApi.list({ page: 1, size: 999, kw: filters.q || undefined })
    data.value = res.data.list
  }
  finally {
    loading.value = false
  }
}
function openCreate() {
  dialog.value = { open: true, id: 0, mode: 'create' }
}
function openEdit(row: Proxy) {
  dialog.value = { open: true, id: row.id, mode: 'edit' }
}
async function deleteRow(row: Proxy) {
  await proxyApi.delete(row.id)
  toast.success(t('crud.deleteOk'))
  load()
}

// 测试代理：经 SOCKS5 实测连通性/延迟/出口 IP 并自动识别归属，完成后刷新列表
async function testRow(row: Proxy) {
  if (testingId.value)
    return
  testingId.value = row.id
  try {
    const res = await proxyApi.test(row.id)
    toast[res.data.status === 'ok' ? 'success' : 'error'](
      res.data.status === 'ok' ? t('proxy.testOk') : t('proxy.testFail'),
    )
    await load()
  }
  finally {
    testingId.value = 0
  }
}

onMounted(load)
</script>

<template>
  <Tabs default-value="mine" class="space-y-4">
    <TabsList>
      <TabsTrigger value="mine">
        {{ t('partner.tabMine') }}
      </TabsTrigger>
      <TabsTrigger value="recommend">
        {{ t('partner.tabRecommend') }}
      </TabsTrigger>
    </TabsList>

    <TabsContent value="mine">
      <Card>
        <CardHeader>
          <div class="flex items-center justify-between gap-4">
            <div class="space-y-1.5">
              <CardTitle>{{ t('proxy.title') }}</CardTitle>
              <CardDescription>{{ t('proxy.desc') }}</CardDescription>
            </div>
            <div class="flex shrink-0">
              <Button
                class="h-[52px] gap-2.5 rounded-xl px-[32px] has-[>svg]:px-[32px] text-lg font-semibold shadow-[var(--btn-shadow-hover)]"
                @click="openCreate"
              >
                <Plus class="size-6 shrink-0" stroke-width="2.5" />
                {{ t('proxy.add') }}
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <DataTable
            v-model:search-value="filters.q"
            pin-actions-column
            :columns="columns"
            :data="data"
            :loading="loading"
            :search-label="t('proxy.searchLabel')"
            :search-placeholder="t('proxy.searchPlaceholder')"
            expandable
          >
            <template #actions>
              <Button size="sm" variant="outline" @click="importOpen = true">
                <Upload class="size-4" /> {{ t('proxy.import') }}
              </Button>
            </template>

            <template #cell-id="{ row }">
              <span class="text-muted-foreground">#{{ row.id }}</span>
            </template>
            <template #cell-name="{ row }">
              <span class="font-medium">{{ row.name }}</span>
            </template>
            <template #cell-address="{ row }">
              <span class="text-muted-foreground font-mono">{{ row.protocol }}://{{ row.host }}:{{ row.port }}</span>
            </template>
            <template #cell-status="{ row }">
              <Badge v-bind="proxyStatusBadge(row.status)">
                {{ t(`proxy.status_${row.status}`, row.status) }}
              </Badge>
            </template>
            <template #cell-latency="{ row }">
              <span class="text-muted-foreground tabular-nums">{{ row.latency > 0 ? `${row.latency} ms` : '-' }}</span>
            </template>
            <template #cell-egress_ip="{ row }">
              <span class="text-muted-foreground font-mono">{{ row.egress_ip || '-' }}</span>
              <Badge v-if="row.egress_ip && row.country" variant="secondary" class="ml-1.5">
                {{ row.country }}
              </Badge>
            </template>
            <template #cell-remark="{ row }">
              <span class="text-muted-foreground">{{ row.remark || '-' }}</span>
            </template>
            <template #cell-created_at="{ row }">
              <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.created_at) }}</span>
            </template>

            <!-- 行展开：图2 浅灰底 + 嵌套横线表 -->
            <template #expanded="{ row }">
              <TableExpanded>
                <TableExpandedTable>
                  <template #head>
                    <th class="py-2.5 pr-6 font-normal whitespace-nowrap">
                      {{ t('proxy.colUsername') }}
                    </th>
                    <th class="py-2.5 pr-6 font-normal whitespace-nowrap">
                      {{ t('proxy.colRegion') }}
                    </th>
                    <th class="py-2.5 pr-6 font-normal whitespace-nowrap">
                      {{ t('proxy.colAsn') }}
                    </th>
                    <th class="py-2.5 pr-6 font-normal whitespace-nowrap">
                      {{ t('proxy.colCompany') }}
                    </th>
                    <th class="py-2.5 font-normal whitespace-nowrap">
                      {{ t('proxy.lastChecked') }}
                    </th>
                  </template>
                  <tr>
                    <td class="py-3 pr-6 whitespace-nowrap">
                      {{ row.username || '-' }}
                    </td>
                    <td class="py-3 pr-6 whitespace-nowrap">
                      {{ row.region || '-' }}
                    </td>
                    <td class="py-3 pr-6 whitespace-nowrap">
                      {{ asnText(row) }}
                    </td>
                    <td class="py-3 pr-6 whitespace-nowrap">
                      {{ row.company || '-' }}
                    </td>
                    <td class="py-3 tabular-nums whitespace-nowrap">
                      {{ row.last_checked_at ? formatDateTime(row.last_checked_at) : '-' }}
                    </td>
                  </tr>
                </TableExpandedTable>
              </TableExpanded>
            </template>

            <template #cell-actions="{ row }">
              <div class="flex items-center justify-end gap-2">
                <Button variant="outline" size="sm" :disabled="testingId === row.id" @click="testRow(row)">
                  <Loader2 v-if="testingId === row.id" class="size-4 animate-spin" />
                  <Activity v-else class="size-4" />
                  {{ t('proxy.test') }}
                </Button>
                <Button variant="outline" size="sm" @click="openEdit(row)">
                  <SquarePen class="size-4" /> {{ t('crud.edit') }}
                </Button>
                <Popconfirm :title="t('proxy.deleteConfirm', { name: row.name })" @confirm="deleteRow(row)">
                  <Button variant="outline" size="sm" class="text-destructive hover:text-destructive">
                    <Trash2 class="size-4" /> {{ t('crud.delete') }}
                  </Button>
                </Popconfirm>
              </div>
            </template>
          </DataTable>
        </CardContent>

        <ProxyFormDialog :id="dialog.id" v-model="dialog.open" :mode="dialog.mode" @success="load" @tested="load" />
        <ProxyImportDialog v-model="importOpen" @success="load" />
      </Card>
    </TabsContent>

    <TabsContent value="recommend">
      <PartnerRecommendPanel />
    </TabsContent>
  </Tabs>
</template>
