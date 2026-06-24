<script setup lang="ts">
import type { PaymentMethod } from '@/types/billing'
import { GripVertical } from 'lucide-vue-next'
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import billingApi from '@/api/modules/billing'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)

// 行内编辑态：手续费以「%」与「¥」展示编辑，保存时再换算回 bps/cents。
interface MethodRow extends PaymentMethod {
  // 比例手续费（%）展示值，bps/100（200bps → 2）
  feePercent: number
  // 固定手续费（¥）展示值，cents/100（100cents → 1）
  feeFixed: number
}

const methods = ref<MethodRow[]>([])

function toRow(m: PaymentMethod): MethodRow {
  // 余额方式手续费恒为 0，编辑态固定显示 0（输入框另在模板内 disable）。
  const isBalance = m.code === 'balance'
  return {
    ...m,
    feePercent: isBalance ? 0 : (m.fee_percent_bps ?? 0) / 100,
    feeFixed: isBalance ? 0 : (m.fee_fixed_cents ?? 0) / 100,
  }
}

async function load() {
  loading.value = true
  try {
    const { data } = await billingApi.getPaymentMethods()
    methods.value = [...(data.payment_methods ?? [])].sort((a, b) => a.sort - b.sort).map(toRow)
  }
  catch {
    toast.error(t('billing.loadFail'))
  }
  finally {
    loading.value = false
  }
}
onMounted(load)

// ── 原生 HTML5 拖拽排序（无第三方依赖）。
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
  const arr = methods.value
  const [moved] = arr.splice(from, 1)
  arr.splice(idx, 0, moved)
}
function onDragEnd() {
  dragIndex.value = null
  overIndex.value = null
}

async function save() {
  saving.value = true
  try {
    // 以当前展示顺序重写 sort（0..n-1）；手续费换算回 bps/cents。
    // 余额方式（balance）手续费恒为 0（前端兜底，后端校验为最终保证）。
    const payload: PaymentMethod[] = methods.value.map((m, i) => {
      const isBalance = m.code === 'balance'
      const { feePercent, feeFixed, ...base } = m
      return {
        ...base,
        sort: i,
        fee_percent_bps: isBalance ? 0 : Math.round((feePercent || 0) * 100),
        fee_fixed_cents: isBalance ? 0 : Math.round((feeFixed || 0) * 100),
      }
    })
    await billingApi.savePaymentMethods(payload)
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
</script>

<template>
  <Card>
    <CardContent class="pt-6">
      <div class="max-w-3xl space-y-5">
        <div class="flex items-start justify-between gap-3">
          <div class="space-y-1.5">
            <h2 class="text-lg font-semibold">
              {{ t('billing.payTitle') }}
            </h2>
            <p class="text-muted-foreground text-sm">
              {{ t('billing.payDesc') }}
            </p>
          </div>
          <Button v-auth="'billing:manage'" :disabled="saving || loading" @click="save">
            {{ saving ? t('common.loading') : t('crud.save') }}
          </Button>
        </div>
        <div class="divide-y rounded-md border">
          <div
            v-for="(m, idx) in methods"
            :key="m.code"
            class="grid grid-cols-[auto_auto_1fr_auto] items-center gap-3 px-3 py-2.5 transition-colors"
            :class="[
              dragIndex === idx ? 'opacity-50' : '',
              overIndex === idx && dragIndex !== null && dragIndex !== idx ? 'bg-accent' : '',
            ]"
            draggable="true"
            @dragstart="onDragStart(idx)"
            @dragover.prevent="onDragOver(idx)"
            @drop="onDrop(idx)"
            @dragend="onDragEnd"
          >
            <span class="text-muted-foreground cursor-grab active:cursor-grabbing" :title="t('billing.payDragHint')">
              <GripVertical class="size-4" />
            </span>
            <span class="text-muted-foreground w-6 text-center text-xs tabular-nums">{{ idx + 1 }}</span>
            <div class="flex flex-wrap items-end gap-x-4 gap-y-2">
              <div class="flex flex-col gap-1">
                <span class="font-mono text-xs text-muted-foreground">{{ m.code }}</span>
                <Input v-model="m.name" class="h-8 w-44" :placeholder="t('billing.payName')" />
              </div>
              <div class="flex flex-col gap-1">
                <span class="text-muted-foreground text-xs">{{ t('billing.payFeePercent') }}</span>
                <Input
                  v-model.number="m.feePercent"
                  type="number"
                  min="0"
                  max="100"
                  step="0.01"
                  class="h-8 w-28"
                  :disabled="m.code === 'balance'"
                />
              </div>
              <div class="flex flex-col gap-1">
                <span class="text-muted-foreground text-xs">{{ t('billing.payFeeFixed') }}</span>
                <Input
                  v-model.number="m.feeFixed"
                  type="number"
                  min="0"
                  step="0.01"
                  class="h-8 w-28"
                  :disabled="m.code === 'balance'"
                />
              </div>
            </div>
            <div class="flex items-center gap-2 self-center">
              <span class="text-muted-foreground text-xs">{{ m.enabled ? t('billing.payEnabled') : t('billing.payDisabled') }}</span>
              <Switch v-model="m.enabled" />
            </div>
          </div>
        </div>
        <p v-if="!methods.length && !loading" class="text-muted-foreground py-6 text-center text-sm">
          {{ t('billing.payEmpty') }}
        </p>
      </div>
    </CardContent>
  </Card>
</template>
