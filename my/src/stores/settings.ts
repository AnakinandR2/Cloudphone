import type { AppSettings } from '@/types/settings'
import { defineStore } from 'pinia'

import { computed, reactive, ref, watch } from 'vue'
import { setI18nLocale } from '@/locales'
import { defaultSettings } from '@/settings'

const PREFIX = import.meta.env.VITE_APP_STORAGE_PREFIX || 'admin'
const STORAGE_KEY = `${PREFIX}_settings`

function load(): AppSettings {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) {
      return { ...defaultSettings, ...JSON.parse(raw) }
    }
  }
  catch {
    /* ignore */
  }
  return { ...defaultSettings }
}

let mediaQuery: MediaQueryList | null = null

export const useSettingsStore = defineStore('settings', () => {
  const settings = reactive<AppSettings>(load())

  /** 偏好设置面板开关（供账号菜单等处打开） */
  const panelOpen = ref(false)
  function openPanel() {
    panelOpen.value = true
  }

  function applyColorScheme() {
    const isDark
      = settings.colorScheme === 'dark'
        || (settings.colorScheme === 'system'
          && window.matchMedia('(prefers-color-scheme: dark)').matches)
    document.documentElement.classList.toggle('dark', isDark)
  }

  /** 跟随系统：监听系统明暗变化 */
  function bindSystemListener() {
    if (mediaQuery) {
      mediaQuery.onchange = null
    }
    if (settings.colorScheme === 'system') {
      mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
      mediaQuery.onchange = () => applyColorScheme()
    }
  }

  /** 把设置应用到 <html> */
  function apply() {
    const html = document.documentElement
    html.dataset.theme = settings.themeColor
    html.style.setProperty('--radius', `${settings.radius}rem`)
    setI18nLocale(settings.locale)
    applyColorScheme()
    bindSystemListener()
  }

  const isDark = computed(
    () =>
      settings.colorScheme === 'dark'
      || (settings.colorScheme === 'system'
        && typeof window !== 'undefined'
        && window.matchMedia('(prefers-color-scheme: dark)').matches),
  )

  /** 头部按钮：在 亮/暗 之间切换（明确指定，不走 system） */
  function toggleDark() {
    settings.colorScheme = isDark.value ? 'light' : 'dark'
  }

  function reset() {
    Object.assign(settings, defaultSettings)
  }

  /** 生成可粘贴回 src/settings.ts 的默认配置代码 */
  function toDefaultsSnippet(): string {
    const body = Object.entries(settings)
      .map(([k, v]) => `  ${k}: ${JSON.stringify(v)},`)
      .join('\n')
    return `export const defaultSettings: AppSettings = {\n${body}\n}\n`
  }

  // 任意设置变化 → 持久化 + 重新应用
  watch(
    settings,
    () => {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(settings))
      apply()
    },
    { deep: true },
  )

  return { settings, panelOpen, openPanel, isDark, apply, reset, toggleDark, toDefaultsSnippet }
})
