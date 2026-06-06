<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { VirtualMachine } from '@/types/cloudphone'
import { AppWindow, Cpu, MoreHorizontal } from 'lucide-vue-next'
import { computed, onMounted, reactive, ref } from 'vue'

import { useI18n } from 'vue-i18n'
import cloudphoneApi from '@/api/modules/cloudphone'
import DataTable from '@/components/DataTable.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useQuerySync } from '@/composables/useQuerySync'
import { formatDateTime } from '@/utils/date'
import HostResourceDialog from './HostResourceDialog.vue'

const { t } = useI18n()
const all = ref<VirtualMachine[]>([])
const loading = ref(false)

const filters = reactive({ q: '', status: 'all' })
useQuerySync(filters, { q: '', status: 'all' })

const statusList = computed(() =>
  Array.from(new Set(all.value.map(v => v.vmStatus).filter(Boolean))),
)

const filtered = computed(() =>
  all.value.filter(v => filters.status === 'all' || v.vmStatus === filters.status),
)

const columns = computed<ColumnDef<VirtualMachine>[]>(() => [
  { accessorKey: 'vmUid', id: 'vmUid', header: t('cloudphone.vmUid'), meta: { label: 'cloudphone.vmUid' } },
  { accessorKey: 'specificationName', id: 'spec', header: t('cloudphone.specName'), meta: { label: 'cloudphone.specName' } },
  { accessorKey: 'vmIp', id: 'vmIp', header: t('cloudphone.vmIp'), meta: { label: 'cloudphone.vmIp' } },
  { id: 'resource', header: t('cloudphone.resource'), meta: { label: 'cloudphone.resource' } },
  { id: 'cp', header: t('cloudphone.cpCount'), meta: { label: 'cloudphone.cpCount' } },
  { accessorKey: 'vmStatus', id: 'status', header: t('table.status'), meta: { label: 'table.status' } },
  { accessorKey: 'createTime', id: 'createTime', header: t('cloudphone.createTime'), meta: { label: 'cloudphone.createTime' } },
  { accessorKey: 'expireTime', id: 'expireTime', header: t('cloudphone.expireTime'), meta: { label: 'cloudphone.expireTime' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
])

// 行操作：查看该主机「可用云手机规格」/「可用镜像」（按 specificationId 查中台套餐/镜像）。
const resDialog = ref({ open: false, host: null as VirtualMachine | null, kind: 'specs' as 'specs' | 'images' })
function openResource(host: VirtualMachine, kind: 'specs' | 'images') {
  resDialog.value = { open: true, host, kind }
}

function statusVariant(status: string) {
  if (status === 'ONLINE') return 'default'
  if (status === 'DESTROYED') return 'destructive'
  return 'secondary'
}

async function load() {
  loading.value = true
  try {
    const res = await cloudphoneApi.listVMs()
    all.value = res.data
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
      <CardTitle>{{ t('cloudphone.vmsTitle') }}</CardTitle>
      <CardDescription>{{ t('cloudphone.vmsDesc') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <DataTable
        v-model:search-value="filters.q"
        :columns="columns"
        :data="filtered"
        :loading="loading"
        :search-placeholder="t('cloudphone.searchVm')"
      >
        <template #filters>
          <Select v-model="filters.status">
            <SelectTrigger class="h-9 w-36">
              <SelectValue :placeholder="t('table.status')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">
                {{ t('cloudphone.allStatus') }}
              </SelectItem>
              <SelectItem v-for="s in statusList" :key="s" :value="s">
                {{ s }}
              </SelectItem>
            </SelectContent>
          </Select>
        </template>

        <template #cell-vmUid="{ row }">
          <span class="font-mono text-xs">{{ row.vmUid }}</span>
        </template>
        <template #cell-resource="{ row }">
          <span class="tabular-nums">{{ row.core }}C/{{ row.memory }}G/{{ row.storage }}G</span>
        </template>
        <template #cell-cp="{ row }">
          <span class="tabular-nums">{{ row.createPhoneNumber }} / {{ row.maxPhone }}</span>
        </template>
        <template #cell-status="{ row }">
          <Badge :variant="statusVariant(row.vmStatus)">
            {{ row.vmStatus }}
          </Badge>
        </template>
        <template #cell-createTime="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.createTime) }}</span>
        </template>
        <template #cell-expireTime="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ row.expireTime ? formatDateTime(row.expireTime) : '-' }}</span>
        </template>
        <template #cell-actions="{ row }">
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="ghost" size="icon" class="size-8">
                <MoreHorizontal class="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" class="w-44">
              <DropdownMenuItem @click="openResource(row, 'specs')">
                <Cpu class="size-4" /> {{ t('cloudphone.viewSpecs') }}
              </DropdownMenuItem>
              <DropdownMenuItem @click="openResource(row, 'images')">
                <AppWindow class="size-4" /> {{ t('cloudphone.viewImages') }}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </template>
      </DataTable>
    </CardContent>

    <HostResourceDialog v-model="resDialog.open" :host="resDialog.host" :kind="resDialog.kind" />
  </Card>
</template>
