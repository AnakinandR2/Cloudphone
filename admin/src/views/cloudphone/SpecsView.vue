<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { PhoneSpec, SpecKind, VirtualSpec } from '@/types/cloudphone'
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'

import cloudphoneApi from '@/api/modules/cloudphone'
import DataTable from '@/components/DataTable.vue'
import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'

const { t } = useI18n()
const route = useRoute()

// 规格类型由路由决定：云手机规格 / 云主机规格 两个独立菜单共用本组件。
const kind = computed<SpecKind>(() => (route.name === 'cpVmSpecs' ? 'vm' : 'phone'))
const isVm = computed(() => kind.value === 'vm')

const data = ref<(PhoneSpec | VirtualSpec)[]>([])
const loading = ref(false)

const columns = computed<ColumnDef<PhoneSpec | VirtualSpec>[]>(() => {
  const cols: ColumnDef<PhoneSpec | VirtualSpec>[] = [
    { accessorKey: 'id', id: 'id', header: t('table.id'), meta: { label: 'table.id' } },
    { accessorKey: 'name', id: 'name', header: t('cloudphone.specName'), meta: { label: 'cloudphone.specName' } },
    { accessorKey: 'zoneName', id: 'zoneName', header: t('cloudphone.zone'), meta: { label: 'cloudphone.zone' } },
    { accessorKey: 'supplier', id: 'supplier', header: t('cloudphone.supplier'), meta: { label: 'cloudphone.supplier' } },
    { accessorKey: 'core', id: 'core', header: t('cloudphone.core'), meta: { label: 'cloudphone.core' } },
    { accessorKey: 'memory', id: 'memory', header: t('cloudphone.memory'), meta: { label: 'cloudphone.memory' } },
    { accessorKey: 'storage', id: 'storage', header: t('cloudphone.storage'), meta: { label: 'cloudphone.storage' } },
  ]
  if (isVm.value) {
    cols.push(
      { accessorKey: 'card', id: 'card', header: t('cloudphone.card'), meta: { label: 'cloudphone.card' } },
      { accessorKey: 'maxPhone', id: 'maxPhone', header: t('cloudphone.maxPhone'), meta: { label: 'cloudphone.maxPhone' } },
    )
  }
  cols.push(
    { accessorKey: 'type', id: 'type', header: t('cloudphone.type'), meta: { label: 'cloudphone.type' } },
    { accessorKey: 'feature', id: 'feature', header: t('cloudphone.feature'), meta: { label: 'cloudphone.feature' } },
  )
  if (isVm.value) {
    cols.push({ accessorKey: 'hasVM', id: 'hasVM', header: t('cloudphone.hasVM'), meta: { label: 'cloudphone.hasVM' } })
  }
  return cols
})

async function loadSpecs() {
  loading.value = true
  try {
    // 不传 zoneId：聚合全部可用区，每条带 zoneName。
    const res = await cloudphoneApi.listSpecs(kind.value)
    data.value = res.data
  }
  finally {
    loading.value = false
  }
}

// 两个规格菜单共用本组件，路由切换时（kind 变化）重新拉取。
watch(kind, loadSpecs)
onMounted(loadSpecs)
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ isVm ? t('menu.cpVmSpecs') : t('menu.cpPhoneSpecs') }}</CardTitle>
      <CardDescription>{{ t('cloudphone.specsDesc') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <DataTable
        :columns="columns"
        :data="data"
        :loading="loading"
        :search-placeholder="t('cloudphone.searchSpec')"
      >
        <template #cell-id="{ row }">
          <span class="text-muted-foreground">#{{ row.id }}</span>
        </template>
        <template #cell-zoneName="{ row }">
          <Badge variant="secondary">
            {{ row.zoneName || '-' }}
          </Badge>
        </template>
        <template #cell-supplier="{ row }">
          <Badge variant="outline">
            {{ row.supplier }}
          </Badge>
        </template>
        <template #cell-core="{ row }">
          <span class="tabular-nums">{{ row.core }}C</span>
        </template>
        <template #cell-memory="{ row }">
          <span class="tabular-nums">{{ row.memory }}G</span>
        </template>
        <template #cell-storage="{ row }">
          <span class="tabular-nums">{{ row.storage }}G</span>
        </template>
        <template #cell-hasVM="{ row }">
          <Badge :variant="(row as VirtualSpec).hasVM ? 'default' : 'secondary'">
            {{ (row as VirtualSpec).hasVM ? t('cloudphone.yes') : t('cloudphone.no') }}
          </Badge>
        </template>
      </DataTable>
    </CardContent>
  </Card>
</template>
