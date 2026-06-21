<script setup lang="ts">
import type { RuntimeBillingConfig } from '@/types/billing'
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

interface Form {
  unit_price_yuan_per_minute: number
  packs: { minutes: number, discount_bps: number }[]
  min_minutes: number
  daily_cap_minutes: number
  recycle_retention_days: number
}

const form = reactive<Form>({
  unit_price_yuan_per_minute: 0,
  packs: [],
  min_minutes: 0,
  daily_cap_minutes: 200,
  recycle_retention_days: 30,
})
const loading = ref(false)
const saving = ref(false)

async function load() {
  loading.value = true
  try {
    const { data } = await billingApi.getRuntimeBillingConfig()
    form.unit_price_yuan_per_minute = data.unit_price_cents_per_minute / 100
    form.packs = (data.packs ?? []).map(x => ({ ...x }))
    form.min_minutes = data.min_minutes
    form.daily_cap_minutes = data.daily_cap_minutes
    form.recycle_retention_days = data.recycle_retention_days
  }
  catch {
    toast.error(t('billing.loadFail'))
  }
  finally {
    loading.value = false
  }
}
onMounted(load)

async function save() {
  saving.value = true
  try {
    const payload: RuntimeBillingConfig = {
      unit_price_cents_per_minute: Math.round(form.unit_price_yuan_per_minute * 100),
      packs: form.packs.map(p => ({ minutes: Number(p.minutes) || 0, discount_bps: Number(p.discount_bps) || 10000 })),
      min_minutes: Number(form.min_minutes) || 0,
      daily_cap_minutes: Number(form.daily_cap_minutes) || 0,
      recycle_retention_days: Number(form.recycle_retention_days) || 0,
    }
    await billingApi.saveRuntimeBillingConfig(payload)
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

function addPack() {
  form.packs.push({ minutes: 600, discount_bps: 10000 })
}
function removePack(idx: number) {
  form.packs.splice(idx, 1)
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <div class="flex items-center justify-between">
      <div>
        <h2 class="text-lg font-semibold">{{ t('billing.rtCfgTitle') }}</h2>
        <p class="text-muted-foreground text-sm">{{ t('billing.rtCfgDesc') }}</p>
      </div>
      <Button v-auth="'billing:manage'" :disabled="saving || loading" @click="save">
        {{ saving ? t('common.loading') : t('crud.save') }}
      </Button>
    </div>

    <Card>
      <CardHeader>
        <CardTitle>{{ t('billing.rtBasicTitle') }}</CardTitle>
        <CardDescription>{{ t('billing.rtBasicDesc') }}</CardDescription>
      </CardHeader>
      <CardContent class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div class="flex flex-col gap-1.5">
          <Label>{{ t('billing.rtPerMinute') }}</Label>
          <div class="flex items-center gap-2">
            <span class="text-muted-foreground text-sm">¥</span>
            <Input v-model.number="form.unit_price_yuan_per_minute" type="number" min="0" step="0.01" class="tabular-nums" />
          </div>
        </div>
        <div class="flex flex-col gap-1.5">
          <Label>{{ t('billing.rtMinMinutes') }}</Label>
          <Input v-model.number="form.min_minutes" type="number" min="0" step="1" />
          <p class="text-muted-foreground text-xs">{{ t('billing.rtMinMinutesHint') }}</p>
        </div>
        <div class="flex flex-col gap-1.5">
          <Label>{{ t('billing.rtDailyCap') }}</Label>
          <Input v-model.number="form.daily_cap_minutes" type="number" min="0" step="1" />
          <p class="text-muted-foreground text-xs">{{ t('billing.rtDailyCapHint') }}</p>
        </div>
        <div class="flex flex-col gap-1.5">
          <Label>{{ t('billing.rtRecycleDays') }}</Label>
          <Input v-model.number="form.recycle_retention_days" type="number" min="0" step="1" />
          <p class="text-muted-foreground text-xs">{{ t('billing.rtRecycleDaysHint') }}</p>
        </div>
      </CardContent>
    </Card>

    <Card>
      <CardHeader class="flex-row items-start justify-between gap-3 space-y-0">
        <div class="space-y-1.5">
          <CardTitle>{{ t('billing.rtPacksTitle') }}</CardTitle>
          <CardDescription>{{ t('billing.rtPacksDesc') }}</CardDescription>
        </div>
        <Button v-auth="'billing:manage'" variant="outline" size="sm" @click="addPack">
          <Plus class="size-4" /> {{ t('billing.addPackRow') }}
        </Button>
      </CardHeader>
      <CardContent>
        <div v-if="form.packs.length" class="divide-y rounded-md border">
          <div v-for="(p, idx) in form.packs" :key="idx" class="grid grid-cols-[1fr_1fr_auto_auto] items-center gap-3 px-3 py-2">
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
        <p v-else class="text-muted-foreground text-xs">{{ t('billing.noPacks') }}</p>
      </CardContent>
    </Card>
  </div>
</template>
