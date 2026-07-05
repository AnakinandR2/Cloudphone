<script setup lang="ts">
import type { Locale } from '@/locales'
import type {
  ColorScheme,
  MenuMode,
  PageTransition,
  ThemeColor,
} from '@/types/settings'
import { Check, Copy, Monitor, Moon, RotateCcw, SlidersHorizontal, Sun } from 'lucide-vue-next'
import { storeToRefs } from 'pinia'
import { computed } from 'vue'

import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { cn } from '@/lib/utils'
import { localeOptions } from '@/locales'
import { useSettingsStore } from '@/stores/settings'
import { copyText } from '@/utils/clipboard'

const route = useRoute()
// 远程控制弹窗页不显示右下角 tweak 悬浮按钮
const showTweak = computed(() => import.meta.env.DEV && route.name !== 'phoneRemote')
const store = useSettingsStore()
const { panelOpen: open } = storeToRefs(store)
const s = store.settings
const { t } = useI18n()

const themeColors: { key: ThemeColor, color: string }[] = [
  { key: 'green', color: '#22c55e' },
  { key: 'indigo', color: '#6366f1' },
  { key: 'violet', color: '#8b5cf6' },
  { key: 'blue', color: '#3b82f6' },
  { key: 'cyan', color: '#06b6d4' },
  { key: 'teal', color: '#14b8a6' },
  { key: 'emerald', color: '#10b981' },
  { key: 'rose', color: '#f43f5e' },
  { key: 'orange', color: '#f97316' },
  { key: 'amber', color: '#f59e0b' },
  { key: 'pink', color: '#ec4899' },
  { key: 'neutral', color: '#52525b' },
]

const schemes = computed<{ key: ColorScheme, label: string, icon: typeof Sun }[]>(() => [
  { key: 'light', label: t('settings.light'), icon: Sun },
  { key: 'dark', label: t('settings.dark'), icon: Moon },
  { key: 'system', label: t('settings.system'), icon: Monitor },
])

const menuModes = computed<{ key: MenuMode, label: string }[]>(() => [
  { key: 'double', label: t('settings.menuDouble') },
  { key: 'single', label: t('settings.menuSingle') },
])

const transitions = computed<{ key: PageTransition, label: string }[]>(() => [
  { key: 'none', label: t('settings.transNone') },
  { key: 'fade', label: t('settings.transFade') },
  { key: 'slide', label: t('settings.transSlide') },
  { key: 'slide-up', label: t('settings.transSlide-up') },
  { key: 'zoom', label: t('settings.transZoom') },
  { key: 'blur', label: t('settings.transBlur') },
])

const radii = [0, 0.25, 0.5, 0.75, 1]

function pickLocale(value: Locale) {
  s.locale = value
}

async function copyDefaults() {
  // 兼容非 HTTPS（CP-0046 / #41）：navigator.clipboard 不可用时回退 execCommand。
  if (await copyText(store.toDefaultsSnippet()))
    toast.success(t('settings.copied'), { description: t('settings.copiedDesc') })
  else
    toast.error(t('settings.copyFailed'))
}

function reset() {
  store.reset()
  toast.success(t('settings.resetDone'))
}
</script>

<template>
  <!-- 开发态：右下角 tweak 悬浮按钮 -->
  <button
    v-if="showTweak"
    class="group bg-primary text-primary-foreground fixed right-5 bottom-5 z-50 flex size-12 items-center justify-center rounded-full shadow-lg ring-1 ring-black/5 transition-all duration-200 hover:scale-105 hover:shadow-xl active:scale-95"
    :title="t('settings.title')"
    @click="open = true"
  >
    <SlidersHorizontal
      class="size-5 transition-transform duration-300 group-hover:rotate-90"
    />
  </button>

  <Sheet v-model:open="open">
    <SheetContent class="w-[340px] overflow-y-auto sm:max-w-none">
      <SheetHeader>
        <SheetTitle>{{ t('settings.title') }}</SheetTitle>
        <SheetDescription>{{ t('settings.desc') }}</SheetDescription>
      </SheetHeader>

      <div class="space-y-6 px-4 pb-4">
        <!-- 语言 -->
        <section class="space-y-2">
          <h4 class="text-sm font-medium">
            {{ t('settings.language') }}
          </h4>
          <div class="grid grid-cols-2 gap-2">
            <Button
              v-for="opt in localeOptions"
              :key="opt.value"
              :variant="s.locale === opt.value ? 'default' : 'outline'"
              size="sm"
              @click="pickLocale(opt.value)"
            >
              {{ opt.label }}
            </Button>
          </div>
        </section>

        <!-- 主题色 -->
        <section class="space-y-2">
          <h4 class="text-sm font-medium">
            {{ t('settings.themeColor') }}
          </h4>
          <div class="grid grid-cols-6 gap-2">
            <button
              v-for="c in themeColors"
              :key="c.key"
              class="relative flex aspect-square items-center justify-center rounded-lg ring-offset-2 ring-offset-background transition-all hover:scale-105"
              :class="s.themeColor === c.key ? 'ring-2 ring-ring' : ''"
              :style="{ backgroundColor: c.color }"
              :title="t(`settings.colors.${c.key}`)"
              @click="s.themeColor = c.key"
            >
              <Check
                v-if="s.themeColor === c.key"
                class="size-4 text-white drop-shadow"
              />
            </button>
          </div>
        </section>

        <!-- 明暗模式 -->
        <section class="space-y-2">
          <h4 class="text-sm font-medium">
            {{ t('settings.colorScheme') }}
          </h4>
          <div class="grid grid-cols-3 gap-2">
            <Button
              v-for="m in schemes"
              :key="m.key"
              :variant="s.colorScheme === m.key ? 'default' : 'outline'"
              size="sm"
              class="flex-col h-auto gap-1 py-2"
              @click="s.colorScheme = m.key"
            >
              <component :is="m.icon" class="size-4" />
              <span class="text-xs">{{ m.label }}</span>
            </Button>
          </div>
        </section>

        <!-- 菜单模式 -->
        <section class="space-y-2">
          <h4 class="text-sm font-medium">
            {{ t('settings.menuMode') }}
          </h4>
          <div class="grid gap-2">
            <Button
              v-for="m in menuModes"
              :key="m.key"
              :variant="s.menuMode === m.key ? 'default' : 'outline'"
              size="sm"
              class="justify-start"
              @click="s.menuMode = m.key"
            >
              {{ m.label }}
            </Button>
          </div>
        </section>

        <!-- 页面动效 -->
        <section class="space-y-2">
          <h4 class="text-sm font-medium">
            {{ t('settings.pageTransition') }}
          </h4>
          <div class="grid grid-cols-3 gap-2">
            <Button
              v-for="tr in transitions"
              :key="tr.key"
              :variant="s.pageTransition === tr.key ? 'default' : 'outline'"
              size="sm"
              @click="s.pageTransition = tr.key"
            >
              {{ tr.label }}
            </Button>
          </div>
        </section>

        <!-- 进度条 -->
        <section class="flex items-center justify-between">
          <h4 class="text-sm font-medium">
            {{ t('settings.progressBar') }}
          </h4>
          <button
            role="switch"
            :aria-checked="s.progressBar"
            :class="cn(
              'relative h-6 w-11 rounded-full transition-colors',
              s.progressBar ? 'bg-primary' : 'bg-input',
            )"
            @click="s.progressBar = !s.progressBar"
          >
            <span
              :class="cn(
                'absolute top-0.5 left-0.5 size-5 rounded-full bg-white transition-transform',
                s.progressBar && 'translate-x-5',
              )"
            />
          </button>
        </section>

        <!-- 圆角 -->
        <section class="space-y-2">
          <h4 class="text-sm font-medium">
            {{ t('settings.radius') }}
          </h4>
          <div class="grid grid-cols-5 gap-2">
            <Button
              v-for="r in radii"
              :key="r"
              :variant="s.radius === r ? 'default' : 'outline'"
              size="sm"
              @click="s.radius = r"
            >
              {{ r }}
            </Button>
          </div>
        </section>
      </div>

      <div class="mt-auto flex gap-2 border-t p-4">
        <Button class="flex-1" @click="copyDefaults">
          <Copy class="size-4" />
          {{ t('settings.copyDefaults') }}
        </Button>
        <Button variant="outline" @click="reset">
          <RotateCcw class="size-4" />
          {{ t('settings.reset') }}
        </Button>
      </div>
    </SheetContent>
  </Sheet>
</template>
