<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { AutomationScript } from '@/types/automation'
import { Code2, FileCode, Pencil, Play, Plus, Power, Trash2 } from 'lucide-vue-next'
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useQuerySync } from '@/composables/useQuerySync'
import { formatDateTime } from '@/utils/date'
import ScriptEditDialog from './ScriptEditDialog.vue'
import TaskCreateDialog from './TaskCreateDialog.vue'

const { t } = useI18n()

const mine = ref<AutomationScript[]>([])
const store = ref<AutomationScript[]>([])
const loading = ref(false)
const filters = reactive({ q: '' })
useQuerySync(filters, { q: '' })
const editDialog = ref<{ open: boolean, script: AutomationScript | null }>({ open: false, script: null })
const taskDialog = ref<{ open: boolean, scriptId: number }>({ open: false, scriptId: 0 })

const columns = computed<ColumnDef<AutomationScript>[]>(() => [
  { accessorKey: 'name', id: 'name', header: t('script.colName'), meta: { label: 'script.colName' } },
  { accessorKey: 'version', id: 'version', header: t('script.colVersion'), meta: { label: 'script.colVersion' } },
  { accessorKey: 'description', id: 'description', header: t('script.colDesc'), meta: { label: 'script.colDesc' } },
  { accessorKey: 'updateTime', id: 'updateTime', header: t('script.colUpdated'), meta: { label: 'script.colUpdated' } },
  { accessorKey: 'status', id: 'status', header: t('script.colStatus'), meta: { label: 'script.colStatus' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
])

async function load() {
  loading.value = true
  try {
    const [m, s] = await Promise.all([automationApi.listScripts(), automationApi.storeScripts()])
    mine.value = m.data ?? []
    store.value = s.data ?? []
  }
  finally {
    loading.value = false
  }
}
onMounted(load)

function openNew() {
  editDialog.value = { open: true, script: null }
}
function openEdit(s: AutomationScript) {
  editDialog.value = { open: true, script: s }
}
function runWith(scriptId: number) {
  taskDialog.value = { open: true, scriptId }
}
async function toggle(s: AutomationScript) {
  await automationApi.toggleScript(s.id, s.status !== 'enabled')
  toast.success(t('script.saveOk'))
  load()
}
async function remove(s: AutomationScript) {
  await automationApi.deleteScript(s.id)
  toast.success(t('crud.deleteOk'))
  load()
}
</script>

<template>
  <Card>
    <CardHeader>
      <div class="flex items-start justify-between gap-4">
        <div class="space-y-1.5">
          <CardTitle>{{ t('script.title') }}</CardTitle>
          <CardDescription>{{ t('script.desc') }}</CardDescription>
        </div>
        <Button size="sm" class="shrink-0" @click="openNew">
          <Plus class="size-4" /> {{ t('script.newScript') }}
        </Button>
      </div>
    </CardHeader>
    <CardContent>
      <Tabs default-value="mine">
        <TabsList>
          <TabsTrigger value="mine">
            {{ t('script.tabMine') }}
          </TabsTrigger>
          <TabsTrigger value="store">
            {{ t('script.tabStore') }}
          </TabsTrigger>
        </TabsList>

        <!-- 我的脚本 -->
        <TabsContent value="mine" class="mt-4">
          <DataTable
            v-model:search-value="filters.q"
            :columns="columns"
            :data="mine"
            :loading="loading"
            :get-row-id="(r) => String(r.id)"
            :search-placeholder="t('script.searchMine')"
          >
            <template #cell-name="{ row }">
              <span class="flex items-center gap-2 font-medium"><FileCode class="size-4 text-muted-foreground" />{{ row.name }}
                <Badge variant="secondary" class="font-normal"><Code2 class="mr-1 size-3" />Lua</Badge>
              </span>
            </template>
            <template #cell-version="{ row }">
              <span class="tabular-nums text-muted-foreground">{{ row.version }}</span>
            </template>
            <template #cell-description="{ row }">
              <span class="block max-w-xs truncate text-muted-foreground">{{ row.description || '—' }}</span>
            </template>
            <template #cell-updateTime="{ row }">
              <span class="tabular-nums text-muted-foreground">{{ formatDateTime(row.updateTime) }}</span>
            </template>
            <template #cell-status="{ row }">
              <Badge :variant="row.status === 'enabled' ? 'default' : 'secondary'">
                {{ row.status === 'enabled' ? t('script.enabled') : t('script.disabled') }}
              </Badge>
            </template>
            <template #cell-actions="{ row }">
              <Button variant="ghost" size="icon" class="size-7" :title="t('script.run')" :disabled="row.status !== 'enabled'" @click="runWith(row.id)">
                <Play class="size-3.5" />
              </Button>
              <Button variant="ghost" size="icon" class="size-7" :title="t('crud.edit')" @click="openEdit(row)">
                <Pencil class="size-3.5" />
              </Button>
              <Button variant="ghost" size="icon" class="size-7" :title="row.status === 'enabled' ? t('script.disable') : t('script.enable')" @click="toggle(row)">
                <Power class="size-3.5" />
              </Button>
              <Popconfirm :title="t('script.delConfirm', { name: row.name })" tone="danger" @confirm="remove(row)">
                <Button variant="ghost" size="icon" class="size-7 text-destructive hover:text-destructive" :title="t('crud.delete')">
                  <Trash2 class="size-3.5" />
                </Button>
              </Popconfirm>
            </template>
          </DataTable>
        </TabsContent>

        <!-- 脚本商店 -->
        <TabsContent value="store" class="mt-4">
          <div v-if="!store.length" class="rounded-lg border border-dashed p-10 text-center text-sm text-muted-foreground">
            {{ t('script.emptyStore') }}
          </div>
          <div v-else class="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-3">
            <div v-for="s in store" :key="s.id" class="flex flex-col gap-3 rounded-lg border p-4">
              <div class="flex items-start gap-3">
                <div class="flex size-11 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                  <FileCode class="size-5" />
                </div>
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-1.5">
                    <span class="truncate font-medium">{{ s.name }}</span>
                    <Badge class="h-4 px-1 text-[10px]">{{ t('script.official') }}</Badge>
                  </div>
                  <div class="mt-0.5 text-xs text-muted-foreground">{{ s.version }}</div>
                </div>
              </div>
              <p class="line-clamp-2 text-xs text-muted-foreground">
                {{ s.description || '—' }}
              </p>
              <div class="mt-auto flex justify-end">
                <Button size="sm" class="h-7" @click="runWith(s.id)">
                  <Play class="size-3.5" /> {{ t('script.useToRun') }}
                </Button>
              </div>
            </div>
          </div>
        </TabsContent>
      </Tabs>
    </CardContent>

    <ScriptEditDialog v-model="editDialog.open" :script="editDialog.script" @saved="load" />
    <TaskCreateDialog v-model="taskDialog.open" :preset-script-id="taskDialog.scriptId" @created="() => {}" />
  </Card>
</template>
