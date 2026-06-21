<script setup lang="ts">
import type { NoticesConfig } from '@/types/billing'
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
import { Label } from '@/components/ui/label'

const { t } = useI18n()

const form = reactive<NoticesConfig>({
  seat: { notice: '', billing_note: '' },
  boot_slot: { notice: '', billing_note: '' },
  runtime_pack: { notice: '' },
})
const loading = ref(false)
const saving = ref(false)

async function load() {
  loading.value = true
  try {
    const { data } = await billingApi.getNotices()
    form.seat = { notice: data.seat?.notice ?? '', billing_note: data.seat?.billing_note ?? '' }
    form.boot_slot = { notice: data.boot_slot?.notice ?? '', billing_note: data.boot_slot?.billing_note ?? '' }
    form.runtime_pack = { notice: data.runtime_pack?.notice ?? '' }
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
    await billingApi.saveNotices({
      seat: { ...form.seat },
      boot_slot: { ...form.boot_slot },
      runtime_pack: { ...form.runtime_pack },
    })
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
  <div class="flex flex-col gap-6">
    <div class="flex items-center justify-between">
      <div>
        <h2 class="text-lg font-semibold">{{ t('billing.noticesTitle') }}</h2>
        <p class="text-muted-foreground text-sm">{{ t('billing.noticesDesc') }}</p>
      </div>
      <Button v-auth="'billing:manage'" :disabled="saving || loading" @click="save">
        {{ saving ? t('common.loading') : t('crud.save') }}
      </Button>
    </div>

    <!-- seat -->
    <Card>
      <CardHeader>
        <CardTitle>{{ t('billing.kind_seat') }}</CardTitle>
        <CardDescription>{{ t('billing.noticesKindDesc') }}</CardDescription>
      </CardHeader>
      <CardContent class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div class="flex flex-col gap-1.5">
          <Label>{{ t('billing.fNotice') }}</Label>
          <textarea v-model="form.seat.notice" class="border-input bg-background min-h-24 rounded-md border px-3 py-2 text-sm" />
        </div>
        <div class="flex flex-col gap-1.5">
          <Label>{{ t('billing.fBillingNote') }}</Label>
          <textarea v-model="form.seat.billing_note" class="border-input bg-background min-h-24 rounded-md border px-3 py-2 text-sm" />
        </div>
      </CardContent>
    </Card>

    <!-- boot_slot -->
    <Card>
      <CardHeader>
        <CardTitle>{{ t('billing.kind_boot_slot') }}</CardTitle>
        <CardDescription>{{ t('billing.noticesKindDesc') }}</CardDescription>
      </CardHeader>
      <CardContent class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div class="flex flex-col gap-1.5">
          <Label>{{ t('billing.fNotice') }}</Label>
          <textarea v-model="form.boot_slot.notice" class="border-input bg-background min-h-24 rounded-md border px-3 py-2 text-sm" />
        </div>
        <div class="flex flex-col gap-1.5">
          <Label>{{ t('billing.fBillingNote') }}</Label>
          <textarea v-model="form.boot_slot.billing_note" class="border-input bg-background min-h-24 rounded-md border px-3 py-2 text-sm" />
        </div>
      </CardContent>
    </Card>

    <!-- runtime_pack -->
    <Card>
      <CardHeader>
        <CardTitle>{{ t('billing.kind_runtime_pack') }}</CardTitle>
        <CardDescription>{{ t('billing.noticesRuntimeDesc') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <div class="flex flex-col gap-1.5">
          <Label>{{ t('billing.fNotice') }}</Label>
          <textarea v-model="form.runtime_pack.notice" class="border-input bg-background min-h-24 rounded-md border px-3 py-2 text-sm" />
        </div>
      </CardContent>
    </Card>
  </div>
</template>
