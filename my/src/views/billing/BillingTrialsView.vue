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
import { useBillingStore } from '@/stores/billing'

const { t } = useI18n()
const billingStore = useBillingStore()

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
    // 同步菜单「待领取」徽标计数
    billingStore.claimableTrials = res.data.filter(it => it.claimable).length
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

// ——— 科目名称（新模型：seat/boot_slot/runtime_minute，复用购买页 KPI 名称）———
function subjectName(subject: string): string {
  if (subject === 'seat') return t('billing.purchase2.kpiSeat')
  if (subject === 'boot_slot') return t('billing.purchase2.kpiBootSlot')
  if (subject === 'runtime_minute') return t('billing.purchase2.kpiRuntime')
  return subject
}

// ——— 科目单位 ———
function subjectUnit(subject: string): string {
  if (subject === 'seat') return t('billing.trialUnitTai')
  if (subject === 'boot_slot') return t('billing.unitGe')
  if (subject === 'runtime_minute') return t('billing.unitMin')
  return ''
}

// 统一展示顺序：实例席位 → 包月开机数 → 临时开机时长。
const SUBJECT_ORDER = ['seat', 'boot_slot', 'runtime_minute']
function subjectRank(s: string): number {
  const i = SUBJECT_ORDER.indexOf(s)
  return i < 0 ? 99 : i
}

// ——— 授予内容（多发放项，逐项「名称 数量单位 · 到期」，逐行展示）———
function grantLines(item: ClaimableItem): string[] {
  return [...(item.policy.items ?? [])].sort((a, b) => subjectRank(a.subject) - subjectRank(b.subject)).map((it) => {
    const expire = it.expire_days > 0
      ? t('billing.trialExpireDays', { n: it.expire_days })
      : t('billing.trialPermanent')
    return `${subjectName(it.subject)} ${it.quantity} ${subjectUnit(it.subject)} · ${expire}`
  })
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
      <h1 class="text-xl font-semibold tracking-tight">
        {{ t('billing.trialsTitle') }}
      </h1>
      <p class="text-muted-foreground mt-1 text-sm">
        {{ t('billing.trialsDesc') }}
      </p>
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
      <p class="text-sm">
        {{ t('billing.trialsEmpty') }}
      </p>
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
            <CardTitle class="text-base leading-tight">
              {{ item.policy.name }}
            </CardTitle>
            <Badge v-if="item.claimed_count > 0" variant="secondary" class="shrink-0 text-xs">
              {{ t('billing.trialClaimedCount', { n: item.claimed_count }) }}
            </Badge>
          </div>
          <CardDescription class="mt-1 text-xs">
            {{ t('billing.trialGrantDesc') }}
          </CardDescription>
          <ul class="text-muted-foreground mt-1 space-y-0.5 text-xs">
            <li v-for="(line, i) in grantLines(item)" :key="i">
              {{ line }}
            </li>
          </ul>
        </CardHeader>

        <CardContent class="pb-2">
          <!-- 每人限领次数 -->
          <div class="text-muted-foreground text-xs">
            {{ t('billing.trialPerUserLimit', { n: item.policy.per_user_limit }) }}
          </div>

          <!-- 邀请码输入（需邀请码即显示，凭码领取） -->
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
