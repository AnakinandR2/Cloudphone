export interface AccentOption {
  value: string
  /** label per locale */
  name: { zh: string; en: string }
  /** color used for the swatch preview */
  swatch: string
}

export const ACCENTS: AccentOption[] = [
  { value: 'purple', name: { zh: '深紫', en: 'Violet' }, swatch: '#a855f7' },
  { value: 'blue', name: { zh: '海蓝', en: 'Blue' }, swatch: '#3b82f6' },
  { value: 'teal', name: { zh: '薄荷', en: 'Teal' }, swatch: '#14b8a6' },
  { value: 'green', name: { zh: '翠绿', en: 'Green' }, swatch: '#22c55e' },
  { value: 'orange', name: { zh: '落日', en: 'Orange' }, swatch: '#f97316' },
  { value: 'pink', name: { zh: '霓粉', en: 'Pink' }, swatch: '#ec4899' },
  { value: 'indigo', name: { zh: '靛青', en: 'Indigo' }, swatch: '#6366f1' },
  { value: 'rose', name: { zh: '朱砂', en: 'Rose' }, swatch: '#f43f5e' },
]

const DEFAULT_ACCENT = 'teal'

/**
 * Swappable theme (accent) color. Persists the choice in a cookie and reflects
 * it on <html data-accent="..."> so the CSS variable presets take effect.
 */
export function useThemeColor() {
  const accent = useCookie<string>('gp-accent', {
    default: () => DEFAULT_ACCENT,
    maxAge: 60 * 60 * 24 * 365,
    sameSite: 'lax',
  })

  // SSR + 客户端都把当前 accent 反映到 <html data-accent>，让 CSS 预设在首屏即生效（无闪色）。
  // useHead 的 htmlAttrs 是响应式的：切换 accent 时 accent ref 变化，html 属性自动更新。
  useHead({
    htmlAttrs: {
      'data-accent': accent,
    },
  })

  function setAccent(value: string) {
    accent.value = value
  }

  return { accent, accents: ACCENTS, setAccent }
}
