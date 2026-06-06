<script setup lang="ts">
import { Check, Clock, Minus, Plus, ShoppingCart, Smartphone, X } from 'lucide-vue-next'
import { computed, ref } from 'vue'
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'

const { t } = useI18n()

// 两种布局放在 tab 里对比：standard=双栏卡片，cashier=左 SKU / 右收银台。
const tab = ref<'standard' | 'cashier'>('standard')

// 金额统一两位小数。
function money(n: number) {
  return n.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}
// 折扣展示：中文「8.5折」，英文「15% off」。
function discountText(d: number) {
  return t('billing.discount', { zhe: +(d * 10).toFixed(1), off: Math.round((1 - d) * 100) })
}

// —— 概览（原型静态数据）——
const overview = {
  activeInstances: 12,
  purchasedInstances: 20,
  remainingHours: 860,
}

// —— 购买实例数 ——
const MONTH_PRICE = 30
const cycles = [
  { key: 'month', months: 1, discount: 1 },
  { key: 'quarter', months: 3, discount: 0.85 },
  { key: 'year', months: 12, discount: 0.7 },
] as const
const cycleKey = ref<(typeof cycles)[number]['key']>('month')
const qty = ref(5)
function cyclePrice(c: (typeof cycles)[number]) {
  return MONTH_PRICE * c.months * c.discount
}
const instancesSubtotal = computed(() => {
  const c = cycles.find(x => x.key === cycleKey.value)!
  return qty.value * cyclePrice(c)
})
function stepQty(d: number) {
  qty.value = Math.min(100, Math.max(1, qty.value + d))
}

// —— 购买开机时长 ——
const PER_HOUR = 0.2
const packages = [
  { hours: 100, discount: 1 },
  { hours: 500, discount: 0.9 },
  { hours: 1000, discount: 0.8 },
] as const
const pkgKey = ref<number | 'custom'>(500)
const customHours = ref(2000)
function pkgPrice(p: (typeof packages)[number]) {
  return p.hours * PER_HOUR * p.discount
}
const customInvalid = computed(() => pkgKey.value === 'custom' && (!Number.isFinite(customHours.value) || customHours.value < 1))
const hoursSubtotal = computed(() => {
  if (pkgKey.value === 'custom')
    return Math.max(0, customHours.value || 0) * PER_HOUR
  return pkgPrice(packages.find(p => p.hours === pkgKey.value)!)
})

// 摘要文案
const cycleLabel = computed(() => t(`billing.cycle_${cycleKey.value}`))
const hoursLabel = computed(() => (pkgKey.value === 'custom' ? (customHours.value || 0) : pkgKey.value))

// —— 收银台版：实例 / 时长各自「加入订单」，默认都不在订单里，
//    可单独购买，也可一起结算（避免给人「必须一起买」的错觉）。——
const addedInstances = ref(false)
const addedHours = ref(false)
const cartEmpty = computed(() => !addedInstances.value && !addedHours.value)
const total = computed(() =>
  (addedInstances.value ? instancesSubtotal.value : 0)
  + (addedHours.value && !customInvalid.value ? hoursSubtotal.value : 0))
const checkoutDisabled = computed(() =>
  cartEmpty.value || (addedHours.value && customInvalid.value) || total.value <= 0)

// —— 支付方式 ——
const payMethod = ref<'wechat' | 'alipay'>('wechat')
const methods = [
  { key: 'wechat', mark: '微', color: '#07C160' },
  { key: 'alipay', mark: '支', color: '#1677FF' },
] as const
// 标准版两类 SKU 各自独立选择支付方式；收银台版用合并的 payMethod。
const payMethodInstance = ref<'wechat' | 'alipay'>('wechat')
const payMethodHours = ref<'wechat' | 'alipay'>('wechat')
const currentMethod = computed(() => {
  const k = payKind.value === 'instance' ? payMethodInstance.value : payMethodHours.value
  return methods.find(m => m.key === k)!
})

// 标准版：单项弹框支付。
const payOpen = ref(false)
const payKind = ref<'instance' | 'hours'>('instance')
const payAmount = computed(() => (payKind.value === 'instance' ? instancesSubtotal.value : hoursSubtotal.value))
const paySummary = computed(() => (payKind.value === 'instance'
  ? t('billing.paySummaryInstance', { qty: qty.value, cycle: cycleLabel.value })
  : t('billing.paySummaryHours', { hours: hoursLabel.value })))
function openPay(kind: 'instance' | 'hours') {
  payKind.value = kind
  payOpen.value = true
}
function confirmPay() {
  payOpen.value = false
  toast.info(t('billing.comingSoon'))
}
// 收银台版：合并结算。
function checkout() {
  toast.info(t('billing.comingSoon'))
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <!-- 标题 + 说明 -->
    <div>
      <h1 class="text-xl font-semibold tracking-tight">{{ t('billing.purchaseTitle') }}</h1>
      <p class="text-muted-foreground mt-1 text-sm">{{ t('billing.purchaseDesc') }}</p>
    </div>

    <!-- 概览（两个 tab 共用） -->
    <div class="grid gap-4 sm:grid-cols-2">
      <Card>
        <CardContent class="flex items-center gap-3 py-5">
          <div class="flex size-10 items-center justify-center rounded-lg bg-blue-500/10 text-blue-600 dark:text-blue-400">
            <Smartphone class="size-5" />
          </div>
          <div>
            <div class="text-muted-foreground text-xs">{{ t('billing.instances') }}</div>
            <div class="text-xl font-semibold tabular-nums">
              {{ overview.activeInstances }} <span class="text-muted-foreground text-base font-normal">/ {{ overview.purchasedInstances }}</span>
            </div>
            <div class="text-muted-foreground text-[11px]">{{ t('billing.activeOverPurchased') }}</div>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="flex items-center gap-3 py-5">
          <div class="flex size-10 items-center justify-center rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400">
            <Clock class="size-5" />
          </div>
          <div>
            <div class="text-muted-foreground text-xs">{{ t('billing.remainingHours') }}</div>
            <div class="text-xl font-semibold tabular-nums">{{ overview.remainingHours }} {{ t('billing.hoursUnit') }}</div>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- 布局切换 -->
    <div class="bg-muted inline-flex w-fit rounded-lg p-1 text-sm">
      <button
        type="button"
        class="cursor-pointer rounded-md px-3 py-1.5 font-medium transition-colors"
        :class="tab === 'standard' ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground'"
        @click="tab = 'standard'"
      >
        {{ t('billing.tabStandard') }}
      </button>
      <button
        type="button"
        class="cursor-pointer rounded-md px-3 py-1.5 font-medium transition-colors"
        :class="tab === 'cashier' ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground'"
        @click="tab = 'cashier'"
      >
        {{ t('billing.tabCashier') }}
      </button>
    </div>

    <!-- ============ 标准版：两类 SKU 各自独立卡片（含支付方式 + 立即购买）============ -->
    <div v-if="tab === 'standard'" class="grid gap-6 lg:grid-cols-2">
      <!-- 实例 -->
      <Card class="flex flex-col">
        <CardHeader>
          <CardTitle class="flex items-center gap-2">
            <Smartphone class="size-5" /> {{ t('billing.buyInstances') }}
          </CardTitle>
          <CardDescription>{{ t('billing.buyInstancesDesc') }}</CardDescription>
        </CardHeader>
        <CardContent class="flex flex-1 flex-col gap-5">
          <div class="flex flex-col gap-2">
            <span class="text-muted-foreground text-sm">{{ t('billing.cycle') }}</span>
            <div class="grid grid-cols-3 gap-2">
              <button
                v-for="c in cycles"
                :key="c.key"
                type="button"
                class="relative cursor-pointer rounded-lg border px-3 py-2.5 text-center transition-colors"
                :class="cycleKey === c.key ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
                @click="cycleKey = c.key"
              >
                <span v-if="c.discount < 1" class="absolute -top-2 -right-1.5 rounded-full bg-red-500 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-white shadow-sm">{{ discountText(c.discount) }}</span>
                <div class="text-sm font-medium">{{ t(`billing.cycle_${c.key}`) }}</div>
                <div class="text-muted-foreground text-xs tabular-nums">¥{{ money(cyclePrice(c)) }}</div>
              </button>
            </div>
          </div>
          <div class="flex flex-col gap-2">
            <span class="text-muted-foreground text-sm">{{ t('billing.quantity') }}</span>
            <div class="flex items-center gap-2">
              <Button variant="outline" size="icon" :disabled="qty <= 1" @click="stepQty(-1)"><Minus class="size-4" /></Button>
              <Input v-model.number="qty" type="number" min="1" max="100" class="h-9 w-20 text-center tabular-nums" />
              <Button variant="outline" size="icon" :disabled="qty >= 100" @click="stepQty(1)"><Plus class="size-4" /></Button>
              <span class="text-muted-foreground text-sm">{{ t('billing.unit') }}</span>
            </div>
          </div>
          <Separator class="mt-auto" />
          <div class="flex items-center justify-between">
            <div class="text-muted-foreground text-sm">{{ t('billing.subtotal') }}</div>
            <div class="text-2xl font-semibold tabular-nums">¥{{ money(instancesSubtotal) }}</div>
          </div>
          <!-- 该 SKU 自己的支付方式 -->
          <div class="flex flex-col gap-2">
            <span class="text-muted-foreground text-xs">{{ t('billing.payMethod') }}</span>
            <div class="grid grid-cols-2 gap-2">
              <button
                v-for="m in methods"
                :key="m.key"
                type="button"
                class="relative flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 transition-colors"
                :class="payMethodInstance === m.key ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
                @click="payMethodInstance = m.key"
              >
                <span class="flex size-7 items-center justify-center rounded-md text-sm font-bold text-white" :style="{ backgroundColor: m.color }">{{ m.mark }}</span>
                <span class="text-sm font-medium">{{ t(`billing.pay_${m.key}`) }}</span>
                <Check v-if="payMethodInstance === m.key" class="text-primary absolute top-1.5 right-1.5 size-3.5" />
              </button>
            </div>
          </div>
          <Button class="w-full" @click="openPay('instance')">{{ t('billing.buyNow') }}</Button>
        </CardContent>
      </Card>

      <!-- 时长 -->
      <Card class="flex flex-col">
        <CardHeader>
          <CardTitle class="flex items-center gap-2">
            <Clock class="size-5" /> {{ t('billing.buyHours') }}
          </CardTitle>
          <CardDescription>{{ t('billing.buyHoursDesc') }}</CardDescription>
        </CardHeader>
        <CardContent class="flex flex-1 flex-col gap-5">
          <div class="flex flex-col gap-2">
            <span class="text-muted-foreground text-sm">{{ t('billing.package') }}</span>
            <div class="grid grid-cols-2 gap-2">
              <button
                v-for="p in packages"
                :key="p.hours"
                type="button"
                class="relative cursor-pointer rounded-lg border px-3 py-3 text-center transition-colors"
                :class="pkgKey === p.hours ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
                @click="pkgKey = p.hours"
              >
                <span v-if="p.discount < 1" class="absolute -top-2 -right-1.5 rounded-full bg-red-500 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-white shadow-sm">{{ discountText(p.discount) }}</span>
                <div class="text-sm font-medium tabular-nums">{{ p.hours }} {{ t('billing.hoursUnit') }}</div>
                <div class="text-muted-foreground text-xs tabular-nums">¥{{ money(pkgPrice(p)) }}</div>
              </button>
              <button
                type="button"
                class="cursor-pointer rounded-lg border px-3 py-3 text-center transition-colors"
                :class="pkgKey === 'custom' ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
                @click="pkgKey = 'custom'"
              >
                <div class="text-sm font-medium">{{ t('billing.custom') }}</div>
                <div class="text-muted-foreground text-xs tabular-nums">¥{{ money(PER_HOUR) }} / {{ t('billing.hoursUnit') }}</div>
              </button>
            </div>
          </div>
          <div v-if="pkgKey === 'custom'" class="flex flex-col gap-1.5">
            <span class="text-muted-foreground text-sm">{{ t('billing.customHours') }}</span>
            <div class="flex items-center gap-2">
              <Input v-model.number="customHours" type="number" min="1" step="100" class="h-9 w-32 tabular-nums" :aria-invalid="customInvalid" :class="customInvalid ? 'border-red-500 focus-visible:ring-red-500/30' : ''" />
              <span class="text-muted-foreground text-sm">{{ t('billing.hoursUnit') }}</span>
            </div>
            <p :class="customInvalid ? 'text-red-500' : 'text-muted-foreground'" class="text-xs">{{ t('billing.customHoursHint') }}</p>
          </div>
          <Separator class="mt-auto" />
          <div class="flex items-center justify-between">
            <div class="text-muted-foreground text-sm">{{ t('billing.subtotal') }}</div>
            <div class="text-2xl font-semibold tabular-nums">¥{{ money(hoursSubtotal) }}</div>
          </div>
          <!-- 该 SKU 自己的支付方式 -->
          <div class="flex flex-col gap-2">
            <span class="text-muted-foreground text-xs">{{ t('billing.payMethod') }}</span>
            <div class="grid grid-cols-2 gap-2">
              <button
                v-for="m in methods"
                :key="m.key"
                type="button"
                class="relative flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 transition-colors"
                :class="payMethodHours === m.key ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
                @click="payMethodHours = m.key"
              >
                <span class="flex size-7 items-center justify-center rounded-md text-sm font-bold text-white" :style="{ backgroundColor: m.color }">{{ m.mark }}</span>
                <span class="text-sm font-medium">{{ t(`billing.pay_${m.key}`) }}</span>
                <Check v-if="payMethodHours === m.key" class="text-primary absolute top-1.5 right-1.5 size-3.5" />
              </button>
            </div>
          </div>
          <Button class="w-full" :disabled="customInvalid" @click="openPay('hours')">{{ t('billing.buyNow') }}</Button>
        </CardContent>
      </Card>
    </div>

    <!-- ============ 收银台版：左 SKU / 右收银台 ============ -->
    <div v-else class="grid gap-6 lg:grid-cols-3">
      <!-- 左：信息与 SKU 选择 -->
      <div class="flex flex-col gap-6 lg:col-span-2">
        <Card>
          <CardHeader>
            <CardTitle class="flex items-center gap-2 text-base">
              <Smartphone class="size-5" /> {{ t('billing.buyInstances') }}
            </CardTitle>
            <CardDescription>{{ t('billing.buyInstancesDesc') }}</CardDescription>
          </CardHeader>
          <CardContent class="flex flex-col gap-5">
            <div class="flex flex-col gap-2">
              <span class="text-muted-foreground text-sm">{{ t('billing.cycle') }}</span>
              <div class="grid grid-cols-3 gap-2">
                <button
                  v-for="c in cycles"
                  :key="c.key"
                  type="button"
                  class="relative cursor-pointer rounded-lg border px-3 py-2.5 text-center transition-colors"
                  :class="cycleKey === c.key ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
                  @click="cycleKey = c.key"
                >
                  <span v-if="c.discount < 1" class="absolute -top-2 -right-1.5 rounded-full bg-red-500 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-white shadow-sm">{{ discountText(c.discount) }}</span>
                  <div class="text-sm font-medium">{{ t(`billing.cycle_${c.key}`) }}</div>
                  <div class="text-muted-foreground text-xs tabular-nums">¥{{ money(cyclePrice(c)) }}</div>
                </button>
              </div>
            </div>
            <div class="flex flex-col gap-2">
              <span class="text-muted-foreground text-sm">{{ t('billing.quantity') }}</span>
              <div class="flex items-center gap-2">
                <Button variant="outline" size="icon" :disabled="qty <= 1" @click="stepQty(-1)"><Minus class="size-4" /></Button>
                <Input v-model.number="qty" type="number" min="1" max="100" class="h-9 w-20 text-center tabular-nums" />
                <Button variant="outline" size="icon" :disabled="qty >= 100" @click="stepQty(1)"><Plus class="size-4" /></Button>
                <span class="text-muted-foreground text-sm">{{ t('billing.unit') }}</span>
              </div>
            </div>
            <Separator />
            <div class="flex items-center justify-between">
              <div class="tabular-nums">
                <span class="text-muted-foreground text-sm">{{ t('billing.subtotal') }}</span>
                <span class="ml-1.5 font-semibold">¥{{ money(instancesSubtotal) }}</span>
              </div>
              <Button :variant="addedInstances ? 'secondary' : 'default'" size="sm" @click="addedInstances = !addedInstances">
                <component :is="addedInstances ? Check : Plus" class="size-4" />
                {{ addedInstances ? t('billing.added') : t('billing.addToOrder') }}
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle class="flex items-center gap-2 text-base">
              <Clock class="size-5" /> {{ t('billing.buyHours') }}
            </CardTitle>
            <CardDescription>{{ t('billing.buyHoursDesc') }}</CardDescription>
          </CardHeader>
          <CardContent class="flex flex-col gap-5">
            <div class="flex flex-col gap-2">
              <span class="text-muted-foreground text-sm">{{ t('billing.package') }}</span>
              <div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
                <button
                  v-for="p in packages"
                  :key="p.hours"
                  type="button"
                  class="relative cursor-pointer rounded-lg border px-3 py-3 text-center transition-colors"
                  :class="pkgKey === p.hours ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
                  @click="pkgKey = p.hours"
                >
                  <span v-if="p.discount < 1" class="absolute -top-2 -right-1.5 rounded-full bg-red-500 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-white shadow-sm">{{ discountText(p.discount) }}</span>
                  <div class="text-sm font-medium tabular-nums">{{ p.hours }} {{ t('billing.hoursUnit') }}</div>
                  <div class="text-muted-foreground text-xs tabular-nums">¥{{ money(pkgPrice(p)) }}</div>
                </button>
                <button
                  type="button"
                  class="cursor-pointer rounded-lg border px-3 py-3 text-center transition-colors"
                  :class="pkgKey === 'custom' ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
                  @click="pkgKey = 'custom'"
                >
                  <div class="text-sm font-medium">{{ t('billing.custom') }}</div>
                  <div class="text-muted-foreground text-xs tabular-nums">¥{{ money(PER_HOUR) }} / {{ t('billing.hoursUnit') }}</div>
                </button>
              </div>
            </div>
            <div v-if="pkgKey === 'custom'" class="flex flex-col gap-1.5">
              <span class="text-muted-foreground text-sm">{{ t('billing.customHours') }}</span>
              <div class="flex items-center gap-2">
                <Input v-model.number="customHours" type="number" min="1" step="100" class="h-9 w-32 tabular-nums" :aria-invalid="customInvalid" :class="customInvalid ? 'border-red-500 focus-visible:ring-red-500/30' : ''" />
                <span class="text-muted-foreground text-sm">{{ t('billing.hoursUnit') }}</span>
              </div>
              <p :class="customInvalid ? 'text-red-500' : 'text-muted-foreground'" class="text-xs">{{ t('billing.customHoursHint') }}</p>
            </div>
            <Separator />
            <div class="flex items-center justify-between">
              <div class="tabular-nums">
                <span class="text-muted-foreground text-sm">{{ t('billing.subtotal') }}</span>
                <span class="ml-1.5 font-semibold">¥{{ money(hoursSubtotal) }}</span>
              </div>
              <Button :variant="addedHours ? 'secondary' : 'default'" size="sm" :disabled="customInvalid" @click="addedHours = !addedHours">
                <component :is="addedHours ? Check : Plus" class="size-4" />
                {{ addedHours ? t('billing.added') : t('billing.addToOrder') }}
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>

      <!-- 右：订单摘要 + 收银台 -->
      <div class="lg:col-span-1">
        <Card class="lg:sticky lg:top-4">
          <CardHeader>
            <CardTitle class="text-base">{{ t('billing.cartTitle') }}</CardTitle>
            <CardDescription>{{ t('billing.cashierNote') }}</CardDescription>
          </CardHeader>
          <CardContent class="flex flex-col gap-4">
            <!-- 空购物车：明确「可单选」 -->
            <div v-if="cartEmpty" class="flex flex-col items-center gap-2 py-8 text-center">
              <ShoppingCart class="text-muted-foreground/50 size-8" />
              <p class="text-muted-foreground text-sm">{{ t('billing.emptyCart') }}</p>
              <p class="text-muted-foreground/80 text-xs">{{ t('billing.emptyCartHint') }}</p>
            </div>

            <template v-else>
              <!-- 已加入的实例 -->
              <div v-if="addedInstances" class="flex items-start gap-2">
                <div class="min-w-0 flex-1">
                  <div class="text-sm font-medium">{{ t('billing.lineInstances') }}</div>
                  <div class="text-muted-foreground text-xs tabular-nums">{{ qty }} {{ t('billing.unit') }} · {{ cycleLabel }}</div>
                </div>
                <div class="text-sm font-medium tabular-nums">¥{{ money(instancesSubtotal) }}</div>
                <Button variant="ghost" size="icon" class="text-muted-foreground hover:text-destructive size-6" @click="addedInstances = false"><X class="size-3.5" /></Button>
              </div>
              <!-- 已加入的时长 -->
              <div v-if="addedHours" class="flex items-start gap-2">
                <div class="min-w-0 flex-1">
                  <div class="text-sm font-medium">{{ t('billing.lineHours') }}</div>
                  <div class="text-muted-foreground text-xs tabular-nums">{{ hoursLabel }} {{ t('billing.hoursUnit') }}</div>
                  <p v-if="customInvalid" class="text-xs text-red-500">{{ t('billing.customHoursHint') }}</p>
                </div>
                <div class="text-sm font-medium tabular-nums" :class="customInvalid ? 'text-muted-foreground line-through' : ''">¥{{ money(hoursSubtotal) }}</div>
                <Button variant="ghost" size="icon" class="text-muted-foreground hover:text-destructive size-6" @click="addedHours = false"><X class="size-3.5" /></Button>
              </div>

              <Separator />
              <div class="flex items-baseline justify-between">
                <span class="text-sm font-medium">{{ t('billing.total') }}</span>
                <span class="text-2xl font-semibold tabular-nums text-red-600">¥{{ money(total) }}</span>
              </div>

              <!-- 支付方式 -->
              <div class="flex flex-col gap-2">
                <span class="text-muted-foreground text-sm">{{ t('billing.payMethod') }}</span>
                <button
                  v-for="m in methods"
                  :key="m.key"
                  type="button"
                  class="flex cursor-pointer items-center gap-3 rounded-lg border px-3 py-2 text-left transition-colors"
                  :class="payMethod === m.key ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
                  @click="payMethod = m.key"
                >
                  <span class="flex size-7 items-center justify-center rounded-md text-sm font-bold text-white" :style="{ backgroundColor: m.color }">{{ m.mark }}</span>
                  <span class="flex-1 text-sm font-medium">{{ t(`billing.pay_${m.key}`) }}</span>
                  <span class="flex size-4 items-center justify-center rounded-full border" :class="payMethod === m.key ? 'border-primary bg-primary text-primary-foreground' : 'border-muted-foreground/40'">
                    <svg v-if="payMethod === m.key" viewBox="0 0 24 24" class="size-3" fill="none" stroke="currentColor" stroke-width="3"><path d="M20 6 9 17l-5-5" /></svg>
                  </span>
                </button>
              </div>

              <Button class="w-full" size="lg" :disabled="checkoutDisabled" @click="checkout">{{ t('billing.checkout') }}</Button>
            </template>
          </CardContent>
        </Card>
      </div>
    </div>

    <!-- 标准版的单项支付弹框 -->
    <Dialog v-model:open="payOpen">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{{ t('billing.payTitle') }}</DialogTitle>
          <DialogDescription>{{ paySummary }}</DialogDescription>
        </DialogHeader>
        <div class="bg-muted/40 flex items-baseline justify-between rounded-lg border px-4 py-3">
          <span class="text-muted-foreground text-sm">{{ t('billing.payable') }}</span>
          <span class="text-2xl font-semibold tabular-nums text-red-600">¥{{ money(payAmount) }}</span>
        </div>
        <!-- 支付方式已在外面选择，这里只读回显 -->
        <div class="flex items-center justify-between rounded-lg border px-3 py-2.5">
          <span class="text-muted-foreground text-sm">{{ t('billing.payMethod') }}</span>
          <span class="flex items-center gap-2">
            <span class="flex size-6 items-center justify-center rounded text-xs font-bold text-white" :style="{ backgroundColor: currentMethod.color }">{{ currentMethod.mark }}</span>
            <span class="text-sm font-medium">{{ t(`billing.pay_${currentMethod.key}`) }}</span>
          </span>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="payOpen = false">{{ t('crud.cancel') }}</Button>
          <Button @click="confirmPay">{{ t('billing.payNow') }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
