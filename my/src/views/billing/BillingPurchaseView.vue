<script setup lang="ts">
import type { BillingOverview, PurchaseConfig } from '@/types/billing'
import { Clock, Power, Smartphone, Wallet } from 'lucide-vue-next'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import billingApi from '@/api/modules/billing'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { fmtCents } from '@/utils/money'
import OrderHistoryPanel from './purchase/OrderHistoryPanel.vue'
import ProductBuyPanel from './purchase/ProductBuyPanel.vue'
import ProductRenewPanel from './purchase/ProductRenewPanel.vue'
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

// 打开某个购买/充值面板（抽屉）。
function openPanel(p: Panel) {
  active.value = p
  open.value = true
}

// 抽屉内面板支付成功后：刷新 KPI + 订单列表，并关闭抽屉。
async function onPaid() {
  await loadOverview()
  await orderHistory.value?.reload()
  open.value = false
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <div>
      <h1 class="text-xl font-semibold tracking-tight">
        {{ t('billing.purchase2.title') }}
      </h1>
      <p class="text-muted-foreground mt-1 text-sm">
        {{ t('billing.purchase2.desc') }}
      </p>
    </div>

    <!-- ===== 4 KPI 卡片 ===== -->
    <div v-if="loading" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <Skeleton v-for="i in 4" :key="i" class="h-28 rounded-xl" />
    </div>
    <div v-else-if="overview" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <!-- 余额 -->
      <Card>
        <CardContent class="flex flex-col gap-3 py-4">
          <div class="flex items-center gap-3">
            <div class="flex size-10 items-center justify-center rounded-lg bg-indigo-500/10 text-indigo-600 dark:text-indigo-400">
              <Wallet class="size-5" />
            </div>
            <div>
              <div class="text-muted-foreground text-xs">
                {{ t('billing.purchase2.kpiBalance') }}
              </div>
              <div class="text-xl font-semibold tabular-nums">
                ¥{{ fmtCents(balanceCents) }}
              </div>
            </div>
          </div>
          <Button variant="outline" size="sm" @click="openPanel('recharge')">
            {{ t('billing.purchase2.kpiRecharge') }}
          </Button>
        </CardContent>
      </Card>

      <!-- 实例席位 -->
      <Card>
        <CardContent class="flex flex-col gap-3 py-4">
          <div class="flex items-center gap-3">
            <div class="flex size-10 items-center justify-center rounded-lg bg-blue-500/10 text-blue-600 dark:text-blue-400">
              <Smartphone class="size-5" />
            </div>
            <div>
              <div class="text-muted-foreground text-xs">
                {{ t('billing.purchase2.kpiSeat') }}
              </div>
              <div class="text-xl font-semibold tabular-nums">
                {{ overview.seat.total }}<span class="text-muted-foreground text-sm font-normal"> · {{ t('billing.purchase2.kpiUsed', { n: overview.seat.used }) }}</span>
              </div>
            </div>
          </div>
          <div class="flex gap-2">
            <Button variant="outline" size="sm" class="flex-1" @click="openPanel('seat_new')">
              {{ t('billing.purchase2.kpiBuyInstance') }}
            </Button>
            <Button variant="outline" size="sm" class="flex-1" @click="openPanel('seat_renew')">
              {{ t('billing.purchase2.kpiRenewInstance') }}
            </Button>
          </div>
        </CardContent>
      </Card>

      <!-- 包月开机数 -->
      <Card>
        <CardContent class="flex flex-col gap-3 py-4">
          <div class="flex items-center gap-3">
            <div class="flex size-10 items-center justify-center rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400">
              <Power class="size-5" />
            </div>
            <div>
              <div class="text-muted-foreground text-xs">
                {{ t('billing.purchase2.kpiBootSlot') }}
              </div>
              <div class="text-xl font-semibold tabular-nums">
                {{ overview.boot_slot.total }}<span class="text-muted-foreground text-sm font-normal"> · {{ t('billing.purchase2.kpiInUse', { n: overview.boot_slot.in_use }) }}</span>
              </div>
            </div>
          </div>
          <div class="flex gap-2">
            <Button variant="outline" size="sm" class="flex-1" @click="openPanel('boot_slot_new')">
              {{ t('billing.purchase2.kpiBuyBootSlot') }}
            </Button>
            <Button variant="outline" size="sm" class="flex-1" @click="openPanel('boot_slot_renew')">
              {{ t('billing.purchase2.kpiRenewBootSlot') }}
            </Button>
          </div>
        </CardContent>
      </Card>

      <!-- 临时开机时长 -->
      <Card>
        <CardContent class="flex flex-col gap-3 py-4">
          <div class="flex items-center gap-3">
            <div class="flex size-10 items-center justify-center rounded-lg bg-amber-500/10 text-amber-600 dark:text-amber-400">
              <Clock class="size-5" />
            </div>
            <div>
              <div class="text-muted-foreground text-xs">
                {{ t('billing.purchase2.kpiRuntime') }}
              </div>
              <div class="text-xl font-semibold tabular-nums">
                {{ overview.runtime_minutes_remaining }}<span class="text-muted-foreground text-sm font-normal"> {{ t('billing.purchase2.minuteUnit') }}</span>
              </div>
            </div>
          </div>
          <Button variant="outline" size="sm" @click="openPanel('runtime')">
            {{ t('billing.purchase2.kpiBuyRuntime') }}
          </Button>
        </CardContent>
      </Card>
    </div>

    <!-- ===== KPI 下方：固定显示订单历史 ===== -->
    <Card>
      <CardHeader>
        <CardTitle>{{ t('billing.purchase2.tab_orders') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <OrderHistoryPanel ref="orderHistory" @paid="loadOverview" />
      </CardContent>
    </Card>

    <!-- ===== 购买/充值/续费抽屉 ===== -->
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
