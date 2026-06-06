<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { User } from '@/types/user'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { toast } from 'vue-sonner'
import userApi from '@/api/modules/user'
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

const { t } = useI18n()
const data = ref<User[]>([])
const filters = reactive({ q: '' })
useQuerySync(filters, { q: '' })

const columns = computed<ColumnDef<User>[]>(() => [
  { accessorKey: 'id', id: 'id', header: t('table.id'), meta: { label: 'table.id' } },
  { accessorKey: 'phone', id: 'phone', header: t('users.colPhone'), meta: { label: 'users.colPhone' } },
  { accessorKey: 'nickname', id: 'nickname', header: t('users.colNickname'), meta: { label: 'users.colNickname' } },
  { accessorKey: 'is_active', id: 'status', header: t('table.status'), meta: { label: 'table.status' } },
  { accessorKey: 'created_at', id: 'created_at', header: t('users.colCreatedAt'), meta: { label: 'users.colCreatedAt' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
])

const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const res = await userApi.list({ page: 1, size: 999 })
    data.value = res.data.list
  }
  finally {
    loading.value = false
  }
}

async function toggleStatus(row: User) {
  await userApi.setStatus(row.id, !row.is_active)
  toast.success(row.is_active ? t('users.disableOk') : t('users.enableOk'))
  load()
}

onMounted(load)
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ t('users.title') }}</CardTitle>
      <CardDescription>{{ t('users.desc') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <DataTable
        v-model:search-value="filters.q"
        :columns="columns"
        :data="data"
        :loading="loading"
        :search-placeholder="t('users.searchPlaceholder')"
      >
        <template #cell-id="{ row }">
          <span class="text-muted-foreground">#{{ row.id }}</span>
        </template>
        <template #cell-phone="{ row }">
          <div class="flex items-center gap-2">
            <Avatar class="size-7">
              <AvatarImage :src="row.avatar" :alt="row.nickname || row.phone" />
              <AvatarFallback>{{ (row.nickname || row.phone).slice(0, 2).toUpperCase() }}</AvatarFallback>
            </Avatar>
            <span class="font-medium tabular-nums">{{ row.phone }}</span>
          </div>
        </template>
        <template #cell-nickname="{ row }">
          {{ row.nickname?.trim() || '-' }}
        </template>
        <template #cell-status="{ row }">
          <Badge :variant="row.is_active ? 'default' : 'destructive'">
            {{ row.is_active ? t('table.enabled') : t('table.disabled') }}
          </Badge>
        </template>
        <template #cell-created_at="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.created_at) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <Popconfirm
            :title="row.is_active ? t('users.disableConfirm', { name: row.phone }) : t('users.enableConfirm', { name: row.phone })"
            @confirm="toggleStatus(row)"
          >
            <Button
              v-auth="'user:manage'"
              variant="ghost"
              size="sm"
              :class="row.is_active ? 'text-destructive hover:text-destructive' : ''"
            >
              {{ row.is_active ? t('users.disable') : t('users.enable') }}
            </Button>
          </Popconfirm>
        </template>
      </DataTable>
    </CardContent>
  </Card>
</template>
