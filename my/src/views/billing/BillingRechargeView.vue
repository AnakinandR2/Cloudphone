<script setup lang="ts">
import type { BillingOverview, PurchaseConfig } from '@/types/billing'
import { CreditCard, Wallet } from 'lucide-vue-next'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import billingApi from '@/api/modules/billing'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { fmtCents } from '@/utils/money'
import OrderHistoryPanel from './purchase/OrderHistoryPanel.vue'
import RechargePanel from './purchase/RechargePanel.vue'

const { t } = useI18n()

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
    // 概览非关键数据，接口异常时保留页面可操作性。
  }
}

async function loadPage() {
  loading.value = true
  try {
    const [overviewRes, configRes] = await Promise.all([
      billingApi.overview(),
      billingApi.purchaseConfig(),
    ])
    overview.value = overviewRes.data
    config.value = configRes.data
  }
  finally {
    loading.value = false
  }
}

async function onPaid() {
  await loadOverview()
  await orderHistory.value?.reload()
}

onMounted(loadPage)
</script>

<template>
  <div class="flex flex-col gap-6">
    <div>
      <h1 class="text-2xl font-semibold tracking-tight">
        {{ t('billing.purchase2.rechargePageTitle') }}
      </h1>
      <p class="text-muted-foreground mt-1 text-sm">
        {{ t('billing.purchase2.rechargePageDesc') }}
      </p>
    </div>

    <div class="grid gap-6 xl:grid-cols-[360px_minmax(0,1fr)]">
      <Card>
        <CardHeader>
          <CardTitle>{{ t('billing.purchase2.kpiBalance') }}</CardTitle>
          <CardDescription>{{ t('billing.purchase2.rechargeBalanceDesc') }}</CardDescription>
        </CardHeader>
        <CardContent>
          <div v-if="loading" class="space-y-3">
            <Skeleton class="h-10 w-40" />
            <Skeleton class="h-5 w-56" />
          </div>
          <div v-else class="flex items-center gap-4">
            <div class="bg-primary/10 text-primary flex size-12 items-center justify-center rounded-lg">
              <Wallet class="size-6" />
            </div>
            <div>
              <div class="text-muted-foreground text-sm">
                {{ t('billing.purchase2.balanceRemain') }}
              </div>
              <div class="text-3xl font-semibold tabular-nums">
                ¥{{ fmtCents(balanceCents) }}
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{{ t('billing.purchase2.tab_recharge') }}</CardTitle>
          <CardDescription>{{ t('billing.purchase2.rechargeFormDesc') }}</CardDescription>
        </CardHeader>
        <CardContent>
          <div v-if="loading" class="space-y-4">
            <Skeleton class="h-24 w-full" />
            <Skeleton class="h-20 w-full" />
            <Skeleton class="h-10 w-full" />
          </div>
          <RechargePanel
            v-else-if="config"
            :config="config"
            :balance-cents="balanceCents"
            @paid="onPaid"
          />
          <div v-else class="text-muted-foreground flex min-h-32 items-center justify-center gap-2 text-sm">
            <CreditCard class="size-4" />
            {{ t('billing.purchase2.loadFailed') }}
          </div>
        </CardContent>
      </Card>
    </div>

    <Card>
      <CardHeader>
        <CardTitle>{{ t('billing.purchase2.rechargeHistoryTitle') }}</CardTitle>
        <CardDescription>{{ t('billing.purchase2.rechargeHistoryDesc') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <OrderHistoryPanel ref="orderHistory" biz-type="recharge" @paid="loadOverview" />
      </CardContent>
    </Card>
  </div>
</template>
