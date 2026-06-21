<script setup lang="ts">
import type { BillingKind, KindPricing, PricingConfig, RuntimeBillingConfig } from '@/types/billing'
import { Plus, Trash2 } from 'lucide-vue-next'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import billingApi from '@/api/modules/billing'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { fmtDiscountBps } from '@/utils/money'

const { t } = useI18n()

type TabKey = BillingKind | 'runtime' | 'recharge'
const activeTab = ref<TabKey>('seat')

// ── seat / boot_slot 表单态：单价用「元」编辑，存「分」。
interface KindForm {
  unit_price_yuan: number
  unit_label: string
  qty_tiers: { min_quantity: number, discount_bps: number }[]
  duration_unit: 'month' | 'day'
  duration_options: { value: number, discount_bps: number }[]
  notice: string
  billing_note: string
  recycle_retention_days: number // 仅 seat 用：回收站保留天数
}

function emptyForm(): KindForm {
  return {
    unit_price_yuan: 0,
    unit_label: '',
    qty_tiers: [],
    duration_unit: 'month',
    duration_options: [],
    notice: '',
    billing_note: '',
    recycle_retention_days: 30,
  }
}

const forms = reactive<Record<BillingKind, KindForm>>({
  seat: emptyForm(),
  boot_slot: emptyForm(),
})

// ── 临时开机时长表单态。
interface RuntimeForm {
  unit_price_yuan_per_minute: number
  packs: { minutes: number, discount_bps: number }[]
  min_minutes: number
  daily_cap_minutes: number
  gift_minutes_per_seat_month: number
  notice: string
}
const runtimeForm = reactive<RuntimeForm>({
  unit_price_yuan_per_minute: 0,
  packs: [],
  min_minutes: 0,
  daily_cap_minutes: 200,
  gift_minutes_per_seat_month: 200,
  notice: '',
})

// ── 充值预设（以「元」编辑，存「分」）。
const rechargeYuan = ref<number[]>([])

const loading = ref(false)
const saving = ref(false)

function toForm(p: KindPricing): KindForm {
  return {
    unit_price_yuan: p.unit_price_cents / 100,
    unit_label: p.unit_label ?? '',
    qty_tiers: (p.qty_tiers ?? []).map(x => ({ ...x })),
    duration_unit: p.duration_unit,
    duration_options: (p.duration_options ?? []).map(x => ({ ...x })),
    notice: p.notice ?? '',
    billing_note: p.billing_note ?? '',
    recycle_retention_days: p.recycle_retention_days ?? 30,
  }
}

async function load() {
  loading.value = true
  try {
    const [{ data: pricing }, { data: runtime }, { data: recharge }] = await Promise.all([
      billingApi.getPricing(),
      billingApi.getRuntimeBillingConfig(),
      billingApi.getRechargePresets(),
    ])
    forms.seat = toForm(pricing.kinds.seat)
    forms.boot_slot = toForm(pricing.kinds.boot_slot)
    runtimeForm.unit_price_yuan_per_minute = runtime.unit_price_cents_per_minute / 100
    runtimeForm.packs = (runtime.packs ?? []).map(x => ({ ...x }))
    runtimeForm.min_minutes = runtime.min_minutes
    runtimeForm.daily_cap_minutes = runtime.daily_cap_minutes
    runtimeForm.gift_minutes_per_seat_month = runtime.gift_minutes_per_seat_month ?? 0
    runtimeForm.notice = runtime.notice ?? ''
    rechargeYuan.value = (recharge.presets_cents ?? []).map(c => c / 100)
  }
  catch {
    toast.error(t('billing.loadFail'))
  }
  finally {
    loading.value = false
  }
}
onMounted(load)

function buildKind(kind: BillingKind): KindPricing {
  const f = forms[kind]
  return {
    unit_price_cents: Math.round(f.unit_price_yuan * 100),
    unit_label: f.unit_label,
    qty_tiers: f.qty_tiers.map(x => ({ min_quantity: Number(x.min_quantity) || 0, discount_bps: Number(x.discount_bps) || 10000 })),
    duration_unit: f.duration_unit,
    duration_options: f.duration_options.map(x => ({ value: Number(x.value) || 0, discount_bps: Number(x.discount_bps) || 10000 })),
    notice: f.notice,
    billing_note: f.billing_note,
    recycle_retention_days: Number(f.recycle_retention_days) || 0,
  }
}

async function save() {
  saving.value = true
  try {
    if (activeTab.value === 'runtime') {
      const payload: RuntimeBillingConfig = {
        unit_price_cents_per_minute: Math.round(runtimeForm.unit_price_yuan_per_minute * 100),
        packs: runtimeForm.packs.map(p => ({ minutes: Number(p.minutes) || 0, discount_bps: Number(p.discount_bps) || 10000 })),
        min_minutes: Number(runtimeForm.min_minutes) || 0,
        daily_cap_minutes: Number(runtimeForm.daily_cap_minutes) || 0,
        gift_minutes_per_seat_month: Number(runtimeForm.gift_minutes_per_seat_month) || 0,
        notice: runtimeForm.notice,
      }
      await billingApi.saveRuntimeBillingConfig(payload)
    }
    else if (activeTab.value === 'recharge') {
      const cents = rechargeYuan.value
        .map(y => Math.round(Number(y) * 100))
        .filter(c => Number.isFinite(c) && c > 0)
      await billingApi.saveRechargePresets(cents)
    }
    else {
      // seat / boot_slot 整体提交（后端 PUT 覆盖 { kinds }），两者一并写回避免清空另一个。
      const payload: PricingConfig = { kinds: { seat: buildKind('seat'), boot_slot: buildKind('boot_slot') } }
      await billingApi.savePricing(payload)
    }
    toast.success(t('billing.savedOk'))
    load()
  }
  catch {
    toast.error(t('billing.updateFail'))
  }
  finally {
    saving.value = false
  }
}

function addTier(kind: BillingKind) {
  forms[kind].qty_tiers.push({ min_quantity: 1, discount_bps: 10000 })
}
function removeTier(kind: BillingKind, idx: number) {
  forms[kind].qty_tiers.splice(idx, 1)
}
function addDuration(kind: BillingKind) {
  forms[kind].duration_options.push({ value: 1, discount_bps: 10000 })
}
function removeDuration(kind: BillingKind, idx: number) {
  forms[kind].duration_options.splice(idx, 1)
}
function addPack() {
  runtimeForm.packs.push({ minutes: 600, discount_bps: 10000 })
}
function removePack(idx: number) {
  runtimeForm.packs.splice(idx, 1)
}
function addPreset() {
  rechargeYuan.value.push(0)
}
function removePreset(idx: number) {
  rechargeYuan.value.splice(idx, 1)
}

function durationUnitLabel(kind: BillingKind) {
  return forms[kind].duration_unit === 'month' ? t('billing.durMonth') : t('billing.durDay')
}

const saveLabel = computed(() => (saving.value ? t('common.loading') : t('crud.save')))
</script>

<template>
  <Card>
    <CardContent class="pt-6">
      <div class="max-w-4xl space-y-6">
        <div class="flex items-start justify-between gap-3">
          <div class="space-y-1.5">
            <h2 class="text-lg font-semibold">
              {{ t('billing.pricingCfgTitle') }}
            </h2>
            <p class="text-muted-foreground text-sm">
              {{ t('billing.pricingCfgDesc') }}
            </p>
          </div>
          <Button v-auth="'billing:manage'" :disabled="saving || loading" @click="save">
            {{ saveLabel }}
          </Button>
        </div>
        <Tabs v-model="activeTab">
          <TabsList>
            <TabsTrigger value="seat">
              {{ t('billing.kind_seat') }}
            </TabsTrigger>
            <TabsTrigger value="boot_slot">
              {{ t('billing.kind_boot_slot') }}
            </TabsTrigger>
            <TabsTrigger value="runtime">
              {{ t('billing.kind_runtime_pack') }}
            </TabsTrigger>
            <TabsTrigger value="recharge">
              {{ t('billing.rechargeTitle') }}
            </TabsTrigger>
          </TabsList>

          <!-- seat / boot_slot tab：复用 KindPricing 表单 -->
          <TabsContent v-for="kind in (['seat', 'boot_slot'] as BillingKind[])" :key="kind" :value="kind" class="space-y-6 pt-4">
            <p class="text-muted-foreground text-sm">
              {{ t(`billing.kindDesc_${kind}`) }}
            </p>

            <!-- 基础单价 + 单位 -->
            <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div class="flex flex-col gap-1.5">
                <Label>{{ t('billing.unitPrice') }}</Label>
                <div class="flex items-center gap-2">
                  <span class="text-muted-foreground text-sm">¥</span>
                  <Input v-model.number="forms[kind].unit_price_yuan" type="number" min="0" step="0.01" class="tabular-nums" />
                </div>
              </div>
              <div class="flex flex-col gap-1.5">
                <Label>{{ t('billing.unitLabel') }}</Label>
                <Input v-model="forms[kind].unit_label" :placeholder="t('billing.unitLabelPlaceholder')" />
              </div>
              <!-- 回收站保留天数：仅席位有意义（席位过期实例进回收站后的保留天数） -->
              <div v-if="kind === 'seat'" class="flex flex-col gap-1.5">
                <Label>{{ t('billing.recycleDays') }}</Label>
                <Input v-model.number="forms[kind].recycle_retention_days" type="number" min="0" step="1" />
                <p class="text-muted-foreground text-xs">
                  {{ t('billing.recycleDaysHint') }}
                </p>
              </div>
            </div>

            <!-- 数量阶梯（数量档位的唯一来源） -->
            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <Label>{{ t('billing.qtyTiers') }}</Label>
                <Button v-auth="'billing:manage'" variant="outline" size="sm" @click="addTier(kind)">
                  <Plus class="size-4" /> {{ t('billing.addTierRow') }}
                </Button>
              </div>
              <p class="text-muted-foreground text-xs">
                {{ t('billing.qtyTiersHint') }}
              </p>
              <div v-if="forms[kind].qty_tiers.length" class="divide-y rounded-md border">
                <div v-for="(tier, idx) in forms[kind].qty_tiers" :key="idx" class="grid grid-cols-[1fr_1fr_auto_auto] items-center gap-3 px-3 py-2">
                  <div class="flex items-center gap-1.5">
                    <span class="text-muted-foreground text-xs">{{ t('billing.minQty') }}</span>
                    <Input v-model.number="tier.min_quantity" type="number" min="1" step="1" class="h-8 w-24" />
                  </div>
                  <div class="flex items-center gap-1.5">
                    <span class="text-muted-foreground text-xs">{{ t('billing.discountBps') }}</span>
                    <Input v-model.number="tier.discount_bps" type="number" min="0" max="10000" step="100" class="h-8 w-28" />
                  </div>
                  <span class="text-muted-foreground text-xs">{{ fmtDiscountBps(tier.discount_bps) || t('billing.fullPrice') }}</span>
                  <Button variant="ghost" size="sm" class="text-destructive" @click="removeTier(kind, idx)">
                    <Trash2 class="size-4" />
                  </Button>
                </div>
              </div>
              <p v-else class="text-muted-foreground text-xs">
                {{ t('billing.noTiers') }}
              </p>
            </div>

            <!-- 时长选项 -->
            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <Label>{{ t('billing.durationOptions') }}（{{ durationUnitLabel(kind) }}）</Label>
                <Button v-auth="'billing:manage'" variant="outline" size="sm" @click="addDuration(kind)">
                  <Plus class="size-4" /> {{ t('billing.addDurationRow') }}
                </Button>
              </div>
              <div v-if="forms[kind].duration_options.length" class="divide-y rounded-md border">
                <div v-for="(d, idx) in forms[kind].duration_options" :key="idx" class="grid grid-cols-[1fr_1fr_auto_auto] items-center gap-3 px-3 py-2">
                  <div class="flex items-center gap-1.5">
                    <span class="text-muted-foreground text-xs">{{ durationUnitLabel(kind) }}</span>
                    <Input v-model.number="d.value" type="number" min="1" step="1" class="h-8 w-24" />
                  </div>
                  <div class="flex items-center gap-1.5">
                    <span class="text-muted-foreground text-xs">{{ t('billing.discountBps') }}</span>
                    <Input v-model.number="d.discount_bps" type="number" min="0" max="10000" step="100" class="h-8 w-28" />
                  </div>
                  <span class="text-muted-foreground text-xs">{{ fmtDiscountBps(d.discount_bps) || t('billing.fullPrice') }}</span>
                  <Button variant="ghost" size="sm" class="text-destructive" @click="removeDuration(kind, idx)">
                    <Trash2 class="size-4" />
                  </Button>
                </div>
              </div>
              <p v-else class="text-muted-foreground text-xs">
                {{ t('billing.noDurations') }}
              </p>
            </div>

            <!-- 须知 / 计费说明 -->
            <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div class="flex flex-col gap-1.5">
                <Label>{{ t('billing.fNotice') }}</Label>
                <textarea
                  v-model="forms[kind].notice"
                  class="border-input bg-background min-h-20 rounded-md border px-3 py-2 text-sm"
                  :placeholder="t('billing.fNoticePlaceholder')"
                />
              </div>
              <div class="flex flex-col gap-1.5">
                <Label>{{ t('billing.fBillingNote') }}</Label>
                <textarea
                  v-model="forms[kind].billing_note"
                  class="border-input bg-background min-h-20 rounded-md border px-3 py-2 text-sm"
                  :placeholder="t('billing.fBillingNotePlaceholder')"
                />
              </div>
            </div>
          </TabsContent>

          <!-- runtime tab：临时开机时长 -->
          <TabsContent value="runtime" class="space-y-6 pt-4">
            <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
              <div class="flex flex-col gap-1.5">
                <Label>{{ t('billing.rtPerMinute') }}</Label>
                <div class="flex items-center gap-2">
                  <span class="text-muted-foreground text-sm">¥</span>
                  <Input v-model.number="runtimeForm.unit_price_yuan_per_minute" type="number" min="0" step="0.01" class="tabular-nums" />
                </div>
              </div>
              <div class="flex flex-col gap-1.5">
                <Label>{{ t('billing.rtMinMinutes') }}</Label>
                <Input v-model.number="runtimeForm.min_minutes" type="number" min="0" step="1" />
                <p class="text-muted-foreground text-xs">
                  {{ t('billing.rtMinMinutesHint') }}
                </p>
              </div>
              <div class="flex flex-col gap-1.5">
                <Label>{{ t('billing.rtDailyCap') }}</Label>
                <Input v-model.number="runtimeForm.daily_cap_minutes" type="number" min="0" step="1" />
                <p class="text-muted-foreground text-xs">
                  {{ t('billing.rtDailyCapHint') }}
                </p>
              </div>
              <div class="flex flex-col gap-1.5">
                <Label>{{ t('billing.rtGiftPerSeatMonth') }}</Label>
                <Input v-model.number="runtimeForm.gift_minutes_per_seat_month" type="number" min="0" step="1" />
                <p class="text-muted-foreground text-xs">
                  {{ t('billing.rtGiftPerSeatMonthHint') }}
                </p>
              </div>
            </div>

            <!-- 时长包预设 -->
            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <Label>{{ t('billing.rtPacksTitle') }}</Label>
                <Button v-auth="'billing:manage'" variant="outline" size="sm" @click="addPack">
                  <Plus class="size-4" /> {{ t('billing.addPackRow') }}
                </Button>
              </div>
              <p class="text-muted-foreground text-xs">
                {{ t('billing.rtPacksDesc') }}
              </p>
              <div v-if="runtimeForm.packs.length" class="divide-y rounded-md border">
                <div v-for="(p, idx) in runtimeForm.packs" :key="idx" class="grid grid-cols-[1fr_1fr_auto_auto] items-center gap-3 px-3 py-2">
                  <div class="flex items-center gap-1.5">
                    <span class="text-muted-foreground text-xs">{{ t('billing.packMinutes') }}</span>
                    <Input v-model.number="p.minutes" type="number" min="1" step="1" class="h-8 w-28" />
                  </div>
                  <div class="flex items-center gap-1.5">
                    <span class="text-muted-foreground text-xs">{{ t('billing.discountBps') }}</span>
                    <Input v-model.number="p.discount_bps" type="number" min="0" max="10000" step="100" class="h-8 w-28" />
                  </div>
                  <span class="text-muted-foreground text-xs">{{ fmtDiscountBps(p.discount_bps) || t('billing.fullPrice') }}</span>
                  <Button variant="ghost" size="sm" class="text-destructive" @click="removePack(idx)">
                    <Trash2 class="size-4" />
                  </Button>
                </div>
              </div>
              <p v-else class="text-muted-foreground text-xs">
                {{ t('billing.noPacks') }}
              </p>
            </div>

            <!-- 须知 -->
            <div class="flex flex-col gap-1.5">
              <Label>{{ t('billing.fNotice') }}</Label>
              <textarea
                v-model="runtimeForm.notice"
                class="border-input bg-background min-h-20 w-full rounded-md border px-3 py-2 text-sm"
                :placeholder="t('billing.fNoticePlaceholder')"
              />
            </div>
          </TabsContent>

          <!-- recharge tab：充值预设 -->
          <TabsContent value="recharge" class="space-y-4 pt-4">
            <div class="flex items-center justify-between">
              <p class="text-muted-foreground text-sm">
                {{ t('billing.rechargeDesc') }}
              </p>
              <Button v-auth="'billing:manage'" variant="outline" size="sm" @click="addPreset">
                <Plus class="size-4" /> {{ t('billing.addPreset') }}
              </Button>
            </div>
            <div v-if="rechargeYuan.length" class="flex flex-wrap gap-3">
              <div v-for="(_, idx) in rechargeYuan" :key="idx" class="flex items-center gap-1.5 rounded-md border px-3 py-2">
                <span class="text-muted-foreground text-sm">¥</span>
                <Input v-model.number="rechargeYuan[idx]" type="number" min="0" step="0.01" class="h-8 w-28 tabular-nums" />
                <Button variant="ghost" size="sm" class="text-destructive" @click="removePreset(idx)">
                  <Trash2 class="size-4" />
                </Button>
              </div>
            </div>
            <p v-else class="text-muted-foreground py-6 text-center text-sm">
              {{ t('billing.noPresets') }}
            </p>
          </TabsContent>
        </Tabs>
      </div>
    </CardContent>
  </Card>
</template>
