<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { Staff } from '@/types/staff'
import { Plus, Star } from 'lucide-vue-next'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { toast } from 'vue-sonner'
import staffApi from '@/api/modules/staff'
import DataTable from '@/components/DataTable.vue'
import Popconfirm from '@/components/Popconfirm.vue'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
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
import StaffFormDialog from './StaffFormDialog.vue'

const { t } = useI18n()
const data = ref<Staff[]>([])
const filters = reactive({ q: '' })
useQuerySync(filters, { q: '' })

const columns = computed<ColumnDef<Staff>[]>(() => [
  { accessorKey: 'id', id: 'id', header: t('table.id'), meta: { label: 'table.id' } },
  { accessorKey: 'username', id: 'username', header: t('table.username'), meta: { label: 'table.username' } },
  { accessorKey: 'name', id: 'name', header: t('table.name'), meta: { label: 'table.name' } },
  { accessorKey: 'is_active', id: 'status', header: t('table.status'), meta: { label: 'table.status' } },
  { accessorKey: 'is_superuser', id: 'superuser', header: t('staff.colSuperuser'), meta: { label: 'staff.colSuperuser' } },
  { id: 'roles', header: t('staff.colRoles'), meta: { label: 'staff.colRoles' } },
  { accessorKey: 'created_at', id: 'created_at', header: t('table.createdAt'), meta: { label: 'table.createdAt' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
])

const dialog = ref({ open: false, id: 0, mode: 'create' as 'create' | 'edit' | 'view' })
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const res = await staffApi.list({ page: 1, size: 999 })
    data.value = res.data.list
  }
  finally {
    loading.value = false
  }
}
function openCreate() {
  dialog.value = { open: true, id: 0, mode: 'create' }
}
function openView(row: Staff) {
  dialog.value = { open: true, id: row.id, mode: 'view' }
}
function openEdit(row: Staff) {
  dialog.value = { open: true, id: row.id, mode: 'edit' }
}
async function deleteRow(row: Staff) {
  await staffApi.delete(row.id)
  toast.success(t('crud.deleteOk'))
  load()
}

onMounted(load)
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ t('staff.title') }}</CardTitle>
      <CardDescription>{{ t('staff.desc') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <DataTable
        v-model:search-value="filters.q"
        :columns="columns"
        :data="data"
        :loading="loading"
        :search-placeholder="t('staff.searchPlaceholder')"
      >
        <template #actions>
          <Button v-auth="'staff:create'" size="sm" @click="openCreate">
            <Plus class="size-4" /> {{ t('staff.add') }}
          </Button>
        </template>

        <template #cell-id="{ row }">
          <span class="text-muted-foreground">#{{ row.id }}</span>
        </template>
        <template #cell-username="{ row }">
          <div class="flex items-center gap-2">
            <Avatar class="size-7">
              <AvatarImage :src="row.avatar" :alt="row.username" />
              <AvatarFallback>{{ row.username.slice(0, 2).toUpperCase() }}</AvatarFallback>
            </Avatar>
            <span class="font-medium">{{ row.username }}</span>
          </div>
        </template>
        <template #cell-name="{ row }">
          {{ row.name?.trim() || row.username }}
        </template>
        <template #cell-status="{ row }">
          <Badge :variant="row.is_active ? 'default' : 'destructive'">
            {{ row.is_active ? t('table.enabled') : t('table.disabled') }}
          </Badge>
        </template>
        <template #cell-superuser="{ row }">
          <Badge v-if="row.is_superuser" variant="secondary" class="gap-1">
            <Star class="size-3" /> {{ t('staff.yes') }}
          </Badge>
          <span v-else class="text-muted-foreground text-sm">{{ t('staff.no') }}</span>
        </template>
        <template #cell-roles="{ row }">
          <Badge v-if="row.is_superuser" variant="destructive">
            {{ t('staff.superAll') }}
          </Badge>
          <div v-else-if="row.roles?.length" class="flex flex-wrap gap-1">
            <Badge v-for="r in row.roles" :key="r.id" variant="outline">
              {{ r.name }}
            </Badge>
          </div>
          <span v-else class="text-muted-foreground text-sm">{{ t('staff.noRoles') }}</span>
        </template>
        <template #cell-created_at="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.created_at) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <Button variant="ghost" size="sm" @click="openView(row)">
            {{ t('crud.view') }}
          </Button>
          <Button v-auth="'staff:edit'" variant="ghost" size="sm" @click="openEdit(row)">
            {{ t('crud.edit') }}
          </Button>
          <Popconfirm :title="t('staff.deleteConfirm', { name: row.username })" @confirm="deleteRow(row)">
            <Button v-auth="'staff:delete'" variant="ghost" size="sm" class="text-destructive hover:text-destructive">
              {{ t('crud.delete') }}
            </Button>
          </Popconfirm>
        </template>
      </DataTable>
    </CardContent>

    <StaffFormDialog :id="dialog.id" v-model="dialog.open" :mode="dialog.mode" @success="load" />
  </Card>
</template>
