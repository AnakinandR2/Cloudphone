<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { AutomationScript } from '@/types/automation'
import { Code2, FileCode, Pencil, Plus, Power, Trash, Upload } from 'lucide-vue-next'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import automationApi from '@/api/modules/automation'
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { useQuerySync } from '@/composables/useQuerySync'
import { formatDateTime } from '@/utils/date'
import ParamsSchemaEditor from './ParamsSchemaEditor.vue'

const { t } = useI18n()

const data = ref<AutomationScript[]>([])
const loading = ref(false)
const filters = reactive({ q: '' })
useQuerySync(filters, { q: '' })
const dialog = ref(false)
const editing = ref<AutomationScript | null>(null)
const form = ref({ name: '', description: '', luaContent: '', paramsSchema: '', fileName: '' })
const saving = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

const columns = computed<ColumnDef<AutomationScript>[]>(() => [
  { accessorKey: 'name', id: 'name', header: t('scriptStore.colName'), meta: { label: 'scriptStore.colName' } },
  { accessorKey: 'description', id: 'description', header: t('scriptStore.colDesc'), meta: { label: 'scriptStore.colDesc' } },
  { accessorKey: 'version', id: 'version', header: t('scriptStore.colVersion'), meta: { label: 'scriptStore.colVersion' } },
  { accessorKey: 'updateTime', id: 'updateTime', header: t('scriptStore.colUpdated'), meta: { label: 'scriptStore.colUpdated' } },
  { accessorKey: 'status', id: 'status', header: t('scriptStore.colStatus'), meta: { label: 'scriptStore.colStatus' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'scriptStore.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
])

async function load() {
  loading.value = true
  try {
    const { data: d } = await automationApi.storeList()
    data.value = d ?? []
  }
  finally {
    loading.value = false
  }
}
onMounted(load)

function openNew() {
  editing.value = null
  form.value = { name: '', description: '', luaContent: '', paramsSchema: '', fileName: '' }
  dialog.value = true
}
function openEdit(s: AutomationScript) {
  editing.value = s
  form.value = { name: s.name, description: s.description, luaContent: s.luaContent, paramsSchema: s.paramsSchema ?? '', fileName: s.fileName }
  dialog.value = true
}
function pickFile() {
  fileInput.value?.click()
}
async function onFile(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f)
    return
  form.value.luaContent = await f.text()
  form.value.fileName = f.name
  if (!form.value.name)
    form.value.name = f.name.replace(/\.lua$/i, '')
}
async function save() {
  if (!form.value.name.trim() || !form.value.luaContent.trim()) {
    toast.error(t('scriptStore.errRequired'))
    return
  }
  saving.value = true
  try {
    if (editing.value)
      await automationApi.storeUpdate(editing.value.id, form.value)
    else
      await automationApi.storeCreate(form.value)
    toast.success(t('scriptStore.saveOk'))
    dialog.value = false
    load()
  }
  finally {
    saving.value = false
  }
}
async function toggle(s: AutomationScript) {
  await automationApi.storeToggle(s.id, s.status !== 'enabled')
  toast.success(t('scriptStore.saveOk'))
  load()
}
async function remove(s: AutomationScript) {
  await automationApi.storeDelete(s.id)
  toast.success(t('scriptStore.deleteOk'))
  load()
}
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ t('scriptStore.title') }}</CardTitle>
      <CardDescription>{{ t('scriptStore.desc') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <DataTable
        v-model:search-value="filters.q"
        :columns="columns"
        :data="data"
        :loading="loading"
        :get-row-id="(r) => String(r.id)"
        :search-placeholder="t('scriptStore.searchPlaceholder')"
      >
        <template #actions>
          <Button size="sm" @click="openNew">
            <Plus class="size-4" /> {{ t('scriptStore.new') }}
          </Button>
        </template>

        <template #cell-name="{ row }">
          <span class="flex items-center gap-2 font-medium"><FileCode class="size-4 text-muted-foreground" />{{ row.name }}
            <Badge variant="secondary" class="font-normal"><Code2 class="mr-1 size-3" />Lua</Badge>
          </span>
        </template>
        <template #cell-description="{ row }">
          <span class="block max-w-xs truncate text-muted-foreground">{{ row.description || '—' }}</span>
        </template>
        <template #cell-version="{ row }">
          <span class="tabular-nums text-muted-foreground">{{ row.version }}</span>
        </template>
        <template #cell-updateTime="{ row }">
          <span class="tabular-nums text-muted-foreground">{{ formatDateTime(row.updateTime) }}</span>
        </template>
        <template #cell-status="{ row }">
          <Badge :variant="row.status === 'enabled' ? 'default' : 'secondary'">
            {{ row.status === 'enabled' ? t('scriptStore.enabled') : t('scriptStore.disabled') }}
          </Badge>
        </template>
        <template #cell-actions="{ row }">
          <Button variant="ghost" size="icon" class="size-7" :title="t('scriptStore.edit')" @click="openEdit(row)">
            <Pencil class="size-3.5" />
          </Button>
          <Button variant="ghost" size="icon" class="size-7" :title="row.status === 'enabled' ? t('scriptStore.disable') : t('scriptStore.enable')" @click="toggle(row)">
            <Power class="size-3.5" />
          </Button>
          <Popconfirm :title="t('scriptStore.delConfirm', { name: row.name })" tone="danger" @confirm="remove(row)">
            <Button variant="ghost" size="icon" class="size-7 text-destructive hover:text-destructive" :title="t('scriptStore.delete')">
              <Trash class="size-3.5" />
            </Button>
          </Popconfirm>
        </template>
      </DataTable>
    </CardContent>

    <Dialog v-model:open="dialog">
      <DialogContent class="flex max-h-[88vh] flex-col gap-0 p-0 sm:max-w-2xl">
        <DialogHeader class="border-b p-4">
          <DialogTitle>{{ editing ? t('scriptStore.editTitle') : t('scriptStore.new') }}</DialogTitle>
          <DialogDescription>{{ t('scriptStore.editDesc') }}</DialogDescription>
        </DialogHeader>
        <div class="min-h-0 flex-1 space-y-4 overflow-y-auto p-4">
          <div class="grid gap-2">
            <Label>{{ t('scriptStore.colName') }}</Label>
            <Input v-model="form.name" :placeholder="t('scriptStore.namePh')" />
          </div>
          <div class="grid gap-2">
            <Label>{{ t('scriptStore.colDesc') }}</Label>
            <Textarea v-model="form.description" :rows="2" />
          </div>
          <div class="grid gap-2">
            <div class="flex items-center justify-between">
              <Label>{{ t('scriptStore.code') }} <span class="text-xs text-muted-foreground">(Lua)</span></Label>
              <Button variant="outline" size="sm" class="h-7" @click="pickFile">
                <Upload class="size-3.5" /> {{ t('scriptStore.upload') }}
              </Button>
              <input ref="fileInput" type="file" accept=".lua,text/*" class="hidden" @change="onFile">
            </div>
            <Textarea v-model="form.luaContent" :rows="12" class="font-mono text-xs" spellcheck="false" />
          </div>
          <div class="grid gap-2">
            <Label>{{ t('scriptStore.params.title') }} <span class="text-xs text-muted-foreground">{{ t('scriptStore.params.hint') }}</span></Label>
            <ParamsSchemaEditor v-model="form.paramsSchema" />
          </div>
        </div>
        <DialogFooter class="border-t p-3">
          <Button variant="outline" @click="dialog = false">
            {{ t('scriptStore.cancel') }}
          </Button>
          <Button :disabled="saving" @click="save">
            {{ t('scriptStore.save') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </Card>
</template>
