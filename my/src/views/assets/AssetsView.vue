<script setup lang="ts">
import type { LibraryOverview } from '@/types/library'
import { AlertTriangle } from 'lucide-vue-next'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import libraryApi from '@/api/modules/library'
import { Button } from '@/components/ui/button'
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
  <div class="flex flex-col gap-4">
    <!-- 顶部：容量条 + 扩容入口（应用与素材共享）-->
    <div class="bg-card flex flex-col gap-3 rounded-xl border p-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex items-baseline gap-2">
          <h1 class="text-lg font-semibold">
            {{ t('assets.title') }}
          </h1>
          <span v-if="overview" class="text-muted-foreground text-sm tabular-nums">
            {{ fmtBytes(overview.usage.used_bytes) }} / {{ fmtBytes(overview.usage.capacity_bytes) }}
          </span>
        </div>
        <Button variant="outline" size="sm" @click="packageOpen = true">
          {{ t('library.managePlan') }}
        </Button>
      </div>
      <div v-if="overview" class="bg-muted h-2 w-full overflow-hidden rounded-full">
        <div
          class="h-full rounded-full transition-all"
          :class="locked ? 'bg-red-500' : usagePct > 85 ? 'bg-amber-500' : 'bg-primary'"
          :style="{ width: `${usagePct}%` }"
        />
      </div>
    </div>

    <!-- 锁定横幅 -->
    <div
      v-if="locked"
      class="flex items-start gap-3 rounded-xl border border-red-300 bg-red-50 p-4 text-sm dark:border-red-900 dark:bg-red-950/40"
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
    <Tabs default-value="apps" class="flex flex-col gap-4">
      <TabsList class="w-full sm:w-auto sm:self-start">
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

      <TabsContent value="apps">
        <MyAppsPanel :overview="overview" @changed="loadOverview" />
      </TabsContent>
      <TabsContent value="market">
        <MarketPanel />
      </TabsContent>
      <TabsContent value="files">
        <MyFilesPanel :overview="overview" @changed="loadOverview" />
      </TabsContent>
    </Tabs>

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
