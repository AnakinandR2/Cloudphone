<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { ExampleItem } from '@/types/example'
import { Plus } from 'lucide-vue-next'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { toast } from 'vue-sonner'
import exampleApi from '@/api/modules/example'
import DataTable from '@/components/DataTable.vue'
import Popconfirm from '@/components/Popconfirm.vue'
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
import ExampleFormDialog from './ExampleFormDialog.vue'

const { t } = useI18n()
const data = ref<ExampleItem[]>([])
const filters = reactive({ q: '' })
useQuerySync(filters, { q: '' })

const columns = computed<ColumnDef<ExampleItem>[]>(() => [
  { accessorKey: 'id', id: 'id', header: t('table.id'), meta: { label: 'table.id' } },
  { accessorKey: 'title', id: 'title', header: t('example.colTitle'), meta: { label: 'example.colTitle' } },
  { accessorKey: 'content', id: 'content', header: t('example.colContent'), meta: { label: 'example.colContent' } },
  { accessorKey: 'created_at', id: 'created_at', header: t('table.createdAt'), meta: { label: 'table.createdAt' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
])

const dialog = ref({ open: false, id: 0, mode: 'create' as 'create' | 'edit' | 'view' })
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const res = await exampleApi.list({ page: 1, size: 999 })
    data.value = res.data.list
  }
  finally {
    loading.value = false
  }
}
function openCreate() {
  dialog.value = { open: true, id: 0, mode: 'create' }
}
function openView(row: ExampleItem) {
  dialog.value = { open: true, id: row.id, mode: 'view' }
}
function openEdit(row: ExampleItem) {
  dialog.value = { open: true, id: row.id, mode: 'edit' }
}
async function deleteRow(row: ExampleItem) {
  await exampleApi.delete(row.id)
  toast.success(t('crud.deleteOk'))
  load()
}

onMounted(load)
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ t('example.title') }}</CardTitle>
      <CardDescription>{{ t('example.desc') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <DataTable
        v-model:search-value="filters.q"
        :columns="columns"
        :data="data"
        :loading="loading"
        :search-placeholder="t('example.searchPlaceholder')"
      >
        <template #actions>
          <Button v-auth="'example:create'" size="sm" @click="openCreate">
            <Plus class="size-4" /> {{ t('example.add') }}
          </Button>
        </template>

        <template #cell-id="{ row }">
          <span class="text-muted-foreground">#{{ row.id }}</span>
        </template>
        <template #cell-title="{ row }">
          <span class="font-medium">{{ row.title }}</span>
        </template>
        <template #cell-content="{ row }">
          <span class="text-muted-foreground block max-w-md truncate">{{ row.content }}</span>
        </template>
        <template #cell-created_at="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.created_at) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <Button variant="ghost" size="sm" @click="openView(row)">
            {{ t('crud.view') }}
          </Button>
          <Button v-auth="'example:edit'" variant="ghost" size="sm" @click="openEdit(row)">
            {{ t('crud.edit') }}
          </Button>
          <Popconfirm :title="t('example.deleteConfirm', { name: row.title })" @confirm="deleteRow(row)">
            <Button v-auth="'example:delete'" variant="ghost" size="sm" class="text-destructive hover:text-destructive">
              {{ t('crud.delete') }}
            </Button>
          </Popconfirm>
        </template>
      </DataTable>
    </CardContent>

    <ExampleFormDialog :id="dialog.id" v-model="dialog.open" :mode="dialog.mode" @success="load" />
  </Card>
</template>
