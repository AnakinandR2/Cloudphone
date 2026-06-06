<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import { Pencil, Plus, Trash2 } from 'lucide-vue-next'
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
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
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

const { t } = useI18n()

type Category = 'instance' | 'hours'
interface Sku {
  id: number
  name: string
  category: Category
  unit: string
  price: number
  status: boolean
}

// 原型：本地可编辑的 SKU 列表（无后端）。
const skus = ref<Sku[]>([
  { id: 1, name: '云手机实例 · 按月', category: 'instance', unit: '月', price: 30, status: true },
  { id: 2, name: '云手机实例 · 按季', category: 'instance', unit: '季', price: 76.5, status: true },
  { id: 3, name: '云手机实例 · 按年', category: 'instance', unit: '年', price: 252, status: true },
  { id: 4, name: '时长包 · 100 小时', category: 'hours', unit: '100 小时', price: 20, status: true },
  { id: 5, name: '时长包 · 500 小时', category: 'hours', unit: '500 小时', price: 90, status: true },
  { id: 6, name: '时长包 · 1000 小时', category: 'hours', unit: '1000 小时', price: 160, status: true },
  { id: 7, name: '时长 · 按小时', category: 'hours', unit: '小时', price: 0.2, status: false },
])
const q = ref('')
let seq = skus.value.length

function money(n: number) {
  return n.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

const columns = computed<ColumnDef<Sku>[]>(() => [
  { accessorKey: 'name', id: 'name', header: t('billing.colName'), meta: { label: 'billing.colName' } },
  { accessorKey: 'category', id: 'category', header: t('billing.colCategory'), meta: { label: 'billing.colCategory' } },
  { accessorKey: 'unit', id: 'unit', header: t('billing.colUnit'), meta: { label: 'billing.colUnit' } },
  { accessorKey: 'price', id: 'price', header: t('billing.colPrice'), meta: { label: 'billing.colPrice' } },
  { accessorKey: 'status', id: 'status', header: t('billing.colStatus'), meta: { label: 'billing.colStatus' } },
  { id: 'actions', header: t('crud.actions'), enableHiding: false, meta: { label: 'crud.actions' } },
])

// —— 新增 / 编辑弹框 ——
const dialogOpen = ref(false)
const editingId = ref<number | null>(null)
const form = reactive<Omit<Sku, 'id'>>({ name: '', category: 'instance', unit: '', price: 0, status: true })

function openCreate() {
  editingId.value = null
  Object.assign(form, { name: '', category: 'instance', unit: '', price: 0, status: true })
  dialogOpen.value = true
}
function openEdit(s: Sku) {
  editingId.value = s.id
  Object.assign(form, { name: s.name, category: s.category, unit: s.unit, price: s.price, status: s.status })
  dialogOpen.value = true
}
function save() {
  if (!form.name.trim()) {
    toast.error(t('billing.errName'))
    return
  }
  if (editingId.value === null) {
    skus.value.unshift({ id: ++seq, ...form })
    toast.success(t('crud.createOk'))
  }
  else {
    const idx = skus.value.findIndex(s => s.id === editingId.value)
    if (idx >= 0) skus.value[idx] = { id: editingId.value, ...form }
    toast.success(t('crud.updateOk'))
  }
  dialogOpen.value = false
}
function remove(s: Sku) {
  skus.value = skus.value.filter(x => x.id !== s.id)
  toast.success(t('crud.deleteOk'))
}
</script>

<template>
  <Card>
    <CardHeader class="flex-row items-start justify-between gap-3 space-y-0">
      <div class="space-y-1.5">
        <CardTitle>{{ t('billing.pricingTitle') }}</CardTitle>
        <CardDescription>{{ t('billing.pricingDesc') }}</CardDescription>
      </div>
      <Button size="sm" @click="openCreate">
        <Plus class="size-4" /> {{ t('billing.addSku') }}
      </Button>
    </CardHeader>
    <CardContent>
      <DataTable
        v-model:search-value="q"
        :columns="columns"
        :data="skus"
        :search-placeholder="t('billing.searchSku')"
      >
        <template #cell-category="{ row }">
          <Badge :variant="row.category === 'instance' ? 'default' : 'secondary'">
            {{ row.category === 'instance' ? t('billing.catInstance') : t('billing.catHours') }}
          </Badge>
        </template>
        <template #cell-price="{ row }">
          <span class="tabular-nums">¥{{ money(row.price) }}</span>
        </template>
        <template #cell-status="{ row }">
          <Badge :variant="row.status ? 'default' : 'outline'">
            {{ row.status ? t('billing.statusActive') : t('billing.statusInactive') }}
          </Badge>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex items-center justify-end gap-1">
            <Button variant="ghost" size="icon" class="size-8" @click="openEdit(row)">
              <Pencil class="size-4" />
            </Button>
            <Popconfirm :title="t('billing.deleteConfirm', { name: row.name })" @confirm="remove(row)">
              <Button variant="ghost" size="icon" class="text-destructive size-8">
                <Trash2 class="size-4" />
              </Button>
            </Popconfirm>
          </div>
        </template>
      </DataTable>
    </CardContent>
  </Card>

  <!-- 新增 / 编辑 SKU -->
  <Dialog v-model:open="dialogOpen">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ editingId === null ? t('billing.createTitle') : t('billing.editTitle') }}</DialogTitle>
      </DialogHeader>
      <div class="flex flex-col gap-4 py-1">
        <div class="flex flex-col gap-1.5">
          <Label>{{ t('billing.fName') }}</Label>
          <Input v-model="form.name" :placeholder="t('billing.fNamePlaceholder')" />
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div class="flex flex-col gap-1.5">
            <Label>{{ t('billing.fCategory') }}</Label>
            <Select v-model="form.category">
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="instance">{{ t('billing.catInstance') }}</SelectItem>
                <SelectItem value="hours">{{ t('billing.catHours') }}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="flex flex-col gap-1.5">
            <Label>{{ t('billing.fUnit') }}</Label>
            <Input v-model="form.unit" :placeholder="t('billing.fUnitPlaceholder')" />
          </div>
        </div>
        <div class="flex flex-col gap-1.5">
          <Label>{{ t('billing.fPrice') }}</Label>
          <div class="flex items-center gap-2">
            <span class="text-muted-foreground">¥</span>
            <Input v-model.number="form.price" type="number" min="0" step="0.01" class="tabular-nums" />
          </div>
        </div>
        <div class="flex items-center justify-between rounded-md border px-3 py-2">
          <Label class="cursor-pointer">{{ t('billing.fStatus') }}</Label>
          <Switch v-model="form.status" />
        </div>
      </div>
      <DialogFooter>
        <Button variant="outline" @click="dialogOpen = false">{{ t('crud.cancel') }}</Button>
        <Button @click="save">{{ t('crud.confirm') }}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
