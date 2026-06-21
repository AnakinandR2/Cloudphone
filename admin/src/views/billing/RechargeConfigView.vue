<script setup lang="ts">
import { Plus, Trash2 } from 'lucide-vue-next'
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

const { t } = useI18n()

// 以「元」编辑，存「分」。
const presetsYuan = ref<number[]>([])
const loading = ref(false)
const saving = ref(false)

async function load() {
  loading.value = true
  try {
    const { data } = await billingApi.getRechargePresets()
    presetsYuan.value = (data.presets_cents ?? []).map(c => c / 100)
  }
  catch {
    toast.error(t('billing.loadFail'))
  }
  finally {
    loading.value = false
  }
}
onMounted(load)

function addPreset() {
  presetsYuan.value.push(0)
}
function removePreset(idx: number) {
  presetsYuan.value.splice(idx, 1)
}

async function save() {
  const cents = presetsYuan.value
    .map(y => Math.round(Number(y) * 100))
    .filter(c => Number.isFinite(c) && c > 0)
  saving.value = true
  try {
    await billingApi.saveRechargePresets(cents)
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
        <CardTitle>{{ t('billing.rechargeTitle') }}</CardTitle>
        <CardDescription>{{ t('billing.rechargeDesc') }}</CardDescription>
      </div>
      <div class="flex items-center gap-2">
        <Button v-auth="'billing:manage'" variant="outline" size="sm" @click="addPreset">
          <Plus class="size-4" /> {{ t('billing.addPreset') }}
        </Button>
        <Button v-auth="'billing:manage'" :disabled="saving || loading" size="sm" @click="save">
          {{ saving ? t('common.loading') : t('crud.save') }}
        </Button>
      </div>
    </CardHeader>
    <CardContent>
      <div v-if="presetsYuan.length" class="flex flex-wrap gap-3">
        <div v-for="(_, idx) in presetsYuan" :key="idx" class="flex items-center gap-1.5 rounded-md border px-3 py-2">
          <span class="text-muted-foreground text-sm">¥</span>
          <Input v-model.number="presetsYuan[idx]" type="number" min="0" step="0.01" class="h-8 w-28 tabular-nums" />
          <Button variant="ghost" size="sm" class="text-destructive" @click="removePreset(idx)">
            <Trash2 class="size-4" />
          </Button>
        </div>
      </div>
      <p v-else class="text-muted-foreground py-6 text-center text-sm">{{ t('billing.noPresets') }}</p>
    </CardContent>
  </Card>
</template>
