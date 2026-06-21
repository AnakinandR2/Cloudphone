<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { Partner } from '@/types/partner'
import { ExternalLink, Plus } from 'lucide-vue-next'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'

import partnerApi from '@/api/modules/partner'
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
import PartnerFormDialog from './PartnerFormDialog.vue'

const { t } = useI18n()
const data = ref<Partner[]>([])
const loading = ref(false)
const filters = reactive({ q: '' })
useQuerySync(filters, { q: '' })

const columns = computed<ColumnDef<Partner>[]>(() => [
  { accessorKey: 'id', id: 'id', header: t('table.id'), meta: { label: 'table.id' } },
  { accessorKey: 'logo_url', id: 'logo_url', header: t('partner.colLogo'), enableHiding: false, meta: { label: 'partner.colLogo' } },
  { accessorKey: 'name', id: 'name', header: t('partner.colName'), meta: { label: 'partner.colName' } },
  { accessorKey: 'promo_url', id: 'promo_url', header: t('partner.colPromo'), meta: { label: 'partner.colPromo' } },
  { accessorKey: 'sort', id: 'sort', header: t('partner.colSort'), meta: { label: 'partner.colSort' } },
  { accessorKey: 'enabled', id: 'enabled', header: t('partner.colEnabled'), meta: { label: 'partner.colEnabled' } },
  { accessorKey: 'click_count', id: 'click_count', header: t('partner.colClicks'), meta: { label: 'partner.colClicks' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
])

const dialog = ref({ open: false, id: 0, mode: 'create' as 'create' | 'edit' })

async function load() {
  loading.value = true
  try {
    const res = await partnerApi.list({ page: 1, size: 999, kw: filters.q || undefined })
    data.value = res.data.list
  }
  finally {
    loading.value = false
  }
}
function openCreate() {
  dialog.value = { open: true, id: 0, mode: 'create' }
}
function openEdit(row: Partner) {
  dialog.value = { open: true, id: row.id, mode: 'edit' }
}
async function deleteRow(row: Partner) {
  await partnerApi.delete(row.id)
  toast.success(t('crud.deleteOk'))
  load()
}

onMounted(load)
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ t('partner.title') }}</CardTitle>
      <CardDescription>{{ t('partner.desc') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <DataTable
        v-model:search-value="filters.q"
        :columns="columns"
        :data="data"
        :loading="loading"
        :search-placeholder="t('partner.searchPlaceholder')"
      >
        <template #actions>
          <Button v-auth="'partner:manage'" size="sm" @click="openCreate">
            <Plus class="size-4" /> {{ t('partner.add') }}
          </Button>
        </template>

        <template #cell-id="{ row }">
          <span class="text-muted-foreground">#{{ row.id }}</span>
        </template>
        <template #cell-logo_url="{ row }">
          <img v-if="row.logo_url" :src="row.logo_url" :alt="row.name" class="size-9 rounded object-contain bg-muted">
          <span v-else class="text-muted-foreground text-xs">—</span>
        </template>
        <template #cell-name="{ row }">
          <span class="font-medium">{{ row.name }}</span>
        </template>
        <template #cell-promo_url="{ row }">
          <a :href="row.promo_url" target="_blank" rel="noopener noreferrer" class="text-primary inline-flex max-w-xs items-center gap-1 truncate hover:underline">
            <ExternalLink class="size-3.5 shrink-0" />
            <span class="truncate">{{ row.promo_url }}</span>
          </a>
        </template>
        <template #cell-sort="{ row }">
          <span class="tabular-nums">{{ row.sort }}</span>
        </template>
        <template #cell-enabled="{ row }">
          <Badge :variant="row.enabled ? 'default' : 'secondary'">
            {{ row.enabled ? t('partner.enabledYes') : t('partner.enabledNo') }}
          </Badge>
        </template>
        <template #cell-click_count="{ row }">
          <span class="tabular-nums">{{ row.click_count }}</span>
        </template>
        <template #cell-actions="{ row }">
          <Button v-auth="'partner:manage'" variant="ghost" size="sm" @click="openEdit(row)">
            {{ t('crud.edit') }}
          </Button>
          <Popconfirm :title="t('partner.deleteConfirm', { name: row.name })" @confirm="deleteRow(row)">
            <Button v-auth="'partner:manage'" variant="ghost" size="sm" class="text-destructive hover:text-destructive">
              {{ t('crud.delete') }}
            </Button>
          </Popconfirm>
        </template>
      </DataTable>
    </CardContent>

    <PartnerFormDialog :id="dialog.id" v-model="dialog.open" :mode="dialog.mode" @success="load" />
  </Card>
</template>
