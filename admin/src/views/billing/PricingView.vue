<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { Sku } from '@/types/billing'
import { Pencil, Plus, Trash2 } from 'lucide-vue-next'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import billingApi from '@/api/modules/billing'
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
import { fmtCents } from '@/utils/money'

const { t } = useI18n()

const skus = ref<Sku[]>([])
const loading = ref(false)
const q = ref('')

async function load() {
  loading.value = true
  try {
    const res = await billingApi.listSkus()
    skus.value = res.data
  }
  catch {
    toast.error(t('billing.loadFail'))
  }
  finally {
    loading.value = false
  }
}

onMounted(load)

// ——— 时长费配置（单价 元/台/分钟，输入用元、存分）———
const rtUnitYuan = ref(0)
const rtLowYuan = ref(0)
const rtSaving = ref(false)
async function loadRuntimeConfig() {
  try {
    const { data } = await billingApi.getRuntimeConfig()
    rtUnitYuan.value = data.unit_price_cents_per_minute / 100
    rtLowYuan.value = data.low_balance_alert_cents / 100
  }
  catch { /* ignore */ }
}
async function saveRuntimeConfig() {
  rtSaving.value = true
  try {
    await billingApi.saveRuntimeConfig({
      unit_price_cents_per_minute: Math.round(rtUnitYuan.value * 100),
      low_balance_alert_cents: Math.round(rtLowYuan.value * 100),
    })
    toast.success(t('crud.updateOk'))
  }
  catch {
    toast.error(t('billing.loadFail'))
  }
  finally { rtSaving.value = false }
}
onMounted(loadRuntimeConfig)

const CATEGORIES = ['instance_fee', 'boot_pack', 'time_pack'] as const

function catLabel(cat: string) {
  const key = `billing.cat_${cat}` as const
  return t(key)
}

function catVariant(cat: string): 'default' | 'secondary' | 'outline' {
  if (cat === 'instance_fee') return 'default'
  if (cat === 'boot_pack') return 'secondary'
  return 'outline'
}

const columns = computed<ColumnDef<Sku>[]>(() => [
  { accessorKey: 'code', id: 'code', header: t('billing.colCode'), meta: { label: 'billing.colCode' } },
  { accessorKey: 'category', id: 'category', header: t('billing.colCategory'), meta: { label: 'billing.colCategory' } },
  { accessorKey: 'name', id: 'name', header: t('billing.colName'), meta: { label: 'billing.colName' } },
  { accessorKey: 'unit_price_cents', id: 'unit_price_cents', header: t('billing.colPrice'), meta: { label: 'billing.colPrice' } },
  { accessorKey: 'unit', id: 'unit', header: t('billing.colUnit'), meta: { label: 'billing.colUnit' } },
  { accessorKey: 'listed', id: 'listed', header: t('billing.colListed'), meta: { label: 'billing.colListed' } },
  { accessorKey: 'sort', id: 'sort', header: t('billing.colSort'), meta: { label: 'billing.colSort' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
])

// —— Dialog ——
const dialogOpen = ref(false)
const editingId = ref<number | null>(null)
const saving = ref(false)

interface FormState {
  code: string
  category: string
  name: string
  description: string
  unit_price_yuan: number
  unit: string
  listed: boolean
  sort: number
}
const form = reactive<FormState>({
  code: '',
  category: 'instance_fee',
  name: '',
  description: '',
  unit_price_yuan: 0,
  unit: '',
  listed: true,
  sort: 0,
})

function openCreate() {
  editingId.value = null
  Object.assign(form, { code: '', category: 'instance_fee', name: '', description: '', unit_price_yuan: 0, unit: '', listed: true, sort: 0 })
  dialogOpen.value = true
}

function openEdit(s: Sku) {
  editingId.value = s.id
  Object.assign(form, {
    code: s.code,
    category: s.category,
    name: s.name,
    description: s.description ?? '',
    unit_price_yuan: s.unit_price_cents / 100,
    unit: s.unit ?? '',
    listed: s.listed,
    sort: s.sort ?? 0,
  })
  dialogOpen.value = true
}

async function save() {
  if (!form.name.trim()) {
    toast.error(t('billing.errName'))
    return
  }
  const unit_price_cents = Math.round(form.unit_price_yuan * 100)
  saving.value = true
  try {
    if (editingId.value === null) {
      await billingApi.createSku({
        code: form.code,
        category: form.category,
        name: form.name,
        description: form.description || undefined,
        unit_price_cents,
        unit: form.unit || undefined,
        listed: form.listed,
        sort: form.sort,
      })
      toast.success(t('crud.createOk'))
    }
    else {
      await billingApi.updateSku(editingId.value, {
        name: form.name,
        description: form.description || undefined,
        unit_price_cents,
        unit: form.unit || undefined,
        listed: form.listed,
        sort: form.sort,
      })
      toast.success(t('crud.updateOk'))
    }
    dialogOpen.value = false
    load()
  }
  catch {
    toast.error(editingId.value === null ? t('billing.createFail') : t('billing.updateFail'))
  }
  finally {
    saving.value = false
  }
}

async function remove(s: Sku) {
  try {
    await billingApi.deleteSku(s.id)
    toast.success(t('crud.deleteOk'))
    load()
  }
  catch {
    toast.error(t('billing.deleteFail'))
  }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <!-- 时长费配置 -->
    <Card>
      <CardHeader>
        <CardTitle>{{ t('billing.runtimeConfigTitle') }}</CardTitle>
        <CardDescription>{{ t('billing.runtimeConfigDesc') }}</CardDescription>
      </CardHeader>
      <CardContent class="flex flex-wrap items-end gap-4">
        <div class="flex flex-col gap-1.5">
          <Label>{{ t('billing.rtUnitPrice') }}</Label>
          <Input v-model.number="rtUnitYuan" type="number" min="0" step="0.01" class="w-40" />
        </div>
        <div class="flex flex-col gap-1.5">
          <Label>{{ t('billing.rtLowBalance') }}</Label>
          <Input v-model.number="rtLowYuan" type="number" min="0" step="0.01" class="w-40" />
        </div>
        <Button v-auth="'billing:manage'" :disabled="rtSaving" @click="saveRuntimeConfig">
          {{ t('crud.save') }}
        </Button>
      </CardContent>
    </Card>

    <Card>
      <CardHeader class="flex-row items-start justify-between gap-3 space-y-0">
        <div class="space-y-1.5">
          <CardTitle>{{ t('billing.pricingTitle') }}</CardTitle>
          <CardDescription>{{ t('billing.pricingDesc') }}</CardDescription>
        </div>
        <Button v-auth="'billing:manage'" size="sm" @click="openCreate">
          <Plus class="size-4" /> {{ t('billing.addSku') }}
        </Button>
      </CardHeader>
      <CardContent>
        <DataTable
          v-model:search-value="q"
          :columns="columns"
          :data="skus"
          :loading="loading"
          :search-placeholder="t('billing.searchSku')"
        >
          <template #cell-code="{ row }">
            <span class="font-mono text-xs">{{ row.code }}</span>
          </template>
          <template #cell-category="{ row }">
            <Badge :variant="catVariant(row.category)">
              {{ catLabel(row.category) }}
            </Badge>
          </template>
          <template #cell-unit_price_cents="{ row }">
            <span class="tabular-nums">¥{{ fmtCents(row.unit_price_cents) }}</span>
          </template>
          <template #cell-listed="{ row }">
            <Badge :variant="row.listed ? 'default' : 'outline'">
              {{ row.listed ? t('billing.statusActive') : t('billing.statusInactive') }}
            </Badge>
          </template>
          <template #cell-actions="{ row }">
            <Button v-auth="'billing:manage'" variant="ghost" size="sm" @click="openEdit(row)">
              <Pencil class="size-4" />
            </Button>
            <Popconfirm :title="t('billing.deleteConfirm', { name: row.name })" @confirm="remove(row)">
              <Button v-auth="'billing:manage'" variant="ghost" size="sm" class="text-destructive hover:text-destructive">
                <Trash2 class="size-4" />
              </Button>
            </Popconfirm>
          </template>
        </DataTable>
      </CardContent>
    </Card>

    <!-- 新增 / 编辑 SKU Dialog -->
    <Dialog v-model:open="dialogOpen">
      <DialogContent class="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{{ editingId === null ? t('billing.createTitle') : t('billing.editTitle') }}</DialogTitle>
        </DialogHeader>
        <div class="flex flex-col gap-4 py-1">
          <div class="grid grid-cols-2 gap-3">
            <div class="flex flex-col gap-1.5">
              <Label>{{ t('billing.fCode') }}</Label>
              <Input v-model="form.code" :placeholder="t('billing.fCodePlaceholder')" :disabled="editingId !== null" class="font-mono" />
            </div>
            <div class="flex flex-col gap-1.5">
              <Label>{{ t('billing.fCategory') }}</Label>
              <Select
                :model-value="form.category"
                :disabled="editingId !== null"
                @update:model-value="(v) => { form.category = String(v) }"
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="cat in CATEGORIES" :key="cat" :value="cat">
                    {{ catLabel(cat) }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <div class="flex flex-col gap-1.5">
            <Label>{{ t('billing.fName') }}</Label>
            <Input v-model="form.name" :placeholder="t('billing.fNamePlaceholder')" />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label>{{ t('billing.fDescription') }}</Label>
            <Input v-model="form.description" :placeholder="t('billing.fDescriptionPlaceholder')" />
          </div>
          <div class="grid grid-cols-2 gap-3">
            <div class="flex flex-col gap-1.5">
              <Label>{{ t('billing.fPrice') }}</Label>
              <div class="flex items-center gap-2">
                <span class="text-muted-foreground text-sm">¥</span>
                <Input v-model.number="form.unit_price_yuan" type="number" min="0" step="0.01" class="tabular-nums" />
              </div>
            </div>
            <div class="flex flex-col gap-1.5">
              <Label>{{ t('billing.fUnit') }}</Label>
              <Input v-model="form.unit" :placeholder="t('billing.fUnitPlaceholder')" />
            </div>
          </div>
          <div class="grid grid-cols-2 gap-3">
            <div class="flex flex-col gap-1.5">
              <Label>{{ t('billing.fSort') }}</Label>
              <Input v-model.number="form.sort" type="number" min="0" />
            </div>
            <div class="flex items-center justify-between rounded-md border px-3 py-2">
              <Label class="cursor-pointer">{{ t('billing.fStatus') }}</Label>
              <Switch v-model="form.listed" />
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="dialogOpen = false">{{ t('crud.cancel') }}</Button>
          <Button :disabled="saving" @click="save">{{ t('crud.confirm') }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
