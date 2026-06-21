<script setup lang="ts">
import type { LicenseKind, LicenseUnit } from '@/types/billing'
import { Search } from 'lucide-vue-next'
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import billingApi from '@/api/modules/billing'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatDate } from '@/utils/date'

// 续费单元表：ID / 创建 / 到期 / 坐着的实例[名称,ID,状态]；多选；过滤到期 + 搜索名称。
const props = defineProps<{
  kind: LicenseKind
}>()

// v-model 选中的单元 ID 列表
const selected = defineModel<number[]>({ default: () => [] })
const { t } = useI18n()

const items = ref<LicenseUnit[]>([])
const loading = ref(false)
const keyword = ref('')
const expiringBefore = ref('') // YYYY-MM-DD，过滤到期时间在此之前

async function load() {
  loading.value = true
  try {
    const params: { kind: LicenseKind, keyword?: string, expiring_before?: string } = { kind: props.kind }
    if (keyword.value) params.keyword = keyword.value
    if (expiringBefore.value) params.expiring_before = new Date(`${expiringBefore.value}T23:59:59`).toISOString()
    const res = await billingApi.licenseUnits(params)
    items.value = res.data.items
    // 清理已不在列表中的选中项
    const ids = new Set(items.value.map(u => u.id))
    selected.value = selected.value.filter(id => ids.has(id))
  }
  finally {
    loading.value = false
  }
}

onMounted(load)
watch(() => props.kind, () => {
  selected.value = []
  keyword.value = ''
  expiringBefore.value = ''
  load()
})

const allChecked = computed(() => items.value.length > 0 && selected.value.length === items.value.length)
function toggleAll(v: boolean) {
  selected.value = v ? items.value.map(u => u.id) : []
}
function toggleOne(id: number, v: boolean) {
  if (v) {
    if (!selected.value.includes(id)) selected.value = [...selected.value, id]
  }
  else {
    selected.value = selected.value.filter(x => x !== id)
  }
}

function statusVariant(s: string): 'default' | 'secondary' | 'outline' {
  if (s === 'RUNNING') return 'default'
  if (s === 'STOPPED') return 'secondary'
  return 'outline'
}

defineExpose({ reload: load })
</script>

<template>
  <div class="flex flex-col gap-3">
    <!-- 过滤器：搜索 + 到期前 -->
    <div class="flex flex-wrap items-center gap-2">
      <div class="relative">
        <Search class="text-muted-foreground absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
        <Input
          v-model="keyword"
          class="h-9 w-52 pl-8"
          :placeholder="t('billing.purchase2.renewSearchPlaceholder')"
          @keyup.enter="load"
        />
      </div>
      <div class="flex items-center gap-1.5">
        <span class="text-muted-foreground text-sm">{{ t('billing.purchase2.renewExpiringBefore') }}</span>
        <Input v-model="expiringBefore" type="date" class="h-9 w-40" @change="load" />
      </div>
      <span class="text-muted-foreground ml-auto text-xs">
        {{ t('billing.purchase2.renewSelectedCount', { n: selected.length }) }}
      </span>
    </div>

    <div v-if="loading" class="space-y-2">
      <Skeleton v-for="i in 4" :key="i" class="h-11 w-full rounded-md" />
    </div>

    <div v-else-if="!items.length" class="text-muted-foreground rounded-lg border py-10 text-center text-sm">
      {{ t('billing.purchase2.renewEmpty') }}
    </div>

    <div v-else class="rounded-lg border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead class="w-10">
              <Checkbox :model-value="allChecked" @update:model-value="(v) => toggleAll(v === true)" />
            </TableHead>
            <TableHead>{{ t('billing.purchase2.colUnitId') }}</TableHead>
            <TableHead>{{ t('billing.purchase2.colCreatedAt') }}</TableHead>
            <TableHead>{{ t('billing.purchase2.colExpireAt') }}</TableHead>
            <TableHead>{{ t('billing.purchase2.colSeatedInstance') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow
            v-for="u in items"
            :key="u.id"
            class="cursor-pointer"
            :class="selected.includes(u.id) ? 'bg-primary/5' : ''"
            @click="toggleOne(u.id, !selected.includes(u.id))"
          >
            <TableCell @click.stop>
              <Checkbox :model-value="selected.includes(u.id)" @update:model-value="(v) => toggleOne(u.id, v === true)" />
            </TableCell>
            <TableCell class="font-mono text-xs">
              #{{ u.id }}
            </TableCell>
            <TableCell class="text-muted-foreground tabular-nums">
              {{ formatDate(u.created_at) }}
            </TableCell>
            <TableCell class="tabular-nums">
              {{ formatDate(u.expire_at) }}
            </TableCell>
            <TableCell>
              <div v-if="u.instance" class="flex items-center gap-2">
                <span class="text-sm font-medium">{{ u.instance.name }}</span>
                <span class="text-muted-foreground font-mono text-xs">{{ u.instance.cp_id }}</span>
                <Badge :variant="statusVariant(u.instance.status)" class="text-[10px]">
                  {{ u.instance.status }}
                </Badge>
              </div>
              <span v-else class="text-muted-foreground text-xs">{{ t('billing.purchase2.unitIdle') }}</span>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>
  </div>
</template>
