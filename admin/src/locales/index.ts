import { createI18n } from 'vue-i18n'

import { defaultSettings } from '@/settings'
import en from './en'
import zhCN from './zh-CN'

export type Locale = 'zh-CN' | 'en'

export const localeOptions: { value: Locale, label: string }[] = [
  { value: 'zh-CN', label: '简体中文' },
  { value: 'en', label: 'English' },
]

const PREFIX = import.meta.env.VITE_APP_STORAGE_PREFIX || 'admin'

/** 从已保存的设置里取初始语言，否则用默认 */
function initialLocale(): Locale {
  try {
    const raw = localStorage.getItem(`${PREFIX}_settings`)
    if (raw) {
      const s = JSON.parse(raw)
      if (s.locale) return s.locale
    }
  }
  catch {
    /* ignore */
  }
  return defaultSettings.locale
}

export const i18n = createI18n({
  legacy: false,
  locale: initialLocale(),
  fallbackLocale: 'zh-CN',
  messages: {
    'zh-CN': zhCN,
    'en': en,
  },
})

/** 在组件外使用的翻译函数（路由守卫、工具函数等） */
export function t(key: string, named?: Record<string, unknown>) {
  return i18n.global.t(key, named ?? {})
}

export function setI18nLocale(locale: Locale) {
  i18n.global.locale.value = locale
  document.documentElement.lang = locale
}
