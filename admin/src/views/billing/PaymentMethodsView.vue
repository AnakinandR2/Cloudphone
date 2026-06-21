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

const methods = ref<PaymentMethod[]>([])
const loading = ref(false)
const saving = ref(false)

async function load() {
  loading.value = true
  try {
    const { data } = await billingApi.getPaymentMethods()
    methods.value = [...(data.payment_methods ?? [])].sort((a, b) => a.sort - b.sort)
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
    // 以当前展示顺序重写 sort（0..n-1）
    const payload = methods.value.map((m, i) => ({ ...m, sort: i }))
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
            <div class="flex flex-col gap-1">
              <span class="font-mono text-xs text-muted-foreground">{{ m.code }}</span>
              <Input v-model="m.name" class="h-8 max-w-xs" :placeholder="t('billing.payName')" />
            </div>
            <div class="flex items-center gap-2">
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
