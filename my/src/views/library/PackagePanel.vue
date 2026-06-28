<script setup lang="ts">
import type { PaymentMethod } from '@/types/billing'
import type {
  LibraryBizType,
  LibraryOverview,
  PackageAction,
  PackageParams,
  PackageQuoteResult,
  TierCfg,
} from '@/types/library'
import { ArrowDownCircle, ArrowUpCircle, Check, RefreshCw, ShoppingBag } from 'lucide-vue-next'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import billingApi from '@/api/modules/billing'
import libraryApi from '@/api/modules/library'
import { Separator } from '@/components/ui/separator'
import { fmtBytes } from '@/utils/bytes'
import { formatDate } from '@/utils/date'
import { fmtCents, fmtDiscountBps } from '@/utils/money'
import PaymentBox from '@/views/billing/purchase/PaymentBox.vue'
import PurchaseNotice from '@/views/billing/purchase/PurchaseNotice.vue'

// 套餐购买面板：档位卡 → 自动判定动作（新购/续费/升级/降级）→ 时长（升级/降级禁用）
// → /library/package/quote 预览 → /billing/orders（lib_*+params）→ /billing/orders/:id/pay。
const props = defineProps<{
  overview: LibraryOverview
}>()
const emit = defineEmits<{ paid: [] }>()
const { t } = useI18n()

// 支付方式与余额（来自 billing）。
const paymentMethods = ref<PaymentMethod[]>([])
const balanceCents = ref(0)
async function loadBilling() {
  try {
    const [cfg, acc] = await Promise.all([billingApi.purchaseConfig(), billingApi.account()])
    paymentMethods.value = cfg.data.payment_methods
    balanceCents.value = acc.data.balance_cents
  }
  catch {
    // 失败由拦截器提示
  }
}
loadBilling()

const cur = computed(() => props.overview.subscription)
// 当前套餐展示名：免费/自定义用文案，预设档用配置里的档名（而非 tier_code 代号）。
const curTierName = computed(() => {
  const s = cur.value
  if (s.is_free) return t('library.package.freePlan')
  if (s.tier_code === 'custom') return t('library.package.customTier')
  return props.overview.tiers.find(x => x.code === s.tier_code)?.name || s.tier_code
})
const tiers = computed(() => props.overview.tiers.filter(x => x.enabled).sort((a, b) => a.sort - b.sort))
const durations = computed(() => props.overview.duration_options)

// 选中档位 code（含自定义档 'custom'）。
const selectedCode = ref<string>('')
const customGB = ref<number>(props.overview.custom_tier.min_capacity_gb || 50)
const days = ref<number>(durations.value[0]?.days ?? 30)
const payMethod = ref('')
const submitting = ref(false)

// 选中档位的月价（用于自动判定动作，前端仅展示用途，权威以后端 quote 为准）。
const selectedTier = computed<TierCfg | null>(() => {
  if (selectedCode.value === 'custom') return null
  return tiers.value.find(x => x.code === selectedCode.value) ?? null
})
const selectedMonthly = computed(() => {
  if (selectedCode.value === 'custom') {
    return customGB.value * props.overview.custom_tier.price_per_gb_month_cents
  }
  return selectedTier.value?.monthly_price_cents ?? 0
})

// 前端预判动作（仅用于 UI 提示与是否禁用时长；最终以 quote.action 为准）。
const predictedAction = computed<PackageAction>(() => {
  if (cur.value.is_free) return 'new'
  // 同档（预设档比 code；自定义档比容量）→ 续费。
  if (selectedCode.value !== 'custom' && selectedCode.value === cur.value.tier_code) return 'renew'
  if (selectedMonthly.value > cur.value.monthly_price_cents) return 'upgrade'
  if (selectedMonthly.value < cur.value.monthly_price_cents) return 'downgrade'
  return 'renew'
})

// 升级/降级禁用时长选择。
const durationDisabled = computed(() =>
  predictedAction.value === 'upgrade' || predictedAction.value === 'downgrade')

// ---- 报价（防抖） ----
const quote = ref<PackageQuoteResult | null>(null)
const quoting = ref(false)
let seq = 0
let timer: ReturnType<typeof setTimeout> | null = null

function buildParams(): PackageParams | null {
  if (!selectedCode.value) return null
  // 升级/降级不可选时长，后端忽略 days；此处也不附带（语义清理）。
  const withDays = !durationDisabled.value
  if (selectedCode.value === 'custom') {
    if (customGB.value < props.overview.custom_tier.min_capacity_gb) return null
    return withDays
      ? { tier_code: 'custom', capacity_gb: customGB.value, days: days.value }
      : { tier_code: 'custom', capacity_gb: customGB.value }
  }
  return withDays
    ? { tier_code: selectedCode.value, days: days.value }
    : { tier_code: selectedCode.value }
}

async function runQuote() {
  const p = buildParams()
  if (!p) {
    quote.value = null
    quoting.value = false
    return
  }
  const my = ++seq
  quoting.value = true
  try {
    const res = await libraryApi.quotePackage(p)
    if (my !== seq) return
    quote.value = res.data
  }
  catch {
    if (my === seq) quote.value = null
  }
  finally {
    if (my === seq) quoting.value = false
  }
}
function scheduleQuote() {
  if (timer) clearTimeout(timer)
  timer = setTimeout(runQuote, 250)
}
watch([selectedCode, customGB, days], scheduleQuote, { immediate: true })

// action → biz_type。
const ACTION_BIZ: Record<PackageAction, LibraryBizType> = {
  new: 'lib_new',
  renew: 'lib_renew',
  upgrade: 'lib_upgrade',
  downgrade: 'lib_downgrade',
}

const actionLabel = computed(() => {
  const a = quote.value?.action ?? predictedAction.value
  return t(`library.package.action.${a}`)
})

// 降级 0 元单仍需选支付方式（后端 total=0 跳过扣款）；这里直接允许确认。
const isZeroOrder = computed(() => (quote.value?.total_cents ?? 0) === 0)

async function confirm() {
  const p = buildParams()
  if (!p || !quote.value) return
  // 用后端权威 action 决定 biz_type。
  const biz = ACTION_BIZ[quote.value.action]
  submitting.value = true
  try {
    const created = await libraryApi.createPackageOrder({
      biz_type: biz,
      pay_method: payMethod.value,
      params: p,
    })
    // 若下单未即时支付（unpaid），补一次 pay。
    if (created.data.pay.status !== 'paid' && created.data.order.id) {
      await libraryApi.payOrder(created.data.order.id)
    }
    const { toast } = await import('vue-sonner')
    toast.success(t('library.package.orderOk'))
    emit('paid')
  }
  catch {
    // 失败由拦截器统一提示
  }
  finally {
    submitting.value = false
  }
}

const tierDiscZhe = computed(() => fmtDiscountBps(quote.value?.tier_discount_bps ?? 10000))
const durDiscZhe = computed(() => fmtDiscountBps(quote.value?.duration_discount_bps ?? 10000))
</script>

<template>
  <div class="flex flex-col gap-5">
    <!-- 购买须知 / 计费说明（与费用购买抽屉同款样式，换行用 whitespace-pre-line） -->
    <PurchaseNotice :notice="overview.notice" :billing-note="overview.billing_note" />

    <!-- 当前订阅与用量 -->
    <div class="bg-muted/30 flex flex-wrap items-center justify-between gap-2 rounded-lg border px-4 py-3 text-sm">
      <div class="flex flex-col gap-0.5">
        <span class="text-muted-foreground text-xs">{{ t('library.package.currentPlan') }}</span>
        <span class="font-medium">
          {{ curTierName }}
          · {{ fmtBytes(cur.capacity_bytes) }}
        </span>
      </div>
      <div v-if="cur.expire_at" class="flex flex-col gap-0.5 text-right">
        <span class="text-muted-foreground text-xs">{{ t('library.package.expireAt') }}</span>
        <span class="tabular-nums">{{ formatDate(cur.expire_at) }}</span>
      </div>
    </div>

    <!-- 档位卡 -->
    <div class="flex flex-col gap-2.5">
      <span class="text-muted-foreground text-sm">{{ t('library.package.chooseTier') }}</span>
      <div class="grid gap-2.5 sm:grid-cols-2 lg:grid-cols-3">
        <button
          v-for="tier in tiers"
          :key="tier.code"
          type="button"
          class="relative flex cursor-pointer flex-col gap-1 rounded-lg border px-3.5 py-3 text-left transition-colors"
          :class="selectedCode === tier.code ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
          @click="selectedCode = tier.code"
        >
          <span class="flex items-center justify-between">
            <span class="text-base font-semibold">{{ tier.name }}</span>
            <Check v-if="selectedCode === tier.code" class="text-primary size-4" />
          </span>
          <span class="text-muted-foreground text-xs">{{ tier.capacity_gb }} GB</span>
          <span class="text-sm">
            <span class="text-red-600 font-semibold tabular-nums">¥{{ fmtCents(tier.monthly_price_cents) }}</span>
            <span class="text-muted-foreground text-xs"> / {{ t('library.package.perMonth') }}</span>
          </span>
          <span
            v-if="fmtDiscountBps(tier.tier_discount_bps)"
            class="absolute -top-2 right-2 rounded-full bg-red-500 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-white shadow-sm"
          >{{ fmtDiscountBps(tier.tier_discount_bps) }}</span>
        </button>

        <!-- 自定义档 -->
        <button
          v-if="overview.custom_tier.enabled"
          type="button"
          class="relative flex cursor-pointer flex-col gap-1.5 rounded-lg border px-3.5 py-3 text-left transition-colors"
          :class="selectedCode === 'custom' ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60'"
          @click="selectedCode = 'custom'"
        >
          <span class="flex items-center justify-between">
            <span class="text-base font-semibold">{{ t('library.package.customTier') }}</span>
            <Check v-if="selectedCode === 'custom'" class="text-primary size-4" />
          </span>
          <span class="text-muted-foreground text-xs">
            {{ t('library.package.customMin', { gb: overview.custom_tier.min_capacity_gb }) }}
          </span>
          <span class="flex items-center gap-2" @click.stop>
            <input
              v-model.number="customGB"
              type="number"
              :min="overview.custom_tier.min_capacity_gb"
              class="border-input w-20 rounded-md border bg-transparent px-2 py-1 text-sm tabular-nums"
            >
            <span class="text-muted-foreground text-xs">GB</span>
          </span>
          <span class="text-sm">
            <span class="text-red-600 font-semibold tabular-nums">¥{{ fmtCents(customGB * overview.custom_tier.price_per_gb_month_cents) }}</span>
            <span class="text-muted-foreground text-xs"> / {{ t('library.package.perMonth') }}</span>
          </span>
        </button>
      </div>
    </div>

    <!-- 时长（升级/降级禁用） -->
    <div v-if="selectedCode" class="flex flex-col gap-2.5">
      <span class="text-muted-foreground text-sm">{{ t('library.package.duration') }}</span>
      <div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <button
          v-for="opt in durations"
          :key="opt.days"
          type="button"
          :disabled="durationDisabled"
          class="relative rounded-lg border px-3 py-2.5 text-center transition-colors"
          :class="[
            days === opt.days && !durationDisabled ? 'border-primary bg-primary/5 ring-primary/30 ring-1' : 'hover:bg-muted/60',
            durationDisabled ? 'cursor-not-allowed opacity-40' : 'cursor-pointer',
          ]"
          @click="!durationDisabled && (days = opt.days)"
        >
          <span
            v-if="fmtDiscountBps(opt.discount_bps)"
            class="absolute -top-2 -right-1.5 rounded-full bg-red-500 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-white shadow-sm"
          >{{ fmtDiscountBps(opt.discount_bps) }}</span>
          <span class="text-sm font-medium tabular-nums">{{ opt.days }} {{ t('library.package.dayUnit') }}</span>
        </button>
      </div>
      <p v-if="durationDisabled" class="text-muted-foreground text-xs">
        {{ t('library.package.durationLocked') }}
      </p>
    </div>

    <!-- 订单摘要 -->
    <div v-if="selectedCode" class="bg-muted/30 relative flex flex-col gap-2 rounded-lg border px-4 py-3.5 text-sm">
      <template v-if="quote">
        <!-- 动作徽章 -->
        <div class="flex items-center justify-between">
          <span class="text-muted-foreground">{{ t('library.package.actionLabel') }}</span>
          <span
            class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium"
            :class="{
              'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300': quote.action === 'new',
              'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300': quote.action === 'renew',
              'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300': quote.action === 'upgrade',
              'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300': quote.action === 'downgrade',
            }"
          >
            <ShoppingBag v-if="quote.action === 'new'" class="size-3" />
            <RefreshCw v-else-if="quote.action === 'renew'" class="size-3" />
            <ArrowUpCircle v-else-if="quote.action === 'upgrade'" class="size-3" />
            <ArrowDownCircle v-else class="size-3" />
            {{ actionLabel }}
          </span>
        </div>
        <div class="flex items-center justify-between">
          <span class="text-muted-foreground">{{ t('library.package.sumCapacity') }}</span>
          <span class="tabular-nums">{{ fmtBytes(quote.capacity_bytes) }}</span>
        </div>
        <div v-if="quote.days > 0" class="flex items-center justify-between">
          <span class="text-muted-foreground">{{ t('library.package.sumDays') }}</span>
          <span class="tabular-nums">{{ quote.days }} {{ t('library.package.dayUnit') }}</span>
        </div>
        <!-- 升级补差价：剩余天数明细 -->
        <div v-if="quote.action === 'upgrade'" class="flex items-center justify-between">
          <span class="text-muted-foreground">{{ t('library.package.sumRemainingDays') }}</span>
          <span class="tabular-nums">{{ quote.remaining_days }} {{ t('library.package.dayUnit') }}</span>
        </div>
        <div v-if="tierDiscZhe || durDiscZhe" class="flex items-center justify-between">
          <span class="text-muted-foreground">{{ t('library.package.sumDiscount') }}</span>
          <span class="tabular-nums text-red-500">
            <span v-if="tierDiscZhe">{{ tierDiscZhe }}</span>
            <span v-if="tierDiscZhe && durDiscZhe"> × </span>
            <span v-if="durDiscZhe">{{ durDiscZhe }}</span>
          </span>
        </div>
        <div v-if="quote.new_expire_at" class="flex items-center justify-between">
          <span class="text-muted-foreground">{{ t('library.package.sumNewExpire') }}</span>
          <span class="tabular-nums">{{ formatDate(quote.new_expire_at) }}</span>
        </div>
        <Separator />
        <div class="flex items-baseline justify-between">
          <span class="font-medium">{{ t('library.package.sumTotal') }}</span>
          <span class="text-2xl font-semibold tabular-nums text-red-600">¥{{ fmtCents(quote.total_cents) }}</span>
        </div>
        <p v-if="quote.action === 'downgrade'" class="text-muted-foreground text-xs">
          {{ t('library.package.downgradeNote') }}
        </p>
      </template>
      <div v-else class="text-muted-foreground py-2 text-center text-xs">
        {{ t('library.package.selectToQuote') }}
      </div>
    </div>

    <!-- 支付：0 元单也走确认（后端跳过扣款）；非 0 走 PaymentBox。 -->
    <PaymentBox
      v-if="selectedCode && quote"
      v-model="payMethod"
      :methods="paymentMethods"
      :allow-balance="true"
      :amount-cents="isZeroOrder ? 1 : quote.total_cents"
      :balance-cents="balanceCents"
      :submitting="submitting"
      :confirm-label="isZeroOrder ? t('library.package.confirmFree') : undefined"
      @confirm="confirm"
    />

  </div>
</template>
