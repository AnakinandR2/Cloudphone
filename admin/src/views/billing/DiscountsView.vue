<script setup lang="ts">
import { Plus, Save, Trash2 } from 'lucide-vue-next'
import { reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
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
import { Switch } from '@/components/ui/switch'

const { t } = useI18n()

// 折扣以「折」表示：8.5 = 8.5 折（= 85%）。原型本地状态。
const cycle = reactive({ quarter: 8.5, year: 7 })
const packages = ref([
  { id: 1, name: '500 小时包', discount: 9 },
  { id: 2, name: '1000 小时包', discount: 8 },
])
const promos = ref([
  { id: 1, code: 'WELCOME15', discount: 8.5, expires: '2026-12-31', enabled: true },
  { id: 2, code: 'SUMMER20', discount: 8, expires: '2026-09-30', enabled: false },
])
let promoSeq = promos.value.length

function addPromo() {
  promos.value.push({ id: ++promoSeq, code: '', discount: 9, expires: '', enabled: true })
}
function removePromo(id: number) {
  promos.value = promos.value.filter(p => p.id !== id)
}
function save() {
  toast.success(t('billing.savedOk'))
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 class="text-xl font-semibold tracking-tight">{{ t('billing.discountsTitle') }}</h1>
        <p class="text-muted-foreground mt-1 text-sm">{{ t('billing.discountsDesc') }}</p>
      </div>
      <Button size="sm" @click="save">
        <Save class="size-4" /> {{ t('billing.save') }}
      </Button>
    </div>

    <!-- 周期折扣 -->
    <Card>
      <CardHeader>
        <CardTitle class="text-base">{{ t('billing.cycleDiscount') }}</CardTitle>
        <CardDescription>{{ t('billing.cycleDiscountDesc') }}</CardDescription>
      </CardHeader>
      <CardContent class="grid gap-4 sm:grid-cols-2">
        <div class="flex flex-col gap-1.5">
          <Label>{{ t('billing.quarterDiscount') }}</Label>
          <div class="flex items-center gap-2">
            <Input v-model.number="cycle.quarter" type="number" min="0" max="10" step="0.1" class="w-28 tabular-nums" />
            <span class="text-muted-foreground text-sm">{{ t('billing.discountUnit') }}</span>
          </div>
        </div>
        <div class="flex flex-col gap-1.5">
          <Label>{{ t('billing.yearDiscount') }}</Label>
          <div class="flex items-center gap-2">
            <Input v-model.number="cycle.year" type="number" min="0" max="10" step="0.1" class="w-28 tabular-nums" />
            <span class="text-muted-foreground text-sm">{{ t('billing.discountUnit') }}</span>
          </div>
        </div>
        <p class="text-muted-foreground sm:col-span-2 text-xs">{{ t('billing.discountHint') }}</p>
      </CardContent>
    </Card>

    <!-- 时长包折扣 -->
    <Card>
      <CardHeader>
        <CardTitle class="text-base">{{ t('billing.packageDiscount') }}</CardTitle>
        <CardDescription>{{ t('billing.packageDiscountDesc') }}</CardDescription>
      </CardHeader>
      <CardContent class="flex flex-col gap-3">
        <div v-for="p in packages" :key="p.id" class="flex items-center gap-3">
          <span class="w-32 text-sm font-medium">{{ p.name }}</span>
          <Input v-model.number="p.discount" type="number" min="0" max="10" step="0.1" class="w-28 tabular-nums" />
          <span class="text-muted-foreground text-sm">{{ t('billing.discountUnit') }}</span>
        </div>
      </CardContent>
    </Card>

    <!-- 优惠码 -->
    <Card>
      <CardHeader class="flex-row items-start justify-between gap-3 space-y-0">
        <div class="space-y-1.5">
          <CardTitle class="text-base">{{ t('billing.promoTitle') }}</CardTitle>
          <CardDescription>{{ t('billing.promoDesc') }}</CardDescription>
        </div>
        <Button size="sm" variant="outline" @click="addPromo">
          <Plus class="size-4" /> {{ t('billing.addPromo') }}
        </Button>
      </CardHeader>
      <CardContent class="flex flex-col gap-3">
        <div class="text-muted-foreground grid grid-cols-[1fr_7rem_10rem_4rem_2.5rem] items-center gap-3 px-1 text-xs">
          <span>{{ t('billing.promoCode') }}</span>
          <span>{{ t('billing.promoDiscount') }}</span>
          <span>{{ t('billing.promoExpires') }}</span>
          <span>{{ t('billing.promoEnabled') }}</span>
          <span />
        </div>
        <div v-for="p in promos" :key="p.id" class="grid grid-cols-[1fr_7rem_10rem_4rem_2.5rem] items-center gap-3">
          <Input v-model="p.code" :placeholder="t('billing.promoCodePlaceholder')" class="font-mono" />
          <div class="flex items-center gap-1">
            <Input v-model.number="p.discount" type="number" min="0" max="10" step="0.1" class="tabular-nums" />
            <span class="text-muted-foreground text-xs">{{ t('billing.discountUnit') }}</span>
          </div>
          <Input v-model="p.expires" type="date" class="tabular-nums" />
          <Switch v-model="p.enabled" />
          <Button variant="ghost" size="icon" class="text-destructive size-8" @click="removePromo(p.id)">
            <Trash2 class="size-4" />
          </Button>
        </div>
        <p v-if="!promos.length" class="text-muted-foreground py-4 text-center text-sm">{{ t('billing.promoEmpty') }}</p>
      </CardContent>
    </Card>
  </div>
</template>
