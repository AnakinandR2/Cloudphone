<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { Note } from '@/types/note'
import { Plus } from 'lucide-vue-next'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'

import noteApi from '@/api/modules/note'
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
import NoteFormDialog from './NoteFormDialog.vue'

const { t } = useI18n()
const data = ref<Note[]>([])
const loading = ref(false)
const filters = reactive({ q: '' })
useQuerySync(filters, { q: '' })

const columns = computed<ColumnDef<Note>[]>(() => [
  { accessorKey: 'id', id: 'id', header: t('table.id'), meta: { label: 'table.id' } },
  { accessorKey: 'title', id: 'title', header: t('note.colTitle'), meta: { label: 'note.colTitle' } },
  { accessorKey: 'content', id: 'content', header: t('note.colContent'), meta: { label: 'note.colContent' } },
  { accessorKey: 'created_at', id: 'created_at', header: t('table.createdAt'), meta: { label: 'table.createdAt' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
])

const dialog = ref({ open: false, id: 0, mode: 'create' as 'create' | 'edit' | 'view' })

async function load() {
  loading.value = true
  try {
    const res = await noteApi.list({ page: 1, size: 999 })
    data.value = res.data.list
  }
  finally {
    loading.value = false
  }
}
function openCreate() {
  dialog.value = { open: true, id: 0, mode: 'create' }
}
function openView(row: Note) {
  dialog.value = { open: true, id: row.id, mode: 'view' }
}
function openEdit(row: Note) {
  dialog.value = { open: true, id: row.id, mode: 'edit' }
}
async function deleteRow(row: Note) {
  await noteApi.delete(row.id)
  toast.success(t('crud.deleteOk'))
  load()
}

onMounted(load)
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ t('note.title') }}</CardTitle>
      <CardDescription>{{ t('note.desc') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <DataTable
        v-model:search-value="filters.q"
        :columns="columns"
        :data="data"
        :loading="loading"
        :search-placeholder="t('note.searchPlaceholder')"
      >
        <template #actions>
          <Button size="sm" @click="openCreate">
            <Plus class="size-4" /> {{ t('note.add') }}
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
          <Button variant="ghost" size="sm" @click="openEdit(row)">
            {{ t('crud.edit') }}
          </Button>
          <Popconfirm :title="t('note.deleteConfirm', { name: row.title })" @confirm="deleteRow(row)">
            <Button variant="ghost" size="sm" class="text-destructive hover:text-destructive">
              {{ t('crud.delete') }}
            </Button>
          </Popconfirm>
        </template>
      </DataTable>
    </CardContent>

    <NoteFormDialog :id="dialog.id" v-model="dialog.open" :mode="dialog.mode" @success="load" />
  </Card>
</template>
