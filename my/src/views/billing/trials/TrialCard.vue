<script setup lang="ts">
import type { ClaimableItem } from '@/types/billing'
import trialRibbon from '@/assets/icons/billing/trial-ribbon.svg'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { Info } from 'lucide-vue-next'
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'

const props = defineProps<{
  item: ClaimableItem
  claiming: boolean
  subjectName: (subject: string) => string
  subjectUnit: (subject: string) => string
  grantLines: (item: ClaimableItem) => string[]
}>()

const emit = defineEmits<{
  claim: []
  use: []
}>()

const { t } = useI18n()

const SUBJECT_ORDER = ['seat', 'boot_slot', 'runtime_minute']

const grantTextRef = ref<HTMLElement | null>(null)
const grantOverflow = ref(false)

const primaryGrant = computed(() => {
  const items = [...(props.item.policy.items ?? [])].sort(
    (a, b) => SUBJECT_ORDER.indexOf(a.subject) - SUBJECT_ORDER.indexOf(b.subject),
  )
  return items[0]
})

const validityText = computed(() => {
  if (props.item.policy.expire_at) return props.item.policy.expire_at
  const days = primaryGrant.value?.expire_days ?? 0
  if (days > 0) return t('billing.trialExpireDays', { n: days })
  return t('billing.trialPermanent')
})

const couponId = computed(() => props.item.policy.coupon_id ?? props.item.policy.code)

const grantSummary = computed(() => props.grantLines(props.item).join('、'))

const descriptionText = computed(() => props.item.reason)

const isClaimed = computed(() => props.item.claimed_count > 0 && !props.item.claimable)

const isUnavailable = computed(() => !props.item.claimable && props.item.claimed_count === 0)

function checkGrantOverflow() {
  const el = grantTextRef.value
  grantOverflow.value = el ? el.scrollWidth > el.clientWidth : false
}

onMounted(() => nextTick(checkGrantOverflow))

watch(grantSummary, () => nextTick(checkGrantOverflow))
</script>

<template>
  <article
    class="trial-card group relative h-full w-full min-w-0"
    :class="{ 'trial-card--static': isUnavailable }"
  >
    <div class="trial-card-body relative flex h-full flex-col overflow-hidden rounded-2xl border border-[#eaedf1] bg-card">
      <div class="trial-ribbon pointer-events-none absolute -right-px -top-px z-20 h-6 w-[94px] overflow-hidden rounded-bl-2xl">
        <img :src="trialRibbon" alt="" class="block h-full w-full" aria-hidden="true">
        <span class="absolute left-[23px] top-1 max-w-[62px] truncate whitespace-nowrap text-xs font-medium leading-4 text-white">
          {{ t('billing.trialBadgeLabel') }}
        </span>
      </div>

      <div class="shrink-0 bg-card p-4">
        <div class="flex flex-col gap-6">
          <div v-if="primaryGrant" class="text-[#1f2329]">
            <div class="flex items-baseline gap-1">
              <span class="text-[40px] font-bold leading-[48px] tabular-nums">
                {{ primaryGrant.quantity }}
              </span>
              <span class="text-base font-medium leading-6">
                {{ subjectUnit(primaryGrant.subject) }}
              </span>
            </div>
            <p class="mt-0 truncate whitespace-nowrap text-xs font-medium leading-4">
              {{ subjectName(primaryGrant.subject) }}
            </p>
          </div>

          <div class="flex w-full flex-col gap-2">
            <div class="flex w-full items-center gap-[7px] rounded-[8px] bg-[#fff4e8] px-2 py-1">
              <Info class="size-3 shrink-0 text-[#ff7d00]" />
              <p class="min-w-0 truncate whitespace-nowrap text-xs font-medium leading-4 text-[#ff7d00]">
                {{ t('billing.trialPerUserLimit', { n: item.policy.per_user_limit }) }}
                <span v-if="item.claimed_count > 0">
                  · {{ t('billing.trialClaimedCount', { n: item.claimed_count }) }}
                </span>
              </p>
            </div>

            <Button
              v-if="item.claimable"
              class="h-auto w-full rounded-[8px] bg-primary px-3 py-[9px] text-sm font-medium text-primary-foreground hover:bg-[var(--primary-hover)]"
              :disabled="claiming"
              @click="emit('claim')"
            >
              {{ claiming ? t('billing.trialClaiming') : t('billing.trialClaim') }}
            </Button>
            <Button
              v-else-if="isClaimed"
              class="h-auto w-full rounded-[8px] bg-primary px-3 py-[9px] text-sm font-medium text-primary-foreground hover:bg-[var(--primary-hover)]"
              @click="emit('use')"
            >
              {{ t('billing.trialGoUse') }}
            </Button>
            <Button
              v-else
              variant="outline"
              disabled
              class="h-auto w-full rounded-[8px] px-3 py-[9px] text-sm font-medium"
            >
              {{ isUnavailable ? t('billing.trialUnavailable') : t('billing.trialNotClaimable') }}
            </Button>
          </div>
        </div>
      </div>

      <div class="trial-card-footer flex min-h-0 flex-1 flex-col gap-4 border-t border-dashed border-[#DDE2E9] bg-[#fafbfc] px-4 pb-6 pt-4 text-xs leading-4">
        <div class="space-y-0.5">
          <p class="font-medium text-[#646a73]">
            {{ t('billing.trialCouponId') }}
          </p>
          <p class="truncate text-[#8f959e]">
            {{ couponId }}
          </p>
        </div>

        <div class="space-y-0.5">
          <p class="font-medium text-[#646a73]">
            {{ t('billing.trialValidity') }}
          </p>
          <p class="truncate text-[#8f959e]">
            {{ validityText }}
          </p>
        </div>

        <div class="min-w-0 space-y-0.5">
          <p class="font-medium text-[#646a73]">
            {{ t('billing.trialGrantDesc') }}
          </p>
          <TooltipProvider :delay-duration="200">
            <Tooltip :disabled="!grantOverflow">
              <TooltipTrigger as-child>
                <p
                  ref="grantTextRef"
                  class="truncate whitespace-nowrap text-[#8f959e]"
                  :class="grantOverflow ? 'cursor-default' : ''"
                >
                  {{ grantSummary }}
                </p>
              </TooltipTrigger>
              <TooltipContent class="max-w-sm break-words text-xs">
                {{ grantSummary }}
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </div>

        <div v-if="descriptionText" class="space-y-0.5">
          <p class="font-medium text-[#646a73]">
            {{ t('billing.trialDescription') }}
          </p>
          <p class="truncate whitespace-nowrap text-[#8f959e]">
            {{ descriptionText }}
          </p>
        </div>
      </div>
    </div>
  </article>
</template>

<style scoped>
.trial-card-body {
  transition:
    border-color 0.2s ease,
    box-shadow 0.2s ease;
}

.trial-card:not(.trial-card--static):hover .trial-card-body {
  border-color: var(--primary);
  box-shadow: 0 12px 28px rgb(0 0 0 / 6%);
}
</style>
