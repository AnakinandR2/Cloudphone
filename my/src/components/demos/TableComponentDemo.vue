<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import { EllipsisVertical, Monitor } from 'lucide-vue-next'
import { computed, h, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import DataTable from '@/components/DataTable.vue'
import FilterBar from '@/components/FilterBar.vue'
import FilterField from '@/components/FilterField.vue'
import FilterSearchInput from '@/components/FilterSearchInput.vue'
import FilterSelect from '@/components/FilterSelect.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ACTIONS_COLUMN_META } from '@/lib/table'
import { phoneStatusBadge, STATUS_BADGE_DOT } from '@/utils/statusBadge'

interface DemoRow {
  id: number
  name: string
  cp_id: string
  status: string
  proxy: string
  tag: string
  remark: string
  created_at: string
}

const { t } = useI18n()

const rows: DemoRow[] = [
  { id: 4, name: '云手机4', cp_id: 'CP-1004', status: 'RUNNING', proxy: 'socks5://10.0.0.4:1084', tag: '12414141', remark: '1241414124', created_at: '2026-01-10 10:30' },
  { id: 3, name: '云手机3', cp_id: 'CP-1003', status: 'STOPPED', proxy: 'socks5://10.0.0.3:1083', tag: '12414141', remark: '—', created_at: '2026-01-09 16:20' },
  { id: 2, name: '云手机2', cp_id: 'CP-1002', status: 'CREATED', proxy: 'socks5://10.0.0.2:1082', tag: '12414141', remark: '—', created_at: '2026-01-08 09:15' },
  { id: 1, name: '云手机1', cp_id: 'CP-1001', status: 'CREATE_FAILED', proxy: '—', tag: '—', remark: '—', created_at: '2026-01-07 14:00' },
]

const filters = reactive({ q: '', status: 'all' })
const selectedIds = ref<Set<number>>(new Set())

const allRowsSelected = computed(() => rows.length > 0 && rows.every(r => selectedIds.value.has(r.id)))

function toggleSelect(id: number, checked: boolean) {
  const next = new Set(selectedIds.value)
  if (checked)
    next.add(id)
  else next.delete(id)
  selectedIds.value = next
}

function toggleSelectAll(checked: boolean) {
  selectedIds.value = checked ? new Set(rows.map(r => r.id)) : new Set()
}

const statusOptions = computed(() => [
  { value: 'all', label: t('comp.tableDemoStatusAll') },
  { value: 'RUNNING', label: t('phone.status_RUNNING') },
  { value: 'STOPPED', label: t('phone.status_STOPPED') },
  { value: 'CREATED', label: t('phone.status_CREATED') },
  { value: 'CREATE_FAILED', label: t('phone.status_CREATE_FAILED') },
])

const data = computed(() => {
  const q = filters.q.trim().toLowerCase()
  return rows.filter((r) => {
    const matchQ = !q || r.name.toLowerCase().includes(q) || r.cp_id.toLowerCase().includes(q)
    const matchStatus = filters.status === 'all' || r.status === filters.status
    return matchQ && matchStatus
  })
})

const columns = computed<ColumnDef<DemoRow>[]>(() => [
  {
    id: 'select',
    enableHiding: false,
    header: () => h(Checkbox, {
      modelValue: allRowsSelected.value,
      class: 'size-4',
      'onUpdate:modelValue': (checked: boolean | 'indeterminate') => toggleSelectAll(checked === true),
    }),
    meta: { headClass: 'w-12 pl-4', cellClass: 'w-12 pl-4' },
  },
  { id: 'expander', header: '', enableHiding: false, meta: { cellClass: 'w-8' } },
  { accessorKey: 'id', id: 'id', header: t('table.id'), meta: { label: 'table.id' } },
  { accessorKey: 'name', id: 'name', header: t('phone.colName'), meta: { label: 'phone.colName' } },
  { accessorKey: 'cp_id', id: 'cp_id', header: t('phone.colCpId'), meta: { label: 'phone.colCpId' } },
  { accessorKey: 'status', id: 'status', header: t('phone.colStatus'), meta: { label: 'phone.colStatus' } },
  { accessorKey: 'proxy', id: 'proxy', header: t('phone.colProxy'), meta: { label: 'phone.colProxy' } },
  { accessorKey: 'tag', id: 'tag', header: t('phone.tag.col'), meta: { label: 'phone.tag.col' } },
  { accessorKey: 'remark', id: 'remark', header: t('phone.colRemark'), meta: { label: 'phone.colRemark' } },
  { accessorKey: 'created_at', id: 'created_at', header: t('table.createdAt'), meta: { label: 'table.createdAt' } },
  { id: 'actions', header: t('crud.actions'), enableHiding: false, meta: ACTIONS_COLUMN_META },
])

function statusLabel(status: string) {
  return t(`phone.status_${status}`, status)
}

function canPowerOn(status: string) {
  return status === 'STOPPED' || status === 'CREATED'
}

function canPowerOff(status: string) {
  return status === 'RUNNING'
}
</script>

<template>
  <DataTable
    v-model:search-value="filters.q"
    pin-actions-column
    class="w-full"
    :columns="columns"
    :data="data"
    :search="false"
    expandable
    :get-row-id="(r) => String(r.id)"
  >
    <template #filters>
      <FilterBar class="min-w-0 flex-1">
        <FilterField :label="t('comp.filterSearchLabel')">
          <FilterSearchInput v-model="filters.q" :placeholder="t('comp.tableDemoSearchPlaceholder')" />
        </FilterField>
        <FilterField :label="t('comp.filterSelectLabel')">
          <FilterSelect
            v-model="filters.status"
            :options="statusOptions"
            :placeholder="t('comp.tableDemoStatusAll')"
          />
        </FilterField>
      </FilterBar>
    </template>

    <template #cell-select="{ row }">
      <Checkbox
        :model-value="selectedIds.has(row.id)"
        class="size-4"
        @update:model-value="checked => toggleSelect(row.id, checked === true)"
      />
    </template>

    <template #cell-id="{ row }">
      <span class="text-muted-foreground tabular-nums">#{{ row.id }}</span>
    </template>

    <template #cell-status="{ row }">
      <Badge
        :variant="phoneStatusBadge(row.status).variant"
        :class="phoneStatusBadge(row.status).class"
      >
        <span :class="STATUS_BADGE_DOT" />
        {{ statusLabel(row.status) }}
      </Badge>
    </template>

    <template #cell-tag="{ row }">
      <Badge v-if="row.tag !== '—'" variant="outline" class="border-transparent bg-emerald-500/15 font-normal text-emerald-600 dark:text-emerald-400">
        {{ row.tag }}
      </Badge>
      <span v-else class="text-muted-foreground">—</span>
    </template>

    <template #cell-actions="{ row }">
      <div class="flex items-center justify-end gap-2">
        <Button
          v-if="canPowerOff(row.status)"
          size="sm"
          variant="destructive"
        >
          {{ t('phone.op.powerOff') }}
        </Button>
        <Button
          v-else-if="canPowerOn(row.status)"
          size="sm"
          variant="ghost"
          class="bg-sidebar-primary/10 text-sidebar-primary shadow-none hover:bg-sidebar-primary/15 hover:text-sidebar-primary"
        >
          {{ t('phone.op.powerOn') }}
        </Button>
        <Button size="sm" variant="outline" :disabled="row.status !== 'RUNNING'">
          <Monitor class="size-4" /> {{ t('phone.op.remoteControl') }}
        </Button>
        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <Button variant="outline" size="icon-sm" :title="t('crud.more')">
              <EllipsisVertical class="size-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" class="w-36">
            <DropdownMenuItem>{{ t('crud.edit') }}</DropdownMenuItem>
            <DropdownMenuItem>{{ t('crud.delete') }}</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </template>

    <template #expanded="{ row }">
      <div class="text-muted-foreground px-4 py-3 text-sm">
        {{ t('comp.tableDemoExpanded', { name: row.name, id: row.cp_id }) }}
      </div>
    </template>
  </DataTable>
</template>
