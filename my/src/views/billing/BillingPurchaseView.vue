<script setup lang="ts">
import type { BillingOverview, PurchaseConfig } from '@/types/billing'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import billingApi from '@/api/modules/billing'
import iconBalance from '@/assets/icons/billing/balance.svg'
import iconBoot from '@/assets/icons/billing/boot.svg'
import iconRuntime from '@/assets/icons/billing/runtime.svg'
import iconSeat from '@/assets/icons/billing/seat.svg'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { fmtCents } from '@/utils/money'
import OrderHistoryPanel from './purchase/OrderHistoryPanel.vue'
import ProductBuyPanel from './purchase/ProductBuyPanel.vue'
import ProductRenewPanel from './purchase/ProductRenewPanel.vue'
import PurchaseKpiCard from './purchase/PurchaseKpiCard.vue'
import RechargePanel from './purchase/RechargePanel.vue'
import RuntimePackPanel from './purchase/RuntimePackPanel.vue'

const { t } = useI18n()

// KPI 下方固定显示订单历史；充值/购买/续费等表单点 KPI 按钮后在右侧抽屉展示。
type Panel = 'recharge' | 'seat_new' | 'seat_renew' | 'boot_slot_new' | 'boot_slot_renew' | 'runtime'
const active = ref<Panel | null>(null)
const open = ref(false)

const loading = ref(false)
const overview = ref<BillingOverview | null>(null)
const config = ref<PurchaseConfig | null>(null)
const orderHistory = ref<InstanceType<typeof OrderHistoryPanel> | null>(null)

const balanceCents = computed(() => overview.value?.balance_cents ?? 0)
const panelTitle = computed(() => active.value ? t(`billing.purchase2.tab_${active.value}`) : '')

async function loadOverview() {
  try {
    const res = await billingApi.overview()
    overview.value = res.data
  }
  catch {
    // 概览非关键，静默
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
</script>

<template>
  <div class="-mx-6 -mt-6 flex min-h-0 flex-1 flex-col">
    <!-- 购买资源区（Figma 青绿底 + 深色 KPI 卡片） -->
    <section class="bg-primary px-6 pb-12 pt-6">
      <div class="space-y-6">
        <div>
          <h1 class="text-2xl font-semibold tracking-tight text-white">
            {{ t('billing.purchase2.title') }}
          </h1>
          <p class="mt-1 text-sm text-white/80">
            {{ t('billing.purchase2.desc') }}
          </p>
        </div>

        <div v-if="loading" class="flex flex-col gap-6 lg:flex-row lg:items-stretch">
          <div class="flex min-w-0 flex-1 gap-3">
            <Skeleton v-for="i in 3" :key="i" class="h-[220px] min-w-0 flex-1 rounded-[24px] bg-black/10" />
          </div>
          <Skeleton class="balance-card h-[220px] w-full shrink-0 rounded-2xl bg-white/10" />
        </div>

        <div v-else-if="overview" class="flex flex-col gap-6 lg:flex-row lg:items-stretch">
          <div class="flex min-w-0 flex-1 gap-3">
            <PurchaseKpiCard
              :title="t('billing.purchase2.kpiSeat')"
              :icon-src="iconSeat"
              :value="overview.seat.used"
              :suffix="t('billing.purchase2.kpiTotal', { n: overview.seat.total })"
              :actions="[
                { label: t('billing.purchase2.kpiBuyInstance'), onClick: () => openPanel('seat_new') },
                { label: t('billing.purchase2.kpiRenewInstance'), onClick: () => openPanel('seat_renew') },
              ]"
            />
            <PurchaseKpiCard
              :title="t('billing.purchase2.kpiBootSlot')"
              :icon-src="iconBoot"
              :value="overview.boot_slot.in_use"
              :suffix="t('billing.purchase2.kpiTotal', { n: overview.boot_slot.total })"
              :actions="[
                { label: t('billing.purchase2.kpiBuyBootSlot'), onClick: () => openPanel('boot_slot_new') },
                { label: t('billing.purchase2.kpiRenewBootSlot'), onClick: () => openPanel('boot_slot_renew') },
              ]"
            />
            <PurchaseKpiCard
              :title="t('billing.purchase2.kpiRuntime')"
              :icon-src="iconRuntime"
              :value="overview.runtime_minutes_remaining"
              :suffix="t('billing.purchase2.minuteUnit')"
              :actions="[
                { label: t('billing.purchase2.kpiBuyRuntime'), onClick: () => openPanel('runtime') },
              ]"
            />
          </div>

          <div class="balance-card flex min-h-[220px] w-full shrink-0 flex-col gap-10 rounded-2xl p-6">
            <div class="flex flex-col gap-6">
              <img :src="iconBalance" alt="" class="size-12 shrink-0">
              <div class="space-y-1">
                <p class="text-lg font-semibold text-white">
                  {{ t('billing.purchase2.kpiBalance') }}
                </p>
                <p class="text-[40px] font-bold leading-10 tabular-nums text-white">
                  ¥{{ fmtCents(balanceCents) }}
                </p>
              </div>
            </div>
            <button
              type="button"
              class="h-11 w-full rounded-[12px] bg-black/20 text-base font-semibold text-white transition-colors hover:bg-black/30"
              @click="openPanel('recharge')"
            >
              {{ t('billing.purchase2.kpiRecharge') }}
            </button>
          </div>
        </div>
      </div>
    </section>

    <!-- 订单历史（表格组件不改，仅外层容器对齐设计稿） -->
    <section class="relative z-10 -mt-5 flex-1 rounded-t-[32px] bg-white px-8 pb-8 pt-10">
      <h2 class="mb-6 text-2xl font-semibold tracking-tight">
        {{ t('billing.purchase2.tab_orders') }}
      </h2>
      <OrderHistoryPanel ref="orderHistory" @paid="loadOverview" />
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
          <template v-if="config">
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

<style scoped>
/* 1920 视口为 400px 基准；>1920 随视口线性增至 600px（2560 时触顶） */
.balance-card {
  width: 100%;
}

@media (width >= 1024px) {
  .balance-card {
    width: clamp(
      400px,
      calc(400px + max(0px, 100vw - 1920px) * 5 / 16),
      600px
    );
  }
}
</style>
