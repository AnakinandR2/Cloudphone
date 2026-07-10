<script setup lang="ts">
import type { LibraryOverview } from '@/types/library'
import { Add2 as Plus, AlertTriangle } from 'reicon-vue'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import libraryApi from '@/api/modules/library'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '@/components/ui/tabs'
import { fmtBytes } from '@/utils/bytes'
import PackagePanel from '@/views/library/PackagePanel.vue'
import MarketPanel from './MarketPanel.vue'
import MyAppsPanel from './MyAppsPanel.vue'
import MyFilesPanel from './MyFilesPanel.vue'

const { t } = useI18n()

const overview = ref<LibraryOverview | null>(null)
const packageOpen = ref(false)

const locked = computed(() => overview.value?.usage.locked ?? false)
const usagePct = computed(() => {
  const u = overview.value?.usage
  if (!u || u.capacity_bytes <= 0)
    return 0
  return Math.min(100, Math.round((u.used_bytes / u.capacity_bytes) * 100))
})

async function loadOverview() {
  try {
    overview.value = (await libraryApi.overview()).data
  }
  catch {
    // 拦截器提示
  }
}

function onPackagePaid() {
  packageOpen.value = false
  loadOverview()
}

onMounted(loadOverview)
</script>

<template>
  <div class="flex h-full min-h-0 min-w-0 flex-col overflow-hidden">
    <Card class="flex min-h-0 flex-1 flex-col gap-4 overflow-hidden py-5">
      <CardHeader class="shrink-0 space-y-3">
        <div class="flex min-w-0 items-center justify-between gap-4">
          <div class="min-w-0 space-y-1.5">
            <CardTitle>{{ t('assets.title') }}</CardTitle>
            <CardDescription>
              {{ t('assets.desc') }}
              <span
                v-if="overview"
                class="text-foreground/70 tabular-nums"
              >
                · {{ fmtBytes(overview.usage.used_bytes) }} / {{ fmtBytes(overview.usage.capacity_bytes) }}
              </span>
            </CardDescription>
          </div>
          <div class="flex shrink-0">
            <Button
              class="h-[52px] gap-2.5 rounded-xl px-[32px] has-[>svg]:px-[32px] text-lg font-semibold shadow-[var(--btn-shadow-hover)]"
              @click="packageOpen = true"
            >
              <Plus class="size-6 shrink-0" stroke-width="2.5" />
              {{ t('library.managePlan') }}
            </Button>
          </div>
        </div>
        <div v-if="overview" class="bg-muted h-2 w-full overflow-hidden rounded-full">
          <div
            class="h-full rounded-full transition-all"
            :class="locked ? 'bg-red-500' : usagePct > 85 ? 'bg-amber-500' : 'bg-primary'"
            :style="{ width: `${usagePct}%` }"
          />
        </div>
      </CardHeader>

      <CardContent class="flex min-h-0 min-w-0 flex-1 flex-col items-stretch overflow-auto">
        <!-- 锁定横幅 -->
        <div
          v-if="locked"
          class="mb-4 flex items-start gap-3 rounded-xl border border-red-300 bg-red-50 p-4 text-sm dark:border-red-900 dark:bg-red-950/40"
        >
          <AlertTriangle class="mt-0.5 size-5 shrink-0 text-red-500" />
          <div class="flex flex-col gap-1">
            <span class="font-medium text-red-700 dark:text-red-300">{{ t('library.lockedTitle') }}</span>
            <span class="text-red-600/90 dark:text-red-400/90">{{ t('library.lockedDesc') }}</span>
            <div class="mt-1">
              <Button size="sm" variant="outline" class="border-red-300 text-red-700 dark:text-red-300" @click="packageOpen = true">
                {{ t('library.expandNow') }}
              </Button>
            </div>
          </div>
        </div>

        <!-- 三个标签页：我的应用 / 应用市场 / 我的素材 -->
        <Tabs default-value="apps" class="flex min-h-0 flex-1 flex-col gap-4">
          <TabsList class="w-full shrink-0 sm:w-auto sm:self-start">
            <TabsTrigger value="apps">
              {{ t('assets.tabApps') }}
            </TabsTrigger>
            <TabsTrigger value="market">
              {{ t('assets.tabMarket') }}
            </TabsTrigger>
            <TabsTrigger value="files">
              {{ t('assets.tabFiles') }}
            </TabsTrigger>
          </TabsList>

          <TabsContent value="apps" class="min-h-0 flex-1 outline-none">
            <MyAppsPanel :overview="overview" @changed="loadOverview" />
          </TabsContent>
          <TabsContent value="market" class="min-h-0 flex-1 outline-none">
            <MarketPanel />
          </TabsContent>
          <TabsContent value="files" class="min-h-0 flex-1 outline-none">
            <MyFilesPanel :overview="overview" @changed="loadOverview" />
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>

    <!-- 套餐购买抽屉（复用素材库面板）-->
    <Sheet v-model:open="packageOpen">
      <SheetContent class="w-full overflow-y-auto sm:max-w-xl">
        <SheetHeader>
          <SheetTitle>{{ t('library.package.title') }}</SheetTitle>
          <SheetDescription>{{ t('library.package.subtitle') }}</SheetDescription>
        </SheetHeader>
        <div class="px-4 pb-6">
          <PackagePanel v-if="overview" :overview="overview" @paid="onPackagePaid" />
        </div>
      </SheetContent>
    </Sheet>
  </div>
</template>
