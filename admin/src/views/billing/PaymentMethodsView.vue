<script setup lang="ts">
import type { PaymentMethod } from '@/types/billing'
import { ArrowDown, ArrowUp } from 'lucide-vue-next'
import { onMounted, ref } from 'vue'
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

function move(idx: number, dir: -1 | 1) {
  const j = idx + dir
  if (j < 0 || j >= methods.value.length) return
  const arr = methods.value
  ;[arr[idx], arr[j]] = [arr[j], arr[idx]]
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
    <CardHeader class="flex-row items-start justify-between gap-3 space-y-0">
      <div class="space-y-1.5">
        <CardTitle>{{ t('billing.payTitle') }}</CardTitle>
        <CardDescription>{{ t('billing.payDesc') }}</CardDescription>
      </div>
      <Button v-auth="'billing:manage'" :disabled="saving || loading" @click="save">
        {{ saving ? t('common.loading') : t('crud.save') }}
      </Button>
    </CardHeader>
    <CardContent>
      <div class="divide-y rounded-md border">
        <div
          v-for="(m, idx) in methods"
          :key="m.code"
          class="grid grid-cols-[auto_1fr_auto_auto] items-center gap-3 px-3 py-2.5"
        >
          <span class="text-muted-foreground w-6 text-center text-xs tabular-nums">{{ idx + 1 }}</span>
          <div class="flex flex-col gap-1">
            <span class="font-mono text-xs text-muted-foreground">{{ m.code }}</span>
            <Input v-model="m.name" class="h-8 max-w-xs" :placeholder="t('billing.payName')" />
          </div>
          <div class="flex items-center gap-1">
            <Button variant="ghost" size="sm" :disabled="idx === 0" @click="move(idx, -1)">
              <ArrowUp class="size-4" />
            </Button>
            <Button variant="ghost" size="sm" :disabled="idx === methods.length - 1" @click="move(idx, 1)">
              <ArrowDown class="size-4" />
            </Button>
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
    </CardContent>
  </Card>
</template>
