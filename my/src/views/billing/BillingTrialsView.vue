<script setup lang="ts">
import type { ClaimableItem } from '@/types/billing'
import { Gift } from 'lucide-vue-next'
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import billingApi from '@/api/modules/billing'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'

const { t } = useI18n()

// ——— 数据状态 ———
const items = ref<ClaimableItem[]>([])
const loading = ref(false)

// ——— 每项的邀请码输入（按 policy.code 索引）———
const inviteCodes = ref<Record<string, string>>({})
// ——— 每项的领取中状态 ———
const claiming = ref<Record<string, boolean>>({})

// ——— 加载试用列表 ———
async function load() {
  loading.value = true
  try {
    const res = await billingApi.trials()
    items.value = res.data
    // 初始化邀请码输入
    res.data.forEach((item) => {
      if (!(item.policy.code in inviteCodes.value)) {
        inviteCodes.value[item.policy.code] = ''
      }
    })
  }
  catch {
    toast.error(t('billing.loadFailed'))
  }
  finally {
    loading.value = false
  }
}

onMounted(load)

// ——— 科目单位 ———
function subjectUnit(subject: string): string {
  if (subject === 'instance_seat') return t('billing.trialUnitTai')
  if (subject === 'boot_seat') return t('billing.trialUnitBootSeat')
  if (subject === 'runtime_minute') return t('billing.unitMin')
  return ''
}

// ——— 授予内容描述 ———
function grantDesc(item: ClaimableItem): string {
  const { grant_quantity, grant_subject, grant_expire_days } = item.policy
  const unit = subjectUnit(grant_subject)
  const qty = `${grant_quantity} ${unit}`
  const expire = grant_expire_days > 0
    ? t('billing.trialExpireDays', { n: grant_expire_days })
    : t('billing.trialPermanent')
  return `${qty} · ${expire}`
}

// ——— 领取 ———
async function claim(item: ClaimableItem) {
  const code = item.policy.code
  claiming.value[code] = true
  try {
    const inviteCode = inviteCodes.value[code] ?? ''
    await billingApi.claimTrial(code, inviteCode)
    toast.success(t('billing.trialClaimOk'))
    await load()
  }
  catch (err: unknown) {
    const msg = (err as { message?: string })?.message ?? t('billing.trialClaimFail')
    toast.error(msg)
  }
  finally {
    claiming.value[code] = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <!-- 标题 -->
    <div>
      <h1 class="text-xl font-semibold tracking-tight">{{ t('billing.trialsTitle') }}</h1>
      <p class="text-muted-foreground mt-1 text-sm">{{ t('billing.trialsDesc') }}</p>
    </div>

    <!-- 骨架屏 -->
    <div v-if="loading" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <Card v-for="i in 3" :key="i">
        <CardHeader class="pb-2">
          <Skeleton class="h-5 w-40" />
          <Skeleton class="h-3 w-28 mt-1" />
        </CardHeader>
        <CardContent>
          <Skeleton class="h-4 w-32 mb-2" />
          <Skeleton class="h-4 w-24" />
        </CardContent>
        <CardFooter>
          <Skeleton class="h-9 w-24" />
        </CardFooter>
      </Card>
    </div>

    <!-- 空态 -->
    <div
      v-else-if="items.length === 0"
      class="flex flex-col items-center justify-center py-20 gap-3 text-muted-foreground"
    >
      <Gift class="size-10 opacity-30" />
      <p class="text-sm">{{ t('billing.trialsEmpty') }}</p>
    </div>

    <!-- 试用卡片列表 -->
    <div v-else class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <Card
        v-for="item in items"
        :key="item.policy.code"
        :class="{ 'opacity-60': !item.claimable && !item.need_invite }"
      >
        <CardHeader class="pb-2">
          <div class="flex items-start justify-between gap-2">
            <CardTitle class="text-base leading-tight">{{ item.policy.name }}</CardTitle>
            <Badge v-if="item.claimed_count > 0" variant="secondary" class="shrink-0 text-xs">
              {{ t('billing.trialClaimedCount', { n: item.claimed_count }) }}
            </Badge>
          </div>
          <CardDescription class="mt-1 text-xs">
            {{ t('billing.trialGrantDesc') }}: {{ grantDesc(item) }}
          </CardDescription>
        </CardHeader>

        <CardContent class="pb-2">
          <!-- 每人限领次数 -->
          <div class="text-muted-foreground text-xs">
            {{ t('billing.trialPerUserLimit', { n: item.policy.per_user_limit }) }}
          </div>

          <!-- 邀请码输入（需邀请码即显示，凭码领取）-->
          <div v-if="item.need_invite" class="mt-3">
            <Input
              v-model="inviteCodes[item.policy.code]"
              :placeholder="t('billing.trialInviteCodePlaceholder')"
              class="h-8 text-sm"
            />
          </div>

          <!-- 不可领取原因 -->
          <div
            v-if="!item.claimable && !item.need_invite && item.reason"
            class="text-muted-foreground mt-2 text-xs"
          >
            {{ item.reason }}
          </div>
        </CardContent>

        <CardFooter class="pt-2">
          <!-- 可领取 -->
          <Button
            v-if="item.claimable || item.need_invite"
            size="sm"
            :disabled="(item.need_invite && !inviteCodes[item.policy.code]?.trim()) || claiming[item.policy.code]"
            @click="claim(item)"
          >
            {{ claiming[item.policy.code] ? t('billing.trialClaiming') : t('billing.trialClaim') }}
          </Button>

          <!-- 已达上限 / 不符合条件 -->
          <Button v-else size="sm" variant="outline" disabled>
            {{ t('billing.trialNotClaimable') }}
          </Button>
        </CardFooter>
      </Card>
    </div>
  </div>
</template>
