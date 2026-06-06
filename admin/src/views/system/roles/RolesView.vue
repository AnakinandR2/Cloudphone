<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { RoleListItem } from '@/types/role'
import { Plus } from 'lucide-vue-next'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { toast } from 'vue-sonner'
import roleApi from '@/api/modules/role'
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
import RoleFormDialog from './RoleFormDialog.vue'

const { t } = useI18n()
const data = ref<RoleListItem[]>([])
const filters = reactive({ q: '' })
useQuerySync(filters, { q: '' })

const columns = computed<ColumnDef<RoleListItem>[]>(() => [
  { accessorKey: 'id', id: 'id', header: t('table.id'), meta: { label: 'table.id' } },
  { accessorKey: 'name', id: 'name', header: t('table.name'), meta: { label: 'table.name' } },
  { accessorKey: 'description', id: 'description', header: t('table.description'), meta: { label: 'table.description' } },
  { accessorKey: 'permission_count', id: 'permission_count', header: t('roles.colPermCount'), meta: { label: 'roles.colPermCount' } },
  { accessorKey: 'user_count', id: 'user_count', header: t('roles.colUserCount'), meta: { label: 'roles.colUserCount' } },
  { accessorKey: 'is_builtin', id: 'builtin', header: t('table.builtin'), meta: { label: 'table.builtin' } },
  { accessorKey: 'created_at', id: 'created_at', header: t('table.createdAt'), meta: { label: 'table.createdAt' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
])

const dialog = ref({ open: false, id: 0, mode: 'create' as 'create' | 'edit' | 'view' })
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const res = await roleApi.list({ page: 1, size: 999 })
    data.value = res.data.list
  }
  finally {
    loading.value = false
  }
}
function openCreate() {
  dialog.value = { open: true, id: 0, mode: 'create' }
}
function openView(row: RoleListItem) {
  dialog.value = { open: true, id: row.id, mode: 'view' }
}
function openEdit(row: RoleListItem) {
  dialog.value = { open: true, id: row.id, mode: 'edit' }
}
async function deleteRow(row: RoleListItem) {
  await roleApi.delete(row.id)
  toast.success(t('crud.deleteOk'))
  load()
}

onMounted(load)
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ t('roles.title') }}</CardTitle>
      <CardDescription>{{ t('roles.desc') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <DataTable
        v-model:search-value="filters.q"
        :columns="columns"
        :data="data"
        :loading="loading"
        :search-placeholder="t('roles.searchPlaceholder')"
      >
        <template #actions>
          <Button v-auth="'role:create'" size="sm" @click="openCreate">
            <Plus class="size-4" /> {{ t('roles.add') }}
          </Button>
        </template>

        <template #cell-id="{ row }">
          <span class="text-muted-foreground">#{{ row.id }}</span>
        </template>
        <template #cell-name="{ row }">
          <span class="font-medium">{{ row.name }}</span>
        </template>
        <template #cell-description="{ row }">
          <span class="text-muted-foreground">{{ row.description || '-' }}</span>
        </template>
        <template #cell-permission_count="{ row }">
          <Badge variant="secondary">
            {{ row.permission_count }}
          </Badge>
        </template>
        <template #cell-user_count="{ row }">
          <Badge variant="outline">
            {{ row.user_count }}
          </Badge>
        </template>
        <template #cell-builtin="{ row }">
          <Badge v-if="row.is_builtin" variant="secondary">
            {{ t('users.yes') }}
          </Badge>
        </template>
        <template #cell-created_at="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.created_at) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <Button variant="ghost" size="sm" @click="openView(row)">
            {{ t('crud.view') }}
          </Button>
          <Button v-auth="'role:edit'" variant="ghost" size="sm" @click="openEdit(row)">
            {{ t('crud.edit') }}
          </Button>
          <Popconfirm :title="t('roles.deleteConfirm', { name: row.name })" @confirm="deleteRow(row)">
            <Button
              v-auth="'role:delete'"
              variant="ghost"
              size="sm"
              class="text-destructive hover:text-destructive"
              :disabled="row.is_builtin"
            >
              {{ t('crud.delete') }}
            </Button>
          </Popconfirm>
        </template>
      </DataTable>
    </CardContent>

    <RoleFormDialog :id="dialog.id" v-model="dialog.open" :mode="dialog.mode" @success="load" />
  </Card>
</template>
