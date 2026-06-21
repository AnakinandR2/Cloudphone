<script setup lang="ts">
import type { BillingKind, KindPricing, PricingConfig } from '@/types/billing'
import { Plus, Trash2 } from 'lucide-vue-next'
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import billingApi from '@/api/modules/billing'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { fmtDiscountBps } from '@/utils/money'

const { t } = useI18n()

const KINDS: BillingKind[] = ['seat', 'boot_slot']

// 表单态：单价用「元」编辑，存「分」。
interface KindForm {
  unit_price_yuan: number
  unit_label: string
  qty_options_str: string
  qty_tiers: { min_quantity: number, discount_bps: number }[]
  duration_unit: 'month' | 'day'
  duration_options: { value: number, discount_bps: number }[]
  notice: string
  billing_note: string
}

function emptyForm(): KindForm {
  return {
    unit_price_yuan: 0,
    unit_label: '',
    qty_options_str: '',
    qty_tiers: [],
    duration_unit: 'month',
    duration_options: [],
    notice: '',
    billing_note: '',
  }
}

const forms = reactive<Record<BillingKind, KindForm>>({
  seat: emptyForm(),
  boot_slot: emptyForm(),
})
const loading = ref(false)
const saving = ref(false)

function toForm(p: KindPricing): KindForm {
  return {
    unit_price_yuan: p.unit_price_cents / 100,
    unit_label: p.unit_label ?? '',
    qty_options_str: (p.qty_options ?? []).join(', '),
    qty_tiers: (p.qty_tiers ?? []).map(x => ({ ...x })),
    duration_unit: p.duration_unit,
    duration_options: (p.duration_options ?? []).map(x => ({ ...x })),
    notice: p.notice ?? '',
    billing_note: p.billing_note ?? '',
  }
}

async function load() {
  loading.value = true
  try {
    const { data } = await billingApi.getPricing()
    forms.seat = toForm(data.seat)
    forms.boot_slot = toForm(data.boot_slot)
  }
  catch {
    toast.error(t('billing.loadFail'))
  }
  finally {
    loading.value = false
  }
}
onMounted(load)

function parseQtyOptions(s: string): number[] {
  return s
    .split(/[,，\s]+/)
    .map(x => Number(x.trim()))
    .filter(n => Number.isFinite(n) && n > 0)
}

function buildKind(kind: BillingKind): KindPricing {
  const f = forms[kind]
  return {
    kind,
    unit_price_cents: Math.round(f.unit_price_yuan * 100),
    unit_label: f.unit_label,
    qty_options: parseQtyOptions(f.qty_options_str),
    qty_tiers: f.qty_tiers.map(x => ({ min_quantity: Number(x.min_quantity) || 0, discount_bps: Number(x.discount_bps) || 10000 })),
    duration_unit: f.duration_unit,
    duration_options: f.duration_options.map(x => ({ value: Number(x.value) || 0, discount_bps: Number(x.discount_bps) || 10000 })),
    notice: f.notice,
    billing_note: f.billing_note,
  }
}

async function save() {
  saving.value = true
  try {
    const payload: PricingConfig = { seat: buildKind('seat'), boot_slot: buildKind('boot_slot') }
    await billingApi.savePricing(payload)
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

function kindTitle(kind: BillingKind) {
  return t(`billing.kind_${kind}`)
}
function durationUnitLabel(kind: BillingKind) {
  return forms[kind].duration_unit === 'month' ? t('billing.durMonth') : t('billing.durDay')
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <div class="flex items-center justify-between">
      <div>
        <h2 class="text-lg font-semibold">{{ t('billing.pricingCfgTitle') }}</h2>
        <p class="text-muted-foreground text-sm">{{ t('billing.pricingCfgDesc') }}</p>
      </div>
      <Button v-auth="'billing:manage'" :disabled="saving || loading" @click="save">
        {{ saving ? t('common.loading') : t('crud.save') }}
      </Button>
    </div>

    <Card v-for="kind in KINDS" :key="kind">
      <CardHeader>
        <CardTitle>{{ kindTitle(kind) }}</CardTitle>
        <CardDescription>{{ t(`billing.kindDesc_${kind}`) }}</CardDescription>
      </CardHeader>
      <CardContent class="space-y-5">
        <!-- 基础单价 + 单位 -->
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
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
          <div class="flex flex-col gap-1.5">
            <Label>{{ t('billing.qtyOptions') }}</Label>
            <Input v-model="forms[kind].qty_options_str" :placeholder="t('billing.qtyOptionsPlaceholder')" />
            <p class="text-muted-foreground text-xs">{{ t('billing.qtyOptionsHint') }}</p>
          </div>
        </div>

        <!-- 数量阶梯 -->
        <div class="space-y-2">
          <div class="flex items-center justify-between">
            <Label>{{ t('billing.qtyTiers') }}</Label>
            <Button v-auth="'billing:manage'" variant="outline" size="sm" @click="addTier(kind)">
              <Plus class="size-4" /> {{ t('billing.addTierRow') }}
            </Button>
          </div>
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
          <p v-else class="text-muted-foreground text-xs">{{ t('billing.noTiers') }}</p>
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
          <p v-else class="text-muted-foreground text-xs">{{ t('billing.noDurations') }}</p>
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
      </CardContent>
    </Card>
  </div>
</template>
