<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { AdminScript } from '@/types/automation'
import { FileCode, Power, Trash } from 'lucide-vue-next'
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
import { useQuerySync } from '@/composables/useQuerySync'
import { formatDateTime } from '@/utils/date'

const { t } = useI18n()

const data = ref<AdminScript[]>([])
const loading = ref(false)
const filters = reactive({ q: '' })
useQuerySync(filters, { q: '' })

const columns = computed<ColumnDef<AdminScript>[]>(() => [
  { accessorKey: 'name', id: 'name', header: t('userScripts.colName'), meta: { label: 'userScripts.colName' } },
  { accessorKey: 'uploaderId', id: 'uploaderId', header: t('userScripts.colUploader'), meta: { label: 'userScripts.colUploader' } },
  { accessorKey: 'version', id: 'version', header: t('userScripts.colVersion'), meta: { label: 'userScripts.colVersion' } },
  { accessorKey: 'updateTime', id: 'updateTime', header: t('userScripts.colUpdated'), meta: { label: 'userScripts.colUpdated' } },
  { accessorKey: 'status', id: 'status', header: t('userScripts.colStatus'), meta: { label: 'userScripts.colStatus' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'userScripts.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
])

async function load() {
  loading.value = true
  try {
    const { data: d } = await automationApi.userScripts()
    data.value = d ?? []
  }
  finally {
    loading.value = false
  }
}
onMounted(load)

async function toggle(s: AdminScript) {
  await automationApi.userToggle(s.id, s.status !== 'enabled')
  toast.success(t('userScripts.opOk'))
  load()
}
async function remove(s: AdminScript) {
  await automationApi.userDelete(s.id)
  toast.success(t('userScripts.deleteOk'))
  load()
}
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ t('userScripts.title') }}</CardTitle>
      <CardDescription>{{ t('userScripts.desc') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <DataTable
        v-model:search-value="filters.q"
        :columns="columns"
        :data="data"
        :loading="loading"
        :get-row-id="(r) => String(r.id)"
        :search-placeholder="t('userScripts.searchPlaceholder')"
      >
        <template #cell-name="{ row }">
          <span class="flex items-center gap-2 font-medium"><FileCode class="size-4 text-muted-foreground" />{{ row.name }}</span>
        </template>
        <template #cell-uploaderId="{ row }">
          <span class="tabular-nums text-muted-foreground">#{{ row.uploaderId }}</span>
        </template>
        <template #cell-version="{ row }">
          <span class="tabular-nums text-muted-foreground">{{ row.version }}</span>
        </template>
        <template #cell-updateTime="{ row }">
          <span class="tabular-nums text-muted-foreground">{{ formatDateTime(row.updateTime) }}</span>
        </template>
        <template #cell-status="{ row }">
          <Badge :variant="row.status === 'enabled' ? 'default' : 'secondary'">
            {{ row.status === 'enabled' ? t('userScripts.enabled') : t('userScripts.disabled') }}
          </Badge>
        </template>
        <template #cell-actions="{ row }">
          <Button variant="ghost" size="icon" class="size-7" :title="row.status === 'enabled' ? t('userScripts.takedown') : t('userScripts.restore')" @click="toggle(row)">
            <Power class="size-3.5" />
          </Button>
          <Popconfirm :title="t('userScripts.delConfirm', { name: row.name })" tone="danger" @confirm="remove(row)">
            <Button variant="ghost" size="icon" class="size-7 text-destructive hover:text-destructive" :title="t('userScripts.delete')">
              <Trash class="size-3.5" />
            </Button>
          </Popconfirm>
        </template>
      </DataTable>
    </CardContent>
  </Card>
</template>
