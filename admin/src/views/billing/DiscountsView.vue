<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { DiscountTier, Sku } from '@/types/billing'
import { Pencil, Plus, Trash2 } from 'lucide-vue-next'
import { computed, onMounted, reactive, ref, watch } from 'vue'
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
import { fmtDiscountBps } from '@/utils/money'

const { t } = useI18n()

// SKU selector
const skus = ref<Sku[]>([])
const skusLoading = ref(false)
const selectedSkuId = ref<number | null>(null)

async function loadSkus() {
  skusLoading.value = true
  try {
    const res = await billingApi.listSkus()
    skus.value = res.data
  }
  catch {
    toast.error(t('billing.loadFail'))
  }
  finally {
    skusLoading.value = false
  }
}

// Tiers for the selected SKU
const tiers = ref<DiscountTier[]>([])
const tiersLoading = ref(false)

async function loadTiers(skuId: number) {
  tiersLoading.value = true
  try {
    const res = await billingApi.listTiers(skuId)
    tiers.value = res.data
  }
  catch {
    toast.error(t('billing.loadFail'))
  }
  finally {
    tiersLoading.value = false
  }
}

watch(selectedSkuId, (id) => {
  if (id !== null) {
    loadTiers(id)
  }
  else {
    tiers.value = []
  }
})

onMounted(loadSkus)

const selectedSku = computed(() => skus.value.find(s => s.id === selectedSkuId.value) ?? null)

// cycle_months label
function cycleLabel(months: number): string {
  if (months === 1) return t('billing.cycle_1')
  if (months === 3) return t('billing.cycle_3')
  if (months === 12) return t('billing.cycle_12')
  if (months === 0) return t('billing.cycle_0')
  return t('billing.cycleN', { n: months })
}

// category label (reuse from pricing)
function catLabel(cat: string) {
  return t(`billing.cat_${cat}`)
}

const tierColumns = computed<ColumnDef<DiscountTier>[]>(() => [
  { accessorKey: 'cycle_months', id: 'cycle_months', header: t('billing.colCycleMonths'), meta: { label: 'billing.colCycleMonths' } },
  { accessorKey: 'min_quantity', id: 'min_quantity', header: t('billing.colMinQty'), meta: { label: 'billing.colMinQty' } },
  { accessorKey: 'discount_bps', id: 'discount_bps', header: t('billing.colDiscountBps'), meta: { label: 'billing.colDiscountBps' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
])

// —— Dialog ——
const dialogOpen = ref(false)
const editingTierId = ref<number | null>(null)
const saving = ref(false)

interface TierForm {
  cycle_months: number
  min_quantity: number
  // UX: input as 折 (e.g. 8.5) — displayed as "8.5 折 (= 8500 bps)"
  // We store as 折 (float), convert to bps on save: Math.round(zhe * 1000)
  discount_zhe: number
}
const form = reactive<TierForm>({
  cycle_months: 1,
  min_quantity: 1,
  discount_zhe: 9,
})

function openCreateTier() {
  editingTierId.value = null
  Object.assign(form, { cycle_months: 1, min_quantity: 1, discount_zhe: 9 })
  dialogOpen.value = true
}

function openEditTier(tier: DiscountTier) {
  editingTierId.value = tier.id
  Object.assign(form, {
    cycle_months: tier.cycle_months,
    min_quantity: tier.min_quantity,
    discount_zhe: +(tier.discount_bps / 1000).toFixed(2),
  })
  dialogOpen.value = true
}

async function saveTier() {
  if (selectedSkuId.value === null) return
  const discount_bps = Math.round(form.discount_zhe * 1000)
  saving.value = true
  try {
    if (editingTierId.value === null) {
      await billingApi.createTier(selectedSkuId.value, {
        cycle_months: form.cycle_months,
        min_quantity: form.min_quantity,
        discount_bps,
      })
      toast.success(t('crud.createOk'))
    }
    else {
      await billingApi.updateTier(editingTierId.value, {
        cycle_months: form.cycle_months,
        min_quantity: form.min_quantity,
        discount_bps,
      })
      toast.success(t('crud.updateOk'))
    }
    dialogOpen.value = false
    loadTiers(selectedSkuId.value)
  }
  catch {
    toast.error(editingTierId.value === null ? t('billing.createFail') : t('billing.updateFail'))
  }
  finally {
    saving.value = false
  }
}

async function removeTier(tier: DiscountTier) {
  try {
    await billingApi.deleteTier(tier.id)
    toast.success(t('crud.deleteOk'))
    if (selectedSkuId.value !== null) {
      loadTiers(selectedSkuId.value)
    }
  }
  catch {
    toast.error(t('billing.deleteFail'))
  }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 class="text-xl font-semibold tracking-tight">{{ t('billing.discountsTitle') }}</h1>
        <p class="text-muted-foreground mt-1 text-sm">{{ t('billing.discountsDesc') }}</p>
      </div>
    </div>

    <!-- SKU 选择器 -->
    <Card>
      <CardHeader>
        <CardTitle class="text-base">{{ t('billing.selectSkuTitle') }}</CardTitle>
        <CardDescription>{{ t('billing.selectSkuDesc') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <div class="flex flex-wrap gap-2">
          <Button
            v-for="sku in skus"
            :key="sku.id"
            :variant="selectedSkuId === sku.id ? 'default' : 'outline'"
            size="sm"
            :disabled="skusLoading"
            @click="selectedSkuId = sku.id"
          >
            <span class="font-mono text-xs mr-1.5 opacity-60">{{ sku.code }}</span>
            {{ sku.name }}
            <Badge class="ml-1.5" :variant="selectedSkuId === sku.id ? 'secondary' : 'outline'" style="font-size:0.65rem">
              {{ catLabel(sku.category) }}
            </Badge>
          </Button>
          <p v-if="!skusLoading && !skus.length" class="text-muted-foreground text-sm py-1">
            {{ t('billing.noSkus') }}
          </p>
        </div>
      </CardContent>
    </Card>

    <!-- 阶梯列表 -->
    <Card v-if="selectedSku">
      <CardHeader class="flex-row items-start justify-between gap-3 space-y-0">
        <div class="space-y-1.5">
          <CardTitle class="text-base">
            {{ t('billing.tierListTitle', { name: selectedSku.name }) }}
          </CardTitle>
          <CardDescription>{{ t('billing.tierListDesc') }}</CardDescription>
        </div>
        <Button v-auth="'billing:manage'" size="sm" @click="openCreateTier">
          <Plus class="size-4" /> {{ t('billing.addTier') }}
        </Button>
      </CardHeader>
      <CardContent>
        <DataTable
          :columns="tierColumns"
          :data="tiers"
          :loading="tiersLoading"
          :search-placeholder="''"
        >
          <template #cell-cycle_months="{ row }">
            <span>{{ cycleLabel(row.cycle_months) }}</span>
          </template>
          <template #cell-min_quantity="{ row }">
            <span class="tabular-nums">{{ row.min_quantity }}</span>
          </template>
          <template #cell-discount_bps="{ row }">
            <span class="tabular-nums">
              {{ fmtDiscountBps(row.discount_bps) || t('billing.fullPrice') }}
              <span class="text-muted-foreground ml-1 text-xs">({{ row.discount_bps }} bps)</span>
            </span>
          </template>
          <template #cell-actions="{ row }">
            <Button v-auth="'billing:manage'" variant="ghost" size="sm" @click="openEditTier(row)">
              <Pencil class="size-4" />
            </Button>
            <Popconfirm :title="t('billing.deleteTierConfirm')" @confirm="removeTier(row)">
              <Button v-auth="'billing:manage'" variant="ghost" size="sm" class="text-destructive hover:text-destructive">
                <Trash2 class="size-4" />
              </Button>
            </Popconfirm>
          </template>
        </DataTable>
      </CardContent>
    </Card>

    <div v-else-if="!skusLoading" class="text-muted-foreground py-8 text-center text-sm">
      {{ t('billing.selectSkuHint') }}
    </div>

    <!-- 新增 / 编辑 阶梯 Dialog -->
    <Dialog v-model:open="dialogOpen">
      <DialogContent class="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>{{ editingTierId === null ? t('billing.createTierTitle') : t('billing.editTierTitle') }}</DialogTitle>
        </DialogHeader>
        <div class="flex flex-col gap-4 py-1">
          <div class="flex flex-col gap-1.5">
            <Label>{{ t('billing.fCycleMonths') }}</Label>
            <Input v-model.number="form.cycle_months" type="number" min="0" step="1" />
            <p class="text-muted-foreground text-xs">{{ t('billing.fCycleMonthsHint') }}</p>
          </div>
          <div class="flex flex-col gap-1.5">
            <Label>{{ t('billing.fMinQty') }}</Label>
            <Input v-model.number="form.min_quantity" type="number" min="1" step="1" />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label>{{ t('billing.fDiscountZhe') }}</Label>
            <div class="flex items-center gap-2">
              <Input v-model.number="form.discount_zhe" type="number" min="0.1" max="10" step="0.1" class="tabular-nums" />
              <span class="text-muted-foreground text-sm shrink-0">{{ t('billing.discountUnit') }}</span>
            </div>
            <p class="text-muted-foreground text-xs">
              {{ t('billing.fDiscountZheHint', { bps: Math.round(form.discount_zhe * 1000) }) }}
            </p>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="dialogOpen = false">{{ t('crud.cancel') }}</Button>
          <Button :disabled="saving" @click="saveTier">{{ t('crud.confirm') }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
