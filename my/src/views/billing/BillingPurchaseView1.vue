<script setup lang="ts">
import type { BillingOverview, PurchaseConfig } from '@/types/billing'
import type { LibraryOverview } from '@/types/library'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import billingApi from '@/api/modules/billing'
import libraryApi from '@/api/modules/library'
import iconWallet from '@/assets/icons/billing/wallet-balance.svg'
import iconAlipay from '@/assets/icons/billing/pay-alipay.svg'
import iconUnionPay from '@/assets/icons/billing/pay-unionpay.svg'
import iconWechat from '@/assets/icons/billing/pay-wechat.svg'
import imgBoot from '@/assets/images/billing/card-boot.png'
import imgLibrary from '@/assets/images/billing/card-library.png'
import imgRuntime from '@/assets/images/billing/card-runtime.png'
import imgSeat from '@/assets/images/billing/card-seat.png'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { fmtCents } from '@/utils/money'
import PackagePanel from '@/views/library/PackagePanel.vue'
import OrderHistoryPanel from './purchase/OrderHistoryPanel.vue'
import ProductBuyPanel from './purchase/ProductBuyPanel.vue'
import ProductRenewPanel from './purchase/ProductRenewPanel.vue'
import PurchaseResourceCardV1 from './purchase/PurchaseResourceCardV1.vue'
import RechargePanel from './purchase/RechargePanel.vue'
import RuntimePackPanel from './purchase/RuntimePackPanel.vue'

const { t } = useI18n()

type Panel = 'recharge' | 'seat_new' | 'seat_renew' | 'boot_slot_new' | 'boot_slot_renew' | 'runtime' | 'library'
const active = ref<Panel | null>(null)
const open = ref(false)

const loading = ref(false)
const overview = ref<BillingOverview | null>(null)
const library = ref<LibraryOverview | null>(null)
const config = ref<PurchaseConfig | null>(null)
const orderHistory = ref<InstanceType<typeof OrderHistoryPanel> | null>(null)

const balanceCents = computed(() => overview.value?.balance_cents ?? 0)
const panelTitle = computed(() => {
  if (!active.value) return ''
  if (active.value === 'library') return t('billing.purchase2.tab_library')
  return t(`billing.purchase2.tab_${active.value}`)
})

const libraryUsedGb = computed(() => {
  if (!library.value) return 24
  return library.value.usage.used_bytes / (1024 ** 3)
})
const libraryTotalGb = computed(() => {
  if (!library.value) return 500
  return library.value.usage.capacity_bytes / (1024 ** 3)
})
const libraryValue = computed(() => {
  const n = libraryUsedGb.value
  // Figma：大数字取整数部分，小数与单位放在 suffix
  return Math.floor(n)
})
const librarySuffix = computed(() => {
  const used = libraryUsedGb.value
  const total = libraryTotalGb.value
  const frac = (used % 1).toFixed(1).slice(1) // ".0"
  return `${frac}GB / ${total.toFixed(1)}GB`
})

async function loadOverview() {
  try {
    overview.value = (await billingApi.overview()).data
  }
  catch {
    // 概览非关键，静默
  }
  try {
    library.value = (await libraryApi.overview()).data
  }
  catch {
    // 素材库概览可选；无 mock/接口时用占位
  }
}

onMounted(async () => {
  loading.value = true
  try {
    const [ovRes, cfgRes] = await Promise.all([
      billingApi.overview(),
      billingApi.purchaseConfig(),
    ])
    overview.value = ovRes.data
    config.value = cfgRes.data
  }
  finally {
    loading.value = false
  }
  try {
    library.value = (await libraryApi.overview()).data
  }
  catch {
    // 无素材库接口时仍展示卡片，用量走占位
  }
})

function openPanel(p: Panel) {
  active.value = p
  open.value = true
}

async function onPaid() {
  await loadOverview()
  await orderHistory.value?.reload()
  open.value = false
}

function onProductLink() {
  toast.message(t('billing.comingSoon'))
}
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col gap-6 px-3 pb-4 pt-1">
    <!-- 页头 -->
    <div class="min-w-0">
      <h1 class="text-2xl font-semibold tracking-tight text-foreground">
        {{ t('billing.purchase2.title') }}
      </h1>
      <p class="mt-1 text-sm text-[#8f959e]">
        {{ t('billing.purchase2.desc') }}
      </p>
    </div>

    <!-- 余额 + 资源卡：组内 24px -->
    <div class="flex flex-col gap-6">
      <div
        v-if="loading"
        class="h-[58px] w-full rounded-2xl border border-[#eaedf1] bg-card shadow-sm"
      >
        <Skeleton class="size-full rounded-2xl" />
      </div>
      <div
        v-else
        class="flex w-full flex-wrap items-center justify-between gap-x-6 gap-y-3 rounded-2xl border border-[#eaedf1] bg-white px-6 py-3 shadow-sm"
      >
        <!-- shrink-0：空间不够时整块换行，而不是挤叠 -->
        <div class="flex shrink-0 flex-wrap items-center gap-6 sm:gap-[60px]">
          <div class="flex items-center gap-6">
            <img :src="iconWallet" alt="" class="size-6 shrink-0">
            <div class="whitespace-nowrap">
              <p class="text-xs leading-4 text-[#8f959e]">
                {{ t('billing.purchase2.kpiBalanceLabel') }}
              </p>
              <p class="text-base font-bold leading-6 tabular-nums text-primary">
                ¥{{ fmtCents(balanceCents) }}
              </p>
            </div>
          </div>
          <Button
            size="sm"
            class="h-8 rounded-[8px] px-5 text-xs"
            @click="openPanel('recharge')"
          >
            {{ t('billing.purchase2.kpiRecharge') }}
          </Button>
        </div>

        <div class="flex shrink-0 flex-wrap items-center gap-2 text-sm leading-[22px] text-[#646a73]">
          <p class="whitespace-nowrap">
            <span class="font-medium">{{ t('billing.purchase2.balancePayHintLead') }}</span>{{ t('billing.purchase2.balancePayHintMid') }}
          </p>
          <div class="flex items-center gap-1.5">
            <img :src="iconAlipay" alt="Alipay" class="size-[18px] object-contain">
            <img :src="iconWechat" alt="WeChat Pay" class="size-[18px] object-contain">
            <img :src="iconUnionPay" alt="UnionPay" class="h-[15px] w-[25px] object-contain">
          </div>
          <p class="whitespace-nowrap">{{ t('billing.purchase2.balancePayHintAfter') }}</p>
        </div>
      </div>

      <!-- 资源卡片：min 360 换行；行内均分；换行后仍与同列等宽；单卡上限 600 -->
      <div
        v-if="loading"
        class="grid gap-3 [grid-template-columns:repeat(auto-fit,minmax(min(100%,360px),1fr))]"
      >
        <Skeleton
          v-for="i in 4"
          :key="i"
          class="h-[206px] w-full max-w-[600px] rounded-2xl"
        />
      </div>
      <div
        v-else-if="overview"
        class="grid gap-3 [grid-template-columns:repeat(auto-fit,minmax(min(100%,360px),1fr))]"
      >
        <PurchaseResourceCardV1
          :title="t('billing.purchase2.kpiSeat')"
          :value="overview.seat.used"
          :value-suffix="t('billing.purchase2.kpiSeatSuffix', { n: overview.seat.total })"
          :description="t('billing.purchase2.kpiSeatLabel')"
          :illustration="imgSeat"
          :actions="[
            { label: t('billing.purchase2.kpiBuyInstance'), variant: 'default', onClick: () => openPanel('seat_new') },
            { label: t('billing.purchase2.kpiRenewInstance'), variant: 'outline', onClick: () => openPanel('seat_renew') },
          ]"
          @product-link="onProductLink"
        />
        <PurchaseResourceCardV1
          :title="t('billing.purchase2.kpiBootSlot')"
          :value="overview.boot_slot.in_use"
          :value-suffix="t('billing.purchase2.kpiBootSuffix', { n: overview.boot_slot.total })"
          :description="t('billing.purchase2.kpiBootLabel')"
          :illustration="imgBoot"
          :actions="[
            { label: t('billing.purchase2.kpiBuyBootSlot'), variant: 'default', onClick: () => openPanel('boot_slot_new') },
            { label: t('billing.purchase2.kpiRenewBootSlot'), variant: 'outline', onClick: () => openPanel('boot_slot_renew') },
          ]"
          @product-link="onProductLink"
        />
        <PurchaseResourceCardV1
          :title="t('billing.purchase2.kpiRuntime')"
          :value="overview.runtime_minutes_remaining"
          :value-suffix="t('billing.purchase2.minuteUnit')"
          :description="t('billing.purchase2.kpiRuntimeLabel')"
          :illustration="imgRuntime"
          :actions="[
            { label: t('billing.purchase2.kpiBuyRuntime'), variant: 'default', onClick: () => openPanel('runtime') },
          ]"
          @product-link="onProductLink"
        />
        <PurchaseResourceCardV1
          :title="t('billing.purchase2.kpiLibrary')"
          :value="libraryValue"
          :value-suffix="librarySuffix"
          :description="t('billing.purchase2.kpiLibraryLabel')"
          :illustration="imgLibrary"
          :actions="[
            { label: t('billing.purchase2.kpiBuyLibrary'), variant: 'default', onClick: () => openPanel('library') },
          ]"
          @product-link="onProductLink"
        />
      </div>
    </div>

    <!-- 订单历史 -->
    <section class="rounded-[20px] border border-border bg-card shadow-sm">
      <h2 class="px-7 pt-7 text-2xl font-semibold tracking-tight">
        {{ t('billing.purchase2.tab_orders') }}
      </h2>
      <div class="px-7 pb-7 pt-6">
        <OrderHistoryPanel ref="orderHistory" status-tone="primary" @paid="loadOverview" />
      </div>
    </section>

    <!-- 购买/充值/续费抽屉 -->
    <Sheet v-model:open="open">
      <SheetContent side="right" class="flex w-full flex-col gap-0 p-0 sm:max-w-2xl">
        <SheetHeader class="border-b">
          <SheetTitle>{{ panelTitle }}</SheetTitle>
          <SheetDescription class="sr-only">
            {{ panelTitle }}
          </SheetDescription>
        </SheetHeader>
        <div class="min-h-0 flex-1 overflow-y-auto p-5">
          <template v-if="active === 'library'">
            <PackagePanel @paid="onPaid" />
          </template>
          <template v-else-if="config">
            <RechargePanel v-if="active === 'recharge'" :config="config" :balance-cents="balanceCents" @paid="onPaid" />
            <ProductBuyPanel v-else-if="active === 'seat_new'" kind="seat" :config="config" :balance-cents="balanceCents" @paid="onPaid" />
            <ProductRenewPanel v-else-if="active === 'seat_renew'" kind="seat" :config="config" :balance-cents="balanceCents" @paid="onPaid" />
            <ProductBuyPanel v-else-if="active === 'boot_slot_new'" kind="boot_slot" :config="config" :balance-cents="balanceCents" @paid="onPaid" />
            <ProductRenewPanel v-else-if="active === 'boot_slot_renew'" kind="boot_slot" :config="config" :balance-cents="balanceCents" @paid="onPaid" />
            <RuntimePackPanel v-else-if="active === 'runtime'" :config="config" :balance-cents="balanceCents" @paid="onPaid" />
          </template>
        </div>
      </SheetContent>
    </Sheet>
  </div>
</template>
