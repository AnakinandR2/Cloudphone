<script setup lang="ts">
import type { BillingOverview, PurchaseConfig } from '@/types/billing'
import { Clock, Power, Smartphone, Wallet } from 'lucide-vue-next'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import billingApi from '@/api/modules/billing'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { fmtCents } from '@/utils/money'
import OrderHistoryPanel from './purchase/OrderHistoryPanel.vue'
import ProductBuyPanel from './purchase/ProductBuyPanel.vue'
import ProductRenewPanel from './purchase/ProductRenewPanel.vue'
import RechargePanel from './purchase/RechargePanel.vue'
import RuntimePackPanel from './purchase/RuntimePackPanel.vue'

const { t } = useI18n()

// tab：内容区容器。点 KPI 按钮切 tab，默认订单历史。
type Tab = 'orders' | 'recharge' | 'seat_new' | 'seat_renew' | 'boot_slot_new' | 'boot_slot_renew' | 'runtime'
const tab = ref<Tab>('orders')

const loading = ref(false)
const overview = ref<BillingOverview | null>(null)
const config = ref<PurchaseConfig | null>(null)
const orderHistory = ref<InstanceType<typeof OrderHistoryPanel> | null>(null)

const balanceCents = computed(() => overview.value?.balance_cents ?? 0)

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

// 任一面板支付成功后：刷新 KPI + 订单列表，并切回订单历史。
async function onPaid() {
  await loadOverview()
  await orderHistory.value?.reload()
  tab.value = 'orders'
}

function go(target: Tab) {
  tab.value = target
}

// tab 标题（内容区头部）
const tabTitle = computed(() => t(`billing.purchase2.tab_${tab.value}`))
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
          <Button variant="outline" size="sm" @click="go('recharge')">
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
            <Button variant="outline" size="sm" class="flex-1" @click="go('seat_new')">
              {{ t('billing.purchase2.kpiBuyInstance') }}
            </Button>
            <Button variant="outline" size="sm" class="flex-1" @click="go('seat_renew')">
              {{ t('billing.purchase2.kpiRenewInstance') }}
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
          <Button variant="outline" size="sm" @click="go('runtime')">
            {{ t('billing.purchase2.kpiBuyRuntime') }}
          </Button>
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
            <Button variant="outline" size="sm" class="flex-1" @click="go('boot_slot_new')">
              {{ t('billing.purchase2.kpiBuyBootSlot') }}
            </Button>
            <Button variant="outline" size="sm" class="flex-1" @click="go('boot_slot_renew')">
              {{ t('billing.purchase2.kpiRenewBootSlot') }}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- ===== 内容区：tab 容器 ===== -->
    <Card v-if="config">
      <CardContent class="flex flex-col gap-5 py-5">
        <!-- tab 头 -->
        <div class="flex items-center justify-between gap-3">
          <h2 class="text-base font-semibold">
            {{ tabTitle }}
          </h2>
          <Button v-if="tab !== 'orders'" variant="ghost" size="sm" @click="go('orders')">
            {{ t('billing.purchase2.backToOrders') }}
          </Button>
        </div>

        <OrderHistoryPanel v-if="tab === 'orders'" ref="orderHistory" @paid="loadOverview" />
        <RechargePanel v-else-if="tab === 'recharge'" :config="config" :balance-cents="balanceCents" @paid="onPaid" />
        <ProductBuyPanel v-else-if="tab === 'seat_new'" kind="seat" :config="config" :balance-cents="balanceCents" @paid="onPaid" />
        <ProductRenewPanel v-else-if="tab === 'seat_renew'" kind="seat" :config="config" :balance-cents="balanceCents" @paid="onPaid" />
        <ProductBuyPanel v-else-if="tab === 'boot_slot_new'" kind="boot_slot" :config="config" :balance-cents="balanceCents" @paid="onPaid" />
        <ProductRenewPanel v-else-if="tab === 'boot_slot_renew'" kind="boot_slot" :config="config" :balance-cents="balanceCents" @paid="onPaid" />
        <RuntimePackPanel v-else-if="tab === 'runtime'" :config="config" :balance-cents="balanceCents" @paid="onPaid" />
      </CardContent>
    </Card>

    <div v-else-if="loading" class="space-y-3">
      <Skeleton class="h-96 rounded-xl" />
    </div>
    <div v-else class="text-muted-foreground py-12 text-center text-sm">
      {{ t('billing.purchase2.loadFailed') }}
    </div>
  </div>
</template>
