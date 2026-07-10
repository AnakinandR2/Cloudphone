<script setup lang="ts">
import type { ClaimableItem } from '@/types/billing'
import { Gift } from 'lucide-vue-next'
import { useResizeObserver } from '@vueuse/core'
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import billingApi from '@/api/modules/billing'
import CountTabs from '@/components/CountTabs.vue'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useBillingStore } from '@/stores/billing'
import TrialCard from './trials/TrialCard.vue'

type TrialTab = 'pending' | 'claimed' | 'unavailable'

const TRIAL_CARD_MIN_WIDTH = 280
const TRIAL_CARD_MAX_WIDTH = 360
const TRIAL_GRID_GAP = 12
const TRIAL_ROWS_PER_PAGE = 2

const { t } = useI18n()
const router = useRouter()
const billingStore = useBillingStore()

const gridMeasureRef = ref<HTMLElement | null>(null)
const columnsPerRow = ref(4)
const cardWidth = ref(TRIAL_CARD_MIN_WIDTH)

const items = ref<ClaimableItem[]>([])
const loading = ref(false)
const claiming = ref<Record<string, boolean>>({})
const activeTab = ref<TrialTab>('pending')
const page = ref(1)

const SUBJECT_ORDER = ['seat', 'boot_slot', 'runtime_minute']

const pendingItems = computed(() => items.value.filter(it => it.claimable))
const claimedItems = computed(() => items.value.filter(it => it.claimed_count > 0 && !it.claimable))
const unavailableItems = computed(() => items.value.filter(it => !it.claimable && it.claimed_count === 0))

const filteredItems = computed(() => {
  if (activeTab.value === 'pending') return pendingItems.value
  if (activeTab.value === 'claimed') return claimedItems.value
  return unavailableItems.value
})

const pageSize = computed(() => columnsPerRow.value * TRIAL_ROWS_PER_PAGE)

const pageCount = computed(() => Math.max(1, Math.ceil(filteredItems.value.length / pageSize.value)))

const paginatedItems = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return filteredItems.value.slice(start, start + pageSize.value)
})

function updateGridLayout() {
  const width = gridMeasureRef.value?.clientWidth ?? 0
  if (!width) return

  const gap = TRIAL_GRID_GAP
  const minW = TRIAL_CARD_MIN_WIDTH
  const maxW = TRIAL_CARD_MAX_WIDTH

  let cols = Math.max(1, Math.floor((width + gap) / (minW + gap)))
  let w = (width - (cols - 1) * gap) / cols

  if (w > maxW) {
    const colsAtMax = Math.max(1, Math.ceil((width + gap) / (maxW + gap)))
    const wAtMaxCols = (width - (colsAtMax - 1) * gap) / colsAtMax
    if (wAtMaxCols >= minW) {
      cols = colsAtMax
      w = wAtMaxCols
    }
    else {
      cols = Math.max(1, Math.floor((width + gap) / (maxW + gap)))
      w = (width - (cols - 1) * gap) / cols
    }
  }

  let nextW = Math.floor(Math.min(maxW, Math.max(minW, w)))
  let total = cols * nextW + (cols - 1) * gap

  // 舍入后可能仍超出可用宽度，逐列减少直至放得下
  while (total > width && cols > 1) {
    cols -= 1
    nextW = Math.floor(Math.min(maxW, Math.max(minW, (width - (cols - 1) * gap) / cols)))
    total = cols * nextW + (cols - 1) * gap
  }
  while (total > width && nextW > minW) {
    nextW -= 1
    total = cols * nextW + (cols - 1) * gap
  }

  columnsPerRow.value = cols
  cardWidth.value = nextW
}

const gridStyle = computed(() => ({
  gridTemplateColumns: `repeat(${columnsPerRow.value}, ${cardWidth.value}px)`,
}))

useResizeObserver(gridMeasureRef, () => updateGridLayout())

const emptyText = computed(() => {
  if (activeTab.value === 'pending') return t('billing.trialsEmptyPending')
  if (activeTab.value === 'claimed') return t('billing.trialsEmptyClaimed')
  return t('billing.trialsEmptyUnavailable')
})

const trialTabItems = computed(() => [
  { value: 'pending', label: t('billing.trialsTabPending'), count: pendingItems.value.length },
  { value: 'claimed', label: t('billing.trialsTabClaimed'), count: claimedItems.value.length },
  { value: 'unavailable', label: t('billing.trialsTabUnavailable'), count: unavailableItems.value.length },
])

watch(activeTab, () => {
  page.value = 1
})

watch(pageCount, (count) => {
  if (page.value > count) page.value = count
})

watch(pageSize, () => {
  if (page.value > pageCount.value) page.value = pageCount.value
})

async function load() {
  loading.value = true
  try {
    const res = await billingApi.trials()
    items.value = res.data
    billingStore.claimableTrials = res.data.filter(it => it.claimable).length
  }
  catch {
    toast.error(t('billing.loadFailed'))
  }
  finally {
    loading.value = false
  }
}

onMounted(() => {
  load()
  nextTick(updateGridLayout)
})

function subjectName(subject: string): string {
  if (subject === 'seat') return t('billing.purchase2.kpiSeat')
  if (subject === 'boot_slot') return t('billing.purchase2.kpiBootSlot')
  if (subject === 'runtime_minute') return t('billing.purchase2.kpiRuntime')
  return subject
}

function subjectUnit(subject: string): string {
  if (subject === 'seat') return t('billing.trialUnitTai')
  if (subject === 'boot_slot') return t('billing.unitGe')
  if (subject === 'runtime_minute') return t('billing.unitMin')
  return ''
}

function subjectRank(s: string): number {
  const i = SUBJECT_ORDER.indexOf(s)
  return i < 0 ? 99 : i
}

function grantLines(item: ClaimableItem): string[] {
  return [...(item.policy.items ?? [])].sort((a, b) => subjectRank(a.subject) - subjectRank(b.subject)).map((it) => {
    const expire = it.expire_days > 0
      ? t('billing.trialExpireDays', { n: it.expire_days })
      : t('billing.trialPermanent')
    return `${subjectName(it.subject)} ${it.quantity} ${subjectUnit(it.subject)} · ${expire}`
  })
}

async function claim(item: ClaimableItem) {
  const code = item.policy.code
  claiming.value[code] = true
  try {
    await billingApi.claimTrial(code, '')
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

function prevPage() {
  if (page.value <= 1) return
  page.value--
}

function nextPage() {
  if (page.value >= pageCount.value) return
  page.value++
}

function useTrial() {
  router.push('/phone')
}
</script>

<template>
  <div class="-mx-6 -mt-6 flex min-h-0 flex-1 flex-col bg-background">
    <header class="border-border border-b px-8 py-5">
      <h1 class="text-foreground text-xl font-medium leading-7">
        {{ t('billing.trialsTitle') }}
      </h1>
      <p class="text-muted-foreground mt-1 text-sm leading-[22px]">
        {{ t('billing.trialsDesc') }}
      </p>
    </header>

    <div class="px-8 py-4">
      <CountTabs v-model="activeTab" :items="trialTabItems" />
    </div>

    <section class="min-w-0 overflow-x-hidden px-8 pb-8 pt-2">
      <div ref="gridMeasureRef" class="min-w-0 w-full">
      <div v-if="loading" class="trials-grid grid w-full min-w-0 items-stretch gap-3" :style="gridStyle">
        <div v-for="i in pageSize" :key="i" class="flex w-full min-w-0 flex-col overflow-hidden rounded-2xl border border-[#eaedf1] bg-card">
          <div class="space-y-6 bg-card p-4">
            <div class="space-y-1">
              <Skeleton class="h-12 w-24" />
              <Skeleton class="h-3 w-16" />
            </div>
            <div class="space-y-2">
              <Skeleton class="h-7 w-full rounded-lg" />
              <Skeleton class="h-10 w-full rounded-lg" />
            </div>
          </div>
          <div class="flex flex-1 flex-col gap-4 border-t border-dashed border-[#DDE2E9] bg-[#fafbfc] px-4 pb-6 pt-4">
            <div class="space-y-1">
              <Skeleton class="h-3 w-14" />
              <Skeleton class="h-3 w-full" />
            </div>
            <div class="space-y-1">
              <Skeleton class="h-3 w-12" />
              <Skeleton class="h-3 w-24" />
            </div>
            <div class="space-y-1">
              <Skeleton class="h-3 w-14" />
              <Skeleton class="h-3 w-full" />
            </div>
          </div>
        </div>
      </div>

      <template v-else>
        <div
          v-if="filteredItems.length === 0"
          class="text-muted-foreground flex flex-col items-center justify-center gap-3 py-20"
        >
          <Gift class="size-10 opacity-30" />
          <p class="text-sm">
            {{ emptyText }}
          </p>
        </div>

        <template v-else>
          <div class="trials-grid grid w-full min-w-0 items-stretch gap-3" :style="gridStyle">
            <TrialCard
              v-for="item in paginatedItems"
              :key="item.policy.code"
              :item="item"
              :claiming="Boolean(claiming[item.policy.code])"
              :subject-name="subjectName"
              :subject-unit="subjectUnit"
              :grant-lines="grantLines"
              @claim="claim(item)"
              @use="useTrial"
            />
          </div>

          <div class="mt-6 flex items-center justify-between gap-3">
            <span class="text-muted-foreground text-sm">
              {{ t('billing.trialsTotal', { total: filteredItems.length }) }}
            </span>
            <div class="flex items-center gap-2">
              <Button variant="outline" size="sm" :disabled="page <= 1" @click="prevPage">
                {{ t('billing.trialsPrev') }}
              </Button>
              <span class="text-muted-foreground text-sm tabular-nums">
                {{ page }} / {{ pageCount }}
              </span>
              <Button variant="outline" size="sm" :disabled="page >= pageCount" @click="nextPage">
                {{ t('billing.trialsNext') }}
              </Button>
            </div>
          </div>
        </template>
      </template>
      </div>
    </section>
  </div>
</template>