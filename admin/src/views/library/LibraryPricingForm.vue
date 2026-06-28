<script setup lang="ts">
import type { CustomTierCfg, DurationOptCfg, LibraryPricingConfig, TierCfg } from '@/types/library'
import { GripVertical, Plus, Trash2 } from 'lucide-vue-next'
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import libraryApi from '@/api/modules/library'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { fmtDiscountBps } from '@/utils/money'
import { GIB } from '@/types/library'

const { t } = useI18n()

// ── 表单态：容量按 GiB、价格按「元」编辑，存「字节 / 分」。档位排序由表格顺序决定（拖拽手柄）。
interface TierForm {
  code: string
  name: string
  capacity_gb: number
  monthly_price_yuan: number
  tier_discount_bps: number
  enabled: boolean
}
interface CustomForm {
  enabled: boolean
  min_capacity_gb: number
  price_per_gb_month_yuan: number
  tier_discount_bps: number
}
interface DurationForm {
  days: number
  discount_bps: number
}

const freeQuotaGiB = ref(0)
const tiers = ref<TierForm[]>([])
const custom = reactive<CustomForm>({
  enabled: false,
  min_capacity_gb: 0,
  price_per_gb_month_yuan: 0,
  tier_discount_bps: 10000,
})
const durations = ref<DurationForm[]>([])
const notice = ref('')
const billingNote = ref('')

const loading = ref(false)
const saving = ref(false)

function toTierForm(x: TierCfg): TierForm {
  return {
    code: x.code,
    name: x.name,
    capacity_gb: x.capacity_gb,
    monthly_price_yuan: x.monthly_price_cents / 100,
    tier_discount_bps: x.tier_discount_bps,
    enabled: x.enabled,
  }
}

async function load() {
  loading.value = true
  try {
    const { data } = await libraryApi.getPricing()
    freeQuotaGiB.value = data.free_quota_bytes / GIB
    // 按 sort 升序展示，之后顺序即 sort。
    tiers.value = (data.tiers ?? [])
      .slice()
      .sort((a, b) => a.sort - b.sort)
      .map(toTierForm)
    custom.enabled = data.custom_tier?.enabled ?? false
    custom.min_capacity_gb = data.custom_tier?.min_capacity_gb ?? 0
    custom.price_per_gb_month_yuan = (data.custom_tier?.price_per_gb_month_cents ?? 0) / 100
    custom.tier_discount_bps = data.custom_tier?.tier_discount_bps ?? 10000
    durations.value = (data.duration_options ?? []).map(d => ({ ...d }))
    notice.value = data.notice ?? ''
    billingNote.value = data.billing_note ?? ''
  }
  catch {
    toast.error(t('libraryPricing.loadFail'))
  }
  finally {
    loading.value = false
  }
}
onMounted(load)

function buildPayload(): LibraryPricingConfig {
  // 以当前表格展示顺序重写 sort（0..n-1）。
  const tiersOut: TierCfg[] = tiers.value.map((f, i) => ({
    code: f.code.trim(),
    name: f.name.trim(),
    capacity_gb: Number(f.capacity_gb) || 0,
    monthly_price_cents: Math.round((Number(f.monthly_price_yuan) || 0) * 100),
    tier_discount_bps: Number(f.tier_discount_bps) || 10000,
    enabled: f.enabled,
    sort: i,
  }))
  const customOut: CustomTierCfg = {
    enabled: custom.enabled,
    min_capacity_gb: Number(custom.min_capacity_gb) || 0,
    price_per_gb_month_cents: Math.round((Number(custom.price_per_gb_month_yuan) || 0) * 100),
    tier_discount_bps: Number(custom.tier_discount_bps) || 10000,
  }
  const durationsOut: DurationOptCfg[] = durations.value.map(d => ({
    days: Number(d.days) || 0,
    discount_bps: Number(d.discount_bps) || 10000,
  }))
  return {
    free_quota_bytes: Math.round((Number(freeQuotaGiB.value) || 0) * GIB),
    tiers: tiersOut,
    custom_tier: customOut,
    duration_options: durationsOut,
    notice: notice.value,
    billing_note: billingNote.value,
  }
}

async function save() {
  saving.value = true
  try {
    await libraryApi.savePricing(buildPayload())
    toast.success(t('libraryPricing.savedOk'))
    load()
  }
  catch {
    toast.error(t('libraryPricing.updateFail'))
  }
  finally {
    saving.value = false
  }
}

function addTier() {
  tiers.value.push({
    code: '',
    name: '',
    capacity_gb: 50,
    monthly_price_yuan: 0,
    tier_discount_bps: 10000,
    enabled: true,
  })
}
function removeTier(idx: number) {
  tiers.value.splice(idx, 1)
}
function addDuration() {
  durations.value.push({ days: 30, discount_bps: 10000 })
}
function removeDuration(idx: number) {
  durations.value.splice(idx, 1)
}

// ── 原生 HTML5 拖拽排序（仅左侧手柄可拖，行本身不 draggable，便于在输入框框选文本）。
const dragIndex = ref<number | null>(null)
const overIndex = ref<number | null>(null)
function onDragStart(idx: number) {
  dragIndex.value = idx
}
function onDragOver(idx: number) {
  if (dragIndex.value === null) return
  overIndex.value = idx
}
function onDrop(idx: number) {
  const from = dragIndex.value
  dragIndex.value = null
  overIndex.value = null
  if (from === null || from === idx) return
  const arr = tiers.value
  const [moved] = arr.splice(from, 1)
  arr.splice(idx, 0, moved)
}
function onDragEnd() {
  dragIndex.value = null
  overIndex.value = null
}

// 保存由父页（PricingConfigView）顶部的共用保存按钮触发。
defineExpose({ save })
</script>

<template>
  <div class="space-y-6">
    <p class="text-muted-foreground text-sm">
      {{ t('libraryPricing.desc') }}
    </p>

    <!-- 免费额度 -->
    <div class="flex flex-col gap-1.5">
      <Label>{{ t('libraryPricing.freeQuota') }}</Label>
      <div class="flex items-center gap-2">
        <Input v-model.number="freeQuotaGiB" type="number" min="0" step="0.1" class="w-40 tabular-nums" />
        <span class="text-muted-foreground text-sm">GiB</span>
      </div>
      <p class="text-muted-foreground text-xs">
        {{ t('libraryPricing.freeQuotaHint') }}
      </p>
    </div>

    <!-- 档位表（表格 + 左侧手柄拖拽排序） -->
    <div class="space-y-2">
      <div class="flex items-center justify-between">
        <Label>{{ t('libraryPricing.tiers') }}</Label>
        <Button v-auth="'library:manage'" variant="outline" size="sm" @click="addTier">
          <Plus class="size-4" /> {{ t('libraryPricing.addTier') }}
        </Button>
      </div>
      <p class="text-muted-foreground text-xs">
        {{ t('libraryPricing.tiersHint') }}
      </p>
      <div v-if="tiers.length" class="overflow-x-auto rounded-md border">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-muted-foreground bg-muted/40 border-b text-left text-xs">
              <th class="w-8 px-2 py-2" />
              <th class="w-10 px-2 py-2 text-center">
                #
              </th>
              <th class="px-3 py-2 font-medium">
                {{ t('libraryPricing.tierCode') }}
              </th>
              <th class="px-3 py-2 font-medium">
                {{ t('libraryPricing.tierName') }}
              </th>
              <th class="px-3 py-2 font-medium">
                {{ t('libraryPricing.capacityGB') }}
              </th>
              <th class="px-3 py-2 font-medium">
                {{ t('libraryPricing.monthlyPrice') }}
              </th>
              <th class="px-3 py-2 font-medium">
                {{ t('libraryPricing.tierDiscountBps') }}
              </th>
              <th class="w-16 px-3 py-2 text-center font-medium">
                {{ t('libraryPricing.enabled') }}
              </th>
              <th class="w-12 px-2 py-2" />
            </tr>
          </thead>
          <tbody class="divide-y">
            <tr
              v-for="(tier, idx) in tiers"
              :key="idx"
              class="transition-colors"
              :class="[
                dragIndex === idx ? 'opacity-50' : '',
                overIndex === idx && dragIndex !== null && dragIndex !== idx ? 'bg-accent' : '',
              ]"
              @dragover.prevent="onDragOver(idx)"
              @drop="onDrop(idx)"
            >
              <!-- 拖拽手柄：只有它 draggable -->
              <td class="px-2 py-2 align-middle">
                <span
                  class="text-muted-foreground inline-flex cursor-grab active:cursor-grabbing"
                  :title="t('billing.payDragHint')"
                  draggable="true"
                  @dragstart="onDragStart(idx)"
                  @dragend="onDragEnd"
                >
                  <GripVertical class="size-4" />
                </span>
              </td>
              <td class="text-muted-foreground px-2 py-2 text-center align-middle text-xs tabular-nums">
                {{ idx + 1 }}
              </td>
              <td class="px-3 py-2 align-middle">
                <Input v-model="tier.code" class="h-8 w-28" :placeholder="t('libraryPricing.tierCodePlaceholder')" />
              </td>
              <td class="px-3 py-2 align-middle">
                <Input v-model="tier.name" class="h-8 w-32" />
              </td>
              <td class="px-3 py-2 align-middle">
                <Input v-model.number="tier.capacity_gb" type="number" min="0" step="1" class="h-8 w-24 tabular-nums" />
              </td>
              <td class="px-3 py-2 align-middle">
                <Input v-model.number="tier.monthly_price_yuan" type="number" min="0" step="0.01" class="h-8 w-28 tabular-nums" />
              </td>
              <td class="px-3 py-2 align-middle">
                <div class="flex items-center gap-2">
                  <Input v-model.number="tier.tier_discount_bps" type="number" min="0" max="10000" step="100" class="h-8 w-24 tabular-nums" />
                  <span class="text-muted-foreground whitespace-nowrap text-[11px]">{{ fmtDiscountBps(tier.tier_discount_bps) || t('libraryPricing.fullPrice') }}</span>
                </div>
              </td>
              <td class="px-3 py-2 text-center align-middle">
                <Switch v-model="tier.enabled" />
              </td>
              <td class="px-2 py-2 text-center align-middle">
                <Button variant="ghost" size="sm" class="text-destructive" @click="removeTier(idx)">
                  <Trash2 class="size-4" />
                </Button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-else class="text-muted-foreground text-xs">
        {{ t('libraryPricing.noTiers') }}
      </p>
    </div>

    <!-- 自定义档 -->
    <div class="space-y-3 rounded-md border px-3 py-3">
      <div class="flex items-center justify-between">
        <Label>{{ t('libraryPricing.customTier') }}</Label>
        <div class="flex items-center gap-2">
          <span class="text-muted-foreground text-xs">{{ t('libraryPricing.enabled') }}</span>
          <Switch v-model="custom.enabled" />
        </div>
      </div>
      <p class="text-muted-foreground text-xs">
        {{ t('libraryPricing.customTierHint') }}
      </p>
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div class="flex flex-col gap-1.5">
          <Label>{{ t('libraryPricing.customMinGiB') }}</Label>
          <div class="flex items-center gap-2">
            <Input v-model.number="custom.min_capacity_gb" type="number" min="0" step="1" class="tabular-nums" />
            <span class="text-muted-foreground text-sm">GiB</span>
          </div>
        </div>
        <div class="flex flex-col gap-1.5">
          <Label>{{ t('libraryPricing.customPerGiB') }}</Label>
          <div class="flex items-center gap-2">
            <span class="text-muted-foreground text-sm">¥</span>
            <Input v-model.number="custom.price_per_gb_month_yuan" type="number" min="0" step="0.01" class="tabular-nums" />
          </div>
        </div>
        <div class="flex flex-col gap-1.5">
          <Label>{{ t('libraryPricing.tierDiscountBps') }}</Label>
          <div class="flex items-center gap-2">
            <Input v-model.number="custom.tier_discount_bps" type="number" min="0" max="10000" step="100" class="tabular-nums" />
            <span class="text-muted-foreground text-xs">{{ fmtDiscountBps(custom.tier_discount_bps) || t('libraryPricing.fullPrice') }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 时长选项 -->
    <div class="space-y-2">
      <div class="flex items-center justify-between">
        <Label>{{ t('libraryPricing.durations') }}</Label>
        <Button v-auth="'library:manage'" variant="outline" size="sm" @click="addDuration">
          <Plus class="size-4" /> {{ t('libraryPricing.addDuration') }}
        </Button>
      </div>
      <p class="text-muted-foreground text-xs">
        {{ t('libraryPricing.durationsHint') }}
      </p>
      <div v-if="durations.length" class="divide-y rounded-md border">
        <div v-for="(d, idx) in durations" :key="idx" class="grid grid-cols-[1fr_1fr_auto_auto] items-center gap-3 px-3 py-2">
          <div class="flex items-center gap-1.5">
            <span class="text-muted-foreground text-xs">{{ t('libraryPricing.days') }}</span>
            <Input v-model.number="d.days" type="number" min="1" step="1" class="h-8 w-24 tabular-nums" />
          </div>
          <div class="flex items-center gap-1.5">
            <span class="text-muted-foreground text-xs">{{ t('libraryPricing.discountBps') }}</span>
            <Input v-model.number="d.discount_bps" type="number" min="0" max="10000" step="100" class="h-8 w-28 tabular-nums" />
          </div>
          <span class="text-muted-foreground text-xs">{{ fmtDiscountBps(d.discount_bps) || t('libraryPricing.fullPrice') }}</span>
          <Button variant="ghost" size="sm" class="text-destructive" @click="removeDuration(idx)">
            <Trash2 class="size-4" />
          </Button>
        </div>
      </div>
      <p v-else class="text-muted-foreground text-xs">
        {{ t('libraryPricing.noDurations') }}
      </p>
    </div>

    <!-- 须知 / 计费说明 -->
    <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <div class="flex flex-col gap-1.5">
        <Label>{{ t('libraryPricing.notice') }}</Label>
        <textarea
          v-model="notice"
          class="border-input bg-background min-h-20 rounded-md border px-3 py-2 text-sm"
          :placeholder="t('libraryPricing.noticePlaceholder')"
        />
      </div>
      <div class="flex flex-col gap-1.5">
        <Label>{{ t('libraryPricing.billingNote') }}</Label>
        <textarea
          v-model="billingNote"
          class="border-input bg-background min-h-20 rounded-md border px-3 py-2 text-sm"
          :placeholder="t('libraryPricing.billingNotePlaceholder')"
        />
      </div>
    </div>
  </div>
</template>
