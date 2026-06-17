<script setup lang="ts">
import type { BillingAccount, DiscountTier, EntitlementsResult, Sku, SkuWithTiers } from '@/types/billing'
import { Check, Clock, Minus, Plus, ShoppingCart, Smartphone, X } from 'lucide-vue-next'
import { computed, onMounted, ref } from 'vue'
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
import { Skeleton } from '@/components/ui/skeleton'
import { fmtCents, fmtDiscountBps } from '@/utils/money'

const { t } = useI18n()

// ——— 布局切换 ———
const tab = ref<'standard' | 'cashier'>('standard')

// ——— 加载状态 ———
const loading = ref(false)
const submitting = ref(false)

// ——— API 数据 ———
const accountData = ref<BillingAccount | null>(null)
const entitlements = ref<EntitlementsResult | null>(null)
const skuList = ref<SkuWithTiers[]>([])

// ——— SKU 查找助手 ———
function findSku(category: string): SkuWithTiers | undefined {
  return skuList.value.find(s => s.sku.category === category)
}

// ——— 折扣阶梯查找 ———
// 对于订阅型 SKU：按 cycle_months + quantity 匹配，取最大 min_quantity。
// 对于时长包 (cycle_months=0)：按 quantity 匹配。
function pickTier(tiers: DiscountTier[], cycleMonths: number, quantity: number): DiscountTier | null {
  const matching = tiers.filter(t => t.cycle_months === cycleMonths && t.min_quantity <= quantity)
  if (!matching.length) return null
  return matching.reduce((best, t) => t.min_quantity > best.min_quantity ? t : best)
}

// ——— 本地计价（前端即时展示；下单时后端权威）———
function calcSubtotal(sku: Sku, tiers: DiscountTier[], cycleMonths: number, quantity: number): number {
  const tier = pickTier(tiers, cycleMonths, quantity)
  const bps = tier?.discount_bps ?? 10000
  // billing_units: subscription = cycle_months × quantity; time_pack = quantity(hours)
  const billingUnits = sku.category === 'time_pack' ? quantity : cycleMonths * quantity
  const originalCents = sku.unit_price_cents * billingUnits
  return Math.round(originalCents * bps / 10000)
}

function calcDiscount(tiers: DiscountTier[], cycleMonths: number, quantity: number): number {
  const tier = pickTier(tiers, cycleMonths, quantity)
  return tier?.discount_bps ?? 10000
}

// ——— 实例 / 开机包 SKU（按台月订阅）———
const CYCLES = [
  { key: 'month', months: 1 },
  { key: 'quarter', months: 3 },
  { key: 'year', months: 12 },
] as const

type CycleKey = (typeof CYCLES)[number]['key']
const cycleKey = ref<CycleKey>('month')
const qty = ref(5)

function stepQty(d: number) {
  qty.value = Math.min(100, Math.max(1, qty.value + d))
}

// 开机包数量独立
const bootQty = ref(2)
function stepBootQty(d: number) {
  bootQty.value = Math.min(100, Math.max(1, bootQty.value + d))
}

const instanceSku = computed(() => findSku('instance_fee'))
const bootSku = computed(() => findSku('boot_pack'))

const instanceSubtotalCents = computed(() => {
  const s = instanceSku.value
  if (!s) return 0
  const c = CYCLES.find(x => x.key === cycleKey.value)!
  return calcSubtotal(s.sku, s.tiers, c.months, qty.value)
})

const bootSubtotalCents = computed(() => {
  const s = bootSku.value
  if (!s) return 0
  const c = CYCLES.find(x => x.key === cycleKey.value)!
  return calcSubtotal(s.sku, s.tiers, c.months, bootQty.value)
})

const instanceDiscountBps = computed(() => {
  const s = instanceSku.value
  if (!s) return 10000
  const c = CYCLES.find(x => x.key === cycleKey.value)!
  return calcDiscount(s.tiers, c.months, qty.value)
})

const bootDiscountBps = computed(() => {
  const s = bootSku.value
  if (!s) return 10000
  const c = CYCLES.find(x => x.key === cycleKey.value)!
  return calcDiscount(s.tiers, c.months, bootQty.value)
})

// 单台月价格（用于展示每个周期按钮里的价格）
function instanceCyclePriceCents(months: number): number {
  const s = instanceSku.value
  if (!s) return 0
  return calcSubtotal(s.sku, s.tiers, months, 1)
}

function bootCyclePriceCents(months: number): number {
  const s = bootSku.value
  if (!s) return 0
  return calcSubtotal(s.sku, s.tiers, months, 1)
}

function cycleBps(skuWithTiers: SkuWithTiers | undefined, months: number): number {
  if (!skuWithTiers) return 10000
  return calcDiscount(skuWithTiers.tiers, months, 1)
}

// ——— 时长包 SKU ———
const timeSku = computed(() => findSku('time_pack'))

// 从 time_pack tiers 构建套餐档位（取 min_quantity > 1 的，代表有意义的批量档位，以及 min_quantity=1 的最小起购）
const timePackages = computed<Array<{ hours: number; discount_bps: number; subtotalCents: number }>>(() => {
  const s = timeSku.value
  if (!s) return []
  // 以 tiers 的 min_quantity 作为推荐档位（cycle_months=0）
  const t0tiers = s.tiers.filter(t => t.cycle_months === 0).sort((a, b) => a.min_quantity - b.min_quantity)
  return t0tiers.map(tier => ({
    hours: tier.min_quantity,
    discount_bps: tier.discount_bps,
    subtotalCents: calcSubtotal(s.sku, s.tiers, 0, tier.min_quantity),
  }))
})

const pkgHours = ref<number | 'custom'>(0) // 0 = unset; will be set after skus load
const customHours = ref(2000)
const customInvalid = computed(() => pkgHours.value === 'custom' && (!Number.isFinite(customHours.value) || customHours.value < 1))

const timeSubtotalCents = computed(() => {
  const s = timeSku.value
  if (!s) return 0
  if (pkgHours.value === 'custom') {
    const hrs = Math.max(0, customHours.value || 0)
    return calcSubtotal(s.sku, s.tiers, 0, hrs)
  }
  return calcSubtotal(s.sku, s.tiers, 0, pkgHours.value as number)
})

const timeDiscountBps = computed(() => {
  const s = timeSku.value
  if (!s) return 10000
  if (pkgHours.value === 'custom') {
    return calcDiscount(s.tiers, 0, Math.max(0, customHours.value || 0))
  }
  return calcDiscount(s.tiers, 0, pkgHours.value as number)
})

const timeUnitPriceCents = computed(() => timeSku.value?.sku.unit_price_cents ?? 0)

// ——— 收银台：加入订单 ———
const addedInstances = ref(false)
const addedBoot = ref(false)
const addedTime = ref(false)

const cartEmpty = computed(() => !addedInstances.value && !addedBoot.value && !addedTime.value)

const cartTotalCents = computed(() => {
  let total = 0
  if (addedInstances.value) total += instanceSubtotalCents.value
  if (addedBoot.value) total += bootSubtotalCents.value
  if (addedTime.value && !customInvalid.value) total += timeSubtotalCents.value
  return total
})

const checkoutDisabled = computed(() =>
  cartEmpty.value
  || (addedTime.value && customInvalid.value)
  || cartTotalCents.value <= 0
  || submitting.value)

// ——— 支付方式 ———
type PayMethod = 'balance' | 'wechat' | 'alipay'
const payMethod = ref<PayMethod>('balance')
const payMethodInstance = ref<PayMethod>('balance')
const payMethodBoot = ref<PayMethod>('balance')
const payMethodTime = ref<PayMethod>('balance')

const methods = [
  { key: 'balance' as const, mark: '余', color: '#6366f1' },
  { key: 'wechat' as const, mark: '微', color: '#07C160' },
  { key: 'alipay' as const, mark: '支', color: '#1677FF' },
] as const

// ——— 标准版单项支付弹框 ———
const payOpen = ref(false)
type PayKind = 'instance' | 'boot' | 'time'
const payKind = ref<PayKind>('instance')

const payAmountCents = computed(() => {
  if (payKind.value === 'instance') return instanceSubtotalCents.value
  if (payKind.value === 'boot') return bootSubtotalCents.value
  return timeSubtotalCents.value
})

const paySummary = computed(() => {
  if (payKind.value === 'instance')
    return t('billing.paySummaryInstance', { qty: qty.value, cycle: t(`billing.cycle_${cycleKey.value}`) })
  if (payKind.value === 'boot')
    return t('billing.paySummaryBoot', { qty: bootQty.value, cycle: t(`billing.cycle_${cycleKey.value}`) })
  return t('billing.paySummaryHours', { hours: pkgHours.value === 'custom' ? customHours.value : pkgHours.value })
})

const currentPayMethod = computed<PayMethod>(() => {
  if (payKind.value === 'instance') return payMethodInstance.value
  if (payKind.value === 'boot') return payMethodBoot.value
  return payMethodTime.value
})

const currentMethod = computed(() => methods.find(m => m.key === currentPayMethod.value)!)

function openPay(kind: PayKind) {
  payKind.value = kind
  payOpen.value = true
}

// ——— 下单 + 支付 ———
async function doCreateAndPay(
  items: Array<{ sku_code: string; cycle_months: number; quantity: number }>,
  method: PayMethod,
): Promise<boolean> {
  submitting.value = true
  try {
    const createRes = await billingApi.createOrder({ items, pay_method: method })
    // 所有支付方式点支付后均即时到账：余额扣款、微信/支付宝走即时到账桩。
    await billingApi.payOrder(createRes.data.order.id)
    toast.success(t('billing.orderPaidOk'))
    await loadOverview()
    return true
  }
  catch {
    // 失败提示（余额不足等）由 axios 响应拦截器统一弹出，这里不重复 toast。
    return false
  }
  finally {
    submitting.value = false
  }
}

async function confirmPay() {
  payOpen.value = false
  const c = CYCLES.find(x => x.key === cycleKey.value)!
  if (payKind.value === 'instance') {
    const s = instanceSku.value
    if (!s) return
    await doCreateAndPay([{ sku_code: s.sku.code, cycle_months: c.months, quantity: qty.value }], currentPayMethod.value)
  }
  else if (payKind.value === 'boot') {
    const s = bootSku.value
    if (!s) return
    await doCreateAndPay([{ sku_code: s.sku.code, cycle_months: c.months, quantity: bootQty.value }], currentPayMethod.value)
  }
  else {
    const s = timeSku.value
    if (!s) return
    const hours = pkgHours.value === 'custom' ? (customHours.value || 0) : (pkgHours.value as number)
    await doCreateAndPay([{ sku_code: s.sku.code, cycle_months: 0, quantity: hours }], currentPayMethod.value)
  }
}

async function checkout() {
  const c = CYCLES.find(x => x.key === cycleKey.value)!
  const items: Array<{ sku_code: string; cycle_months: number; quantity: number }> = []
  if (addedInstances.value) {
    const s = instanceSku.value
    if (s) items.push({ sku_code: s.sku.code, cycle_months: c.months, quantity: qty.value })
  }
  if (addedBoot.value) {
    const s = bootSku.value
    if (s) items.push({ sku_code: s.sku.code, cycle_months: c.months, quantity: bootQty.value })
  }
  if (addedTime.value && !customInvalid.value) {
    const s = timeSku.value
    if (s) {
      const hours = pkgHours.value === 'custom' ? (customHours.value || 0) : (pkgHours.value as number)
      items.push({ sku_code: s.sku.code, cycle_months: 0, quantity: hours })
    }
  }
  if (!items.length) return
  const ok = await doCreateAndPay(items, payMethod.value)
  if (ok) {
    addedInstances.value = false
    addedBoot.value = false
    addedTime.value = false
  }
}

// ——— 加载概览（账户 + 权益）———
async function loadOverview() {
  try {
    const [accRes, entRes] = await Promise.all([
      billingApi.account(),
      billingApi.entitlements(),
    ])
    if (accRes.code === 0) accountData.value = accRes.data
    if (entRes.code === 0) entitlements.value = entRes.data
  }
  catch {
    // silent — overview is non-critical
  }
}

// ——— 初始加载 ———
onMounted(async () => {
  loading.value = true
  try {
    const [skuRes, accRes, entRes] = await Promise.all([
      billingApi.skus(),
      billingApi.account(),
      billingApi.entitlements(),
    ])
    if (skuRes.code === 0) {
      skuList.value = skuRes.data
      // 默认选中 time_pack 的第一个档位（最小 min_quantity）
      const tp = skuRes.data.find(s => s.sku.category === 'time_pack')
      if (tp) {
        const t0 = tp.tiers.filter(t => t.cycle_months === 0).sort((a, b) => a.min_quantity - b.min_quantity)
        if (t0.length) pkgHours.value = t0[0].min_quantity
      }
    }
    else {
      toast.error(skuRes.message || t('billing.loadSkuFailed'))
    }
    if (accRes.code === 0) accountData.value = accRes.data
    if (entRes.code === 0) entitlements.value = entRes.data
  }
  catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e)
    toast.error(msg || t('billing.loadFailed'))
  }
  finally {
    loading.value = false
  }
})

// ——— 概览数值 ———
const balanceCents = computed(() => accountData.value?.balance_cents ?? 0)
const instanceSeatCap = computed(() => entitlements.value?.capacities.instance_seat ?? 0)
const bootSeatCap = computed(() => entitlements.value?.capacities.boot_seat ?? 0)
const runtimeMinutes = computed(() => entitlements.value?.capacities.runtime_minute ?? 0)

// 时长显示为分钟数（原始值），单独在模板里 label
const runtimeHours = computed(() => Math.floor(runtimeMinutes.value / 60))
const runtimeMinRem = computed(() => runtimeMinutes.value % 60)

// 当前选中时长包的小时数（用于摘要显示）
const selectedHours = computed(() => pkgHours.value === 'custom' ? (customHours.value || 0) : (pkgHours.value as number))
</script>

<template>
  <div class="flex flex-col gap-6">
    <!-- 标题 -->
    <div>
      <h1 class="text-xl font-semibold tracking-tight">{{ t('billing.purchaseTitle') }}</h1>
      <p class="text-muted-foreground mt-1 text-sm">{{ t('billing.purchaseDesc') }}</p>
    </div>

    <!-- 账户概览 -->
    <div v-if="loading" class="grid gap-4 sm:grid-cols-4">
      <Skeleton v-for="i in 4" :key="i" class="h-20 rounded-xl" />
    </div>
    <div v-else class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <!-- 余额 -->
      <Card>
        <CardContent class="flex items-center gap-3 py-4">
          <div class="flex size-10 items-center justify-center rounded-lg bg-indigo-500/10 text-indigo-600 dark:text-indigo-400">
            <span class="text-base font-bold">¥</span>
          </div>
          <div>
            <div class="text-muted-foreground text-xs">{{ t('billing.overviewBalance') }}</div>
            <div class="text-xl font-semibold tabular-nums">¥{{ fmtCents(balanceCents) }}</div>
          </div>
        </CardContent>
      </Card>
      <!-- 实例席位 -->
      <Card>
        <CardContent class="flex items-center gap-3 py-4">
          <div class="flex size-10 items-center justify-center rounded-lg bg-blue-500/10 text-blue-600 dark:text-blue-400">
            <Smartphone class="size-5" />
          </div>
          <div>
            <div class="text-muted-foreground text-xs">{{ t('billing.overviewInstanceSeat') }}</div>
            <div class="text-xl font-semibold tabular-nums">{{ instanceSeatCap }}</div>
            <div class="text-muted-foreground text-[11px]">{{ t('billing.overviewSeatUnit') }}</div>
          </div>
        </CardContent>
      </Card>
      <!-- 开机席位 -->
      <Card>
        <CardContent class="flex items-center gap-3 py-4">
          <div class="flex size-10 items-center justify-center rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400">
            <Smartphone class="size-5" />
          </div>
          <div>
            <div class="text-muted-foreground text-xs">{{ t('billing.overviewBootSeat') }}</div>
            <div class="text-xl font-semibold tabular-nums">{{ bootSeatCap }}</div>
            <div class="text-muted-foreground text-[11px]">{{ t('billing.overviewSeatUnit') }}</div>
          </div>
        </CardContent>
      </Card>
      <!-- 时长余额（分钟→小时+分钟） -->
      <Card>
        <CardContent class="flex items-center gap-3 py-4">
          <div class="flex size-10 items-center justify-center rounded-lg bg-amber-500/10 text-amber-600 dark:text-amber-400">
            <Clock class="size-5" />
          </div>
          <div>
            <div class="text-muted-foreground text-xs">{{ t('billing.overviewRuntime') }}</div>
            <div class="text-xl font-semibold tabular-nums">
              {{ runtimeHours }}<span class="text-base font-normal">h</span>
              <span v-if="runtimeMinRem" class="ml-0.5">{{ runtimeMinRem }}<span class="text-base font-normal">m</span></span>
            </div>
            <div class="text-muted-foreground text-[11px]">{{ runtimeMinutes }} {{ t('billing.overviewMinuteUnit') }}</div>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- SKU 未加载 →  不渲染购买区 -->
    <div v-if="loading" class="grid gap-6 lg:grid-cols-2">
      <Skeleton v-for="i in 3" :key="i" class="h-64 rounded-xl" />
    </div>

    <template v-else-if="skuList.length">
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

      <!-- ========= 标准版 ========= -->
      <div v-if="tab === 'standard'" class="grid gap-6 lg:grid-cols-2 xl:grid-cols-3">

        <!-- 实例费 -->
        <Card v-if="instanceSku" class="flex flex-col">
          <CardHeader>
            <CardTitle class="flex items-center gap-2">
              <Smartphone class="size-5" /> {{ instanceSku.sku.name }}
            </CardTitle>
            <CardDescription>{{ instanceSku.sku.description }}</CardDescription>
          </CardHeader>
          <CardContent class="flex flex-1 flex-col gap-5">
            <!-- 周期 -->
            <div class="flex flex-col gap-2">
              <span class="text-muted-foreground text-sm">{{ t('billing.cycle') }}</span>
              <div class="grid grid-cols-3 gap-2">
                <button
                  v-for="c in CYCLES"
                  :key="c.key"
                  type="button"
                  class="relative cursor-pointer rounded-lg border px-3 py-2.5 text-center transition-colors"
                  :class="cycleKey === c.key ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
                  @click="cycleKey = c.key"
                >
                  <span v-if="fmtDiscountBps(cycleBps(instanceSku, c.months))" class="absolute -top-2 -right-1.5 rounded-full bg-red-500 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-white shadow-sm">{{ fmtDiscountBps(cycleBps(instanceSku, c.months)) }}</span>
                  <div class="text-sm font-medium">{{ t(`billing.cycle_${c.key}`) }}</div>
                  <div class="text-muted-foreground text-xs tabular-nums">¥{{ fmtCents(instanceCyclePriceCents(c.months)) }}</div>
                </button>
              </div>
            </div>
            <!-- 数量 -->
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
              <div class="text-muted-foreground text-sm">
                {{ t('billing.subtotal') }}
                <span v-if="fmtDiscountBps(instanceDiscountBps)" class="ml-1 text-red-500 text-xs">{{ fmtDiscountBps(instanceDiscountBps) }}</span>
              </div>
              <div class="text-2xl font-semibold tabular-nums">¥{{ fmtCents(instanceSubtotalCents) }}</div>
            </div>
            <!-- 支付方式 -->
            <div class="flex flex-col gap-2">
              <span class="text-muted-foreground text-xs">{{ t('billing.payMethod') }}</span>
              <div class="grid grid-cols-3 gap-2">
                <button
                  v-for="m in methods"
                  :key="m.key"
                  type="button"
                  class="relative flex cursor-pointer items-center gap-1.5 rounded-lg border px-2 py-2 transition-colors"
                  :class="payMethodInstance === m.key ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
                  @click="payMethodInstance = m.key"
                >
                  <span class="flex size-6 items-center justify-center rounded text-xs font-bold text-white" :style="{ backgroundColor: m.color }">{{ m.mark }}</span>
                  <span class="text-xs font-medium">{{ t(`billing.pay_${m.key}`) }}</span>
                  <Check v-if="payMethodInstance === m.key" class="text-primary absolute top-1 right-1 size-3" />
                </button>
              </div>
            </div>
            <Button class="w-full" :disabled="submitting" @click="openPay('instance')">{{ t('billing.buyNow') }}</Button>
          </CardContent>
        </Card>

        <!-- 开机包 -->
        <Card v-if="bootSku" class="flex flex-col">
          <CardHeader>
            <CardTitle class="flex items-center gap-2">
              <Smartphone class="size-5" /> {{ bootSku.sku.name }}
            </CardTitle>
            <CardDescription>{{ bootSku.sku.description }}</CardDescription>
          </CardHeader>
          <CardContent class="flex flex-1 flex-col gap-5">
            <!-- 周期（共用 cycleKey） -->
            <div class="flex flex-col gap-2">
              <span class="text-muted-foreground text-sm">{{ t('billing.cycle') }}</span>
              <div class="grid grid-cols-3 gap-2">
                <button
                  v-for="c in CYCLES"
                  :key="c.key"
                  type="button"
                  class="relative cursor-pointer rounded-lg border px-3 py-2.5 text-center transition-colors"
                  :class="cycleKey === c.key ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
                  @click="cycleKey = c.key"
                >
                  <span v-if="fmtDiscountBps(cycleBps(bootSku, c.months))" class="absolute -top-2 -right-1.5 rounded-full bg-red-500 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-white shadow-sm">{{ fmtDiscountBps(cycleBps(bootSku, c.months)) }}</span>
                  <div class="text-sm font-medium">{{ t(`billing.cycle_${c.key}`) }}</div>
                  <div class="text-muted-foreground text-xs tabular-nums">¥{{ fmtCents(bootCyclePriceCents(c.months)) }}</div>
                </button>
              </div>
            </div>
            <!-- 数量 -->
            <div class="flex flex-col gap-2">
              <span class="text-muted-foreground text-sm">{{ t('billing.quantity') }}</span>
              <div class="flex items-center gap-2">
                <Button variant="outline" size="icon" :disabled="bootQty <= 1" @click="stepBootQty(-1)"><Minus class="size-4" /></Button>
                <Input v-model.number="bootQty" type="number" min="1" max="100" class="h-9 w-20 text-center tabular-nums" />
                <Button variant="outline" size="icon" :disabled="bootQty >= 100" @click="stepBootQty(1)"><Plus class="size-4" /></Button>
                <span class="text-muted-foreground text-sm">{{ t('billing.unit') }}</span>
              </div>
            </div>
            <Separator class="mt-auto" />
            <div class="flex items-center justify-between">
              <div class="text-muted-foreground text-sm">
                {{ t('billing.subtotal') }}
                <span v-if="fmtDiscountBps(bootDiscountBps)" class="ml-1 text-red-500 text-xs">{{ fmtDiscountBps(bootDiscountBps) }}</span>
              </div>
              <div class="text-2xl font-semibold tabular-nums">¥{{ fmtCents(bootSubtotalCents) }}</div>
            </div>
            <!-- 支付方式 -->
            <div class="flex flex-col gap-2">
              <span class="text-muted-foreground text-xs">{{ t('billing.payMethod') }}</span>
              <div class="grid grid-cols-3 gap-2">
                <button
                  v-for="m in methods"
                  :key="m.key"
                  type="button"
                  class="relative flex cursor-pointer items-center gap-1.5 rounded-lg border px-2 py-2 transition-colors"
                  :class="payMethodBoot === m.key ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
                  @click="payMethodBoot = m.key"
                >
                  <span class="flex size-6 items-center justify-center rounded text-xs font-bold text-white" :style="{ backgroundColor: m.color }">{{ m.mark }}</span>
                  <span class="text-xs font-medium">{{ t(`billing.pay_${m.key}`) }}</span>
                  <Check v-if="payMethodBoot === m.key" class="text-primary absolute top-1 right-1 size-3" />
                </button>
              </div>
            </div>
            <Button class="w-full" :disabled="submitting" @click="openPay('boot')">{{ t('billing.buyNow') }}</Button>
          </CardContent>
        </Card>

        <!-- 时长包 -->
        <Card v-if="timeSku" class="flex flex-col">
          <CardHeader>
            <CardTitle class="flex items-center gap-2">
              <Clock class="size-5" /> {{ timeSku.sku.name }}
            </CardTitle>
            <CardDescription>{{ timeSku.sku.description }}</CardDescription>
          </CardHeader>
          <CardContent class="flex flex-1 flex-col gap-5">
            <div class="flex flex-col gap-2">
              <span class="text-muted-foreground text-sm">{{ t('billing.package') }}</span>
              <div class="grid grid-cols-2 gap-2">
                <button
                  v-for="pkg in timePackages"
                  :key="pkg.hours"
                  type="button"
                  class="relative cursor-pointer rounded-lg border px-3 py-3 text-center transition-colors"
                  :class="pkgHours === pkg.hours ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
                  @click="pkgHours = pkg.hours"
                >
                  <span v-if="fmtDiscountBps(pkg.discount_bps)" class="absolute -top-2 -right-1.5 rounded-full bg-red-500 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-white shadow-sm">{{ fmtDiscountBps(pkg.discount_bps) }}</span>
                  <div class="text-sm font-medium tabular-nums">{{ pkg.hours }} {{ t('billing.hoursUnit') }}</div>
                  <div class="text-muted-foreground text-xs tabular-nums">¥{{ fmtCents(pkg.subtotalCents) }}</div>
                </button>
                <button
                  type="button"
                  class="cursor-pointer rounded-lg border px-3 py-3 text-center transition-colors"
                  :class="pkgHours === 'custom' ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
                  @click="pkgHours = 'custom'"
                >
                  <div class="text-sm font-medium">{{ t('billing.custom') }}</div>
                  <div class="text-muted-foreground text-xs tabular-nums">¥{{ fmtCents(timeUnitPriceCents) }} / {{ t('billing.hoursUnit') }}</div>
                </button>
              </div>
            </div>
            <div v-if="pkgHours === 'custom'" class="flex flex-col gap-1.5">
              <span class="text-muted-foreground text-sm">{{ t('billing.customHours') }}</span>
              <div class="flex items-center gap-2">
                <Input v-model.number="customHours" type="number" min="1" step="100" class="h-9 w-32 tabular-nums" :aria-invalid="customInvalid" :class="customInvalid ? 'border-red-500 focus-visible:ring-red-500/30' : ''" />
                <span class="text-muted-foreground text-sm">{{ t('billing.hoursUnit') }}</span>
              </div>
              <p :class="customInvalid ? 'text-red-500' : 'text-muted-foreground'" class="text-xs">{{ t('billing.customHoursHint') }}</p>
            </div>
            <Separator class="mt-auto" />
            <div class="flex items-center justify-between">
              <div class="text-muted-foreground text-sm">
                {{ t('billing.subtotal') }}
                <span v-if="fmtDiscountBps(timeDiscountBps)" class="ml-1 text-red-500 text-xs">{{ fmtDiscountBps(timeDiscountBps) }}</span>
              </div>
              <div class="text-2xl font-semibold tabular-nums">¥{{ fmtCents(timeSubtotalCents) }}</div>
            </div>
            <!-- 支付方式 -->
            <div class="flex flex-col gap-2">
              <span class="text-muted-foreground text-xs">{{ t('billing.payMethod') }}</span>
              <div class="grid grid-cols-3 gap-2">
                <button
                  v-for="m in methods"
                  :key="m.key"
                  type="button"
                  class="relative flex cursor-pointer items-center gap-1.5 rounded-lg border px-2 py-2 transition-colors"
                  :class="payMethodTime === m.key ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
                  @click="payMethodTime = m.key"
                >
                  <span class="flex size-6 items-center justify-center rounded text-xs font-bold text-white" :style="{ backgroundColor: m.color }">{{ m.mark }}</span>
                  <span class="text-xs font-medium">{{ t(`billing.pay_${m.key}`) }}</span>
                  <Check v-if="payMethodTime === m.key" class="text-primary absolute top-1 right-1 size-3" />
                </button>
              </div>
            </div>
            <Button class="w-full" :disabled="customInvalid || submitting" @click="openPay('time')">{{ t('billing.buyNow') }}</Button>
          </CardContent>
        </Card>
      </div>

      <!-- ========= 收银台版 ========= -->
      <div v-else class="grid gap-6 lg:grid-cols-3">
        <!-- 左：SKU 选择 -->
        <div class="flex flex-col gap-6 lg:col-span-2">

          <!-- 实例费 -->
          <Card v-if="instanceSku">
            <CardHeader>
              <CardTitle class="flex items-center gap-2 text-base">
                <Smartphone class="size-5" /> {{ instanceSku.sku.name }}
              </CardTitle>
              <CardDescription>{{ instanceSku.sku.description }}</CardDescription>
            </CardHeader>
            <CardContent class="flex flex-col gap-5">
              <div class="flex flex-col gap-2">
                <span class="text-muted-foreground text-sm">{{ t('billing.cycle') }}</span>
                <div class="grid grid-cols-3 gap-2">
                  <button
                    v-for="c in CYCLES"
                    :key="c.key"
                    type="button"
                    class="relative cursor-pointer rounded-lg border px-3 py-2.5 text-center transition-colors"
                    :class="cycleKey === c.key ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
                    @click="cycleKey = c.key"
                  >
                    <span v-if="fmtDiscountBps(cycleBps(instanceSku, c.months))" class="absolute -top-2 -right-1.5 rounded-full bg-red-500 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-white shadow-sm">{{ fmtDiscountBps(cycleBps(instanceSku, c.months)) }}</span>
                    <div class="text-sm font-medium">{{ t(`billing.cycle_${c.key}`) }}</div>
                    <div class="text-muted-foreground text-xs tabular-nums">¥{{ fmtCents(instanceCyclePriceCents(c.months)) }}</div>
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
                  <span v-if="fmtDiscountBps(instanceDiscountBps)" class="ml-1 text-red-500 text-xs">{{ fmtDiscountBps(instanceDiscountBps) }}</span>
                  <span class="ml-1.5 font-semibold">¥{{ fmtCents(instanceSubtotalCents) }}</span>
                </div>
                <Button :variant="addedInstances ? 'secondary' : 'default'" size="sm" @click="addedInstances = !addedInstances">
                  <component :is="addedInstances ? Check : Plus" class="size-4" />
                  {{ addedInstances ? t('billing.added') : t('billing.addToOrder') }}
                </Button>
              </div>
            </CardContent>
          </Card>

          <!-- 开机包 -->
          <Card v-if="bootSku">
            <CardHeader>
              <CardTitle class="flex items-center gap-2 text-base">
                <Smartphone class="size-5" /> {{ bootSku.sku.name }}
              </CardTitle>
              <CardDescription>{{ bootSku.sku.description }}</CardDescription>
            </CardHeader>
            <CardContent class="flex flex-col gap-5">
              <div class="flex flex-col gap-2">
                <span class="text-muted-foreground text-sm">{{ t('billing.cycle') }}</span>
                <div class="grid grid-cols-3 gap-2">
                  <button
                    v-for="c in CYCLES"
                    :key="c.key"
                    type="button"
                    class="relative cursor-pointer rounded-lg border px-3 py-2.5 text-center transition-colors"
                    :class="cycleKey === c.key ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
                    @click="cycleKey = c.key"
                  >
                    <span v-if="fmtDiscountBps(cycleBps(bootSku, c.months))" class="absolute -top-2 -right-1.5 rounded-full bg-red-500 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-white shadow-sm">{{ fmtDiscountBps(cycleBps(bootSku, c.months)) }}</span>
                    <div class="text-sm font-medium">{{ t(`billing.cycle_${c.key}`) }}</div>
                    <div class="text-muted-foreground text-xs tabular-nums">¥{{ fmtCents(bootCyclePriceCents(c.months)) }}</div>
                  </button>
                </div>
              </div>
              <div class="flex flex-col gap-2">
                <span class="text-muted-foreground text-sm">{{ t('billing.quantity') }}</span>
                <div class="flex items-center gap-2">
                  <Button variant="outline" size="icon" :disabled="bootQty <= 1" @click="stepBootQty(-1)"><Minus class="size-4" /></Button>
                  <Input v-model.number="bootQty" type="number" min="1" max="100" class="h-9 w-20 text-center tabular-nums" />
                  <Button variant="outline" size="icon" :disabled="bootQty >= 100" @click="stepBootQty(1)"><Plus class="size-4" /></Button>
                  <span class="text-muted-foreground text-sm">{{ t('billing.unit') }}</span>
                </div>
              </div>
              <Separator />
              <div class="flex items-center justify-between">
                <div class="tabular-nums">
                  <span class="text-muted-foreground text-sm">{{ t('billing.subtotal') }}</span>
                  <span v-if="fmtDiscountBps(bootDiscountBps)" class="ml-1 text-red-500 text-xs">{{ fmtDiscountBps(bootDiscountBps) }}</span>
                  <span class="ml-1.5 font-semibold">¥{{ fmtCents(bootSubtotalCents) }}</span>
                </div>
                <Button :variant="addedBoot ? 'secondary' : 'default'" size="sm" @click="addedBoot = !addedBoot">
                  <component :is="addedBoot ? Check : Plus" class="size-4" />
                  {{ addedBoot ? t('billing.added') : t('billing.addToOrder') }}
                </Button>
              </div>
            </CardContent>
          </Card>

          <!-- 时长包 -->
          <Card v-if="timeSku">
            <CardHeader>
              <CardTitle class="flex items-center gap-2 text-base">
                <Clock class="size-5" /> {{ timeSku.sku.name }}
              </CardTitle>
              <CardDescription>{{ timeSku.sku.description }}</CardDescription>
            </CardHeader>
            <CardContent class="flex flex-col gap-5">
              <div class="flex flex-col gap-2">
                <span class="text-muted-foreground text-sm">{{ t('billing.package') }}</span>
                <div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
                  <button
                    v-for="pkg in timePackages"
                    :key="pkg.hours"
                    type="button"
                    class="relative cursor-pointer rounded-lg border px-3 py-3 text-center transition-colors"
                    :class="pkgHours === pkg.hours ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
                    @click="pkgHours = pkg.hours"
                  >
                    <span v-if="fmtDiscountBps(pkg.discount_bps)" class="absolute -top-2 -right-1.5 rounded-full bg-red-500 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-white shadow-sm">{{ fmtDiscountBps(pkg.discount_bps) }}</span>
                    <div class="text-sm font-medium tabular-nums">{{ pkg.hours }} {{ t('billing.hoursUnit') }}</div>
                    <div class="text-muted-foreground text-xs tabular-nums">¥{{ fmtCents(pkg.subtotalCents) }}</div>
                  </button>
                  <button
                    type="button"
                    class="cursor-pointer rounded-lg border px-3 py-3 text-center transition-colors"
                    :class="pkgHours === 'custom' ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
                    @click="pkgHours = 'custom'"
                  >
                    <div class="text-sm font-medium">{{ t('billing.custom') }}</div>
                    <div class="text-muted-foreground text-xs tabular-nums">¥{{ fmtCents(timeUnitPriceCents) }} / {{ t('billing.hoursUnit') }}</div>
                  </button>
                </div>
              </div>
              <div v-if="pkgHours === 'custom'" class="flex flex-col gap-1.5">
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
                  <span v-if="fmtDiscountBps(timeDiscountBps)" class="ml-1 text-red-500 text-xs">{{ fmtDiscountBps(timeDiscountBps) }}</span>
                  <span class="ml-1.5 font-semibold">¥{{ fmtCents(timeSubtotalCents) }}</span>
                </div>
                <Button :variant="addedTime ? 'secondary' : 'default'" size="sm" :disabled="customInvalid" @click="addedTime = !addedTime">
                  <component :is="addedTime ? Check : Plus" class="size-4" />
                  {{ addedTime ? t('billing.added') : t('billing.addToOrder') }}
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
              <!-- 空购物车 -->
              <div v-if="cartEmpty" class="flex flex-col items-center gap-2 py-8 text-center">
                <ShoppingCart class="text-muted-foreground/50 size-8" />
                <p class="text-muted-foreground text-sm">{{ t('billing.emptyCart') }}</p>
                <p class="text-muted-foreground/80 text-xs">{{ t('billing.emptyCartHint') }}</p>
              </div>

              <template v-else>
                <!-- 已加入项 -->
                <div v-if="addedInstances" class="flex items-start gap-2">
                  <div class="min-w-0 flex-1">
                    <div class="text-sm font-medium">{{ instanceSku?.sku.name ?? t('billing.lineInstances') }}</div>
                    <div class="text-muted-foreground text-xs tabular-nums">{{ qty }} {{ t('billing.unit') }} · {{ t(`billing.cycle_${cycleKey}`) }}</div>
                  </div>
                  <div class="text-sm font-medium tabular-nums">¥{{ fmtCents(instanceSubtotalCents) }}</div>
                  <Button variant="ghost" size="icon" class="text-muted-foreground hover:text-destructive size-6" @click="addedInstances = false"><X class="size-3.5" /></Button>
                </div>
                <div v-if="addedBoot" class="flex items-start gap-2">
                  <div class="min-w-0 flex-1">
                    <div class="text-sm font-medium">{{ bootSku?.sku.name ?? t('billing.lineBootPack') }}</div>
                    <div class="text-muted-foreground text-xs tabular-nums">{{ bootQty }} {{ t('billing.unit') }} · {{ t(`billing.cycle_${cycleKey}`) }}</div>
                  </div>
                  <div class="text-sm font-medium tabular-nums">¥{{ fmtCents(bootSubtotalCents) }}</div>
                  <Button variant="ghost" size="icon" class="text-muted-foreground hover:text-destructive size-6" @click="addedBoot = false"><X class="size-3.5" /></Button>
                </div>
                <div v-if="addedTime" class="flex items-start gap-2">
                  <div class="min-w-0 flex-1">
                    <div class="text-sm font-medium">{{ timeSku?.sku.name ?? t('billing.lineHours') }}</div>
                    <div class="text-muted-foreground text-xs tabular-nums">{{ selectedHours }} {{ t('billing.hoursUnit') }}</div>
                    <p v-if="customInvalid" class="text-xs text-red-500">{{ t('billing.customHoursHint') }}</p>
                  </div>
                  <div class="text-sm font-medium tabular-nums" :class="customInvalid ? 'text-muted-foreground line-through' : ''">¥{{ fmtCents(timeSubtotalCents) }}</div>
                  <Button variant="ghost" size="icon" class="text-muted-foreground hover:text-destructive size-6" @click="addedTime = false"><X class="size-3.5" /></Button>
                </div>

                <Separator />
                <div class="flex items-baseline justify-between">
                  <span class="text-sm font-medium">{{ t('billing.total') }}</span>
                  <span class="text-2xl font-semibold tabular-nums text-red-600">¥{{ fmtCents(cartTotalCents) }}</span>
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

                <Button class="w-full" size="lg" :disabled="checkoutDisabled" @click="checkout">
                  {{ submitting ? t('billing.submitting') : t('billing.checkout') }}
                </Button>
              </template>
            </CardContent>
          </Card>
        </div>
      </div>
    </template>

    <!-- SKU 加载失败占位 -->
    <div v-else-if="!loading" class="text-muted-foreground py-12 text-center text-sm">
      {{ t('billing.loadSkuFailed') }}
    </div>

    <!-- 标准版单项支付弹框 -->
    <Dialog v-model:open="payOpen">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{{ t('billing.payTitle') }}</DialogTitle>
          <DialogDescription>{{ paySummary }}</DialogDescription>
        </DialogHeader>
        <div class="bg-muted/40 flex items-baseline justify-between rounded-lg border px-4 py-3">
          <span class="text-muted-foreground text-sm">{{ t('billing.payable') }}</span>
          <span class="text-2xl font-semibold tabular-nums text-red-600">¥{{ fmtCents(payAmountCents) }}</span>
        </div>
        <!-- 余额信息 -->
        <div v-if="accountData" class="text-muted-foreground flex justify-between rounded-lg border px-3 py-2.5 text-sm">
          <span>{{ t('billing.overviewBalance') }}</span>
          <span class="font-medium">¥{{ fmtCents(balanceCents) }}</span>
        </div>
        <!-- 支付方式（只读回显） -->
        <div class="flex items-center justify-between rounded-lg border px-3 py-2.5">
          <span class="text-muted-foreground text-sm">{{ t('billing.payMethod') }}</span>
          <span class="flex items-center gap-2">
            <span class="flex size-6 items-center justify-center rounded text-xs font-bold text-white" :style="{ backgroundColor: currentMethod.color }">{{ currentMethod.mark }}</span>
            <span class="text-sm font-medium">{{ t(`billing.pay_${currentMethod.key}`) }}</span>
          </span>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="payOpen = false">{{ t('crud.cancel') }}</Button>
          <Button :disabled="submitting" @click="confirmPay">
            {{ submitting ? t('billing.submitting') : t('billing.payNow') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
