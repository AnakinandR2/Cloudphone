// Cookie 同意状态：用 JSON cookie（gp-consent）落库；useCookie 让 SSR 与客户端共享同一值。
// - 未做选择时 value=null，模板据此显示底部 banner。
// - 做过选择后 banner 自动消失；用户后续可在 /cookies 页改偏好。
// - 「必要」类别永远 true，不暴露开关；点「全部拒绝」也保留它。

export type ConsentCategory = 'necessary' | 'functional' | 'analytics' | 'marketing'

export interface ConsentValue {
  v: number // schema 版本；加新类别时 bump，旧记录视为未决策
  ts: number // 决策时间戳（ms）
  necessary: true
  functional: boolean
  analytics: boolean
  marketing: boolean
}

export const CONSENT_VERSION = 1
const COOKIE_NAME = 'gp-consent'
const COOKIE_MAX_AGE = 60 * 60 * 24 * 180 // 半年；过期重新征求

function nowConsent(opts: Partial<Omit<ConsentValue, 'v' | 'ts' | 'necessary'>>): ConsentValue {
  return {
    v: CONSENT_VERSION,
    ts: Date.now(),
    necessary: true,
    functional: opts.functional ?? false,
    analytics: opts.analytics ?? false,
    marketing: opts.marketing ?? false,
  }
}

export function useCookieConsent() {
  const cookie = useCookie<ConsentValue | null>(COOKIE_NAME, {
    default: () => null,
    maxAge: COOKIE_MAX_AGE,
    sameSite: 'lax',
  })

  const consent = computed<ConsentValue | null>(() => {
    const v = cookie.value
    if (!v || typeof v !== 'object' || v.v !== CONSENT_VERSION)
      return null
    return v
  })

  const hasDecided = computed(() => consent.value !== null)

  const allowed = (cat: ConsentCategory) => {
    if (cat === 'necessary')
      return true
    return !!consent.value && !!consent.value[cat]
  }

  function acceptAll() {
    cookie.value = nowConsent({ functional: true, analytics: true, marketing: true })
  }
  function rejectAll() {
    cookie.value = nowConsent({ functional: false, analytics: false, marketing: false })
  }
  function savePreferences(prefs: Partial<Pick<ConsentValue, 'functional' | 'analytics' | 'marketing'>>) {
    cookie.value = nowConsent({
      functional: prefs.functional ?? consent.value?.functional ?? false,
      analytics: prefs.analytics ?? consent.value?.analytics ?? false,
      marketing: prefs.marketing ?? consent.value?.marketing ?? false,
    })
  }
  function reset() {
    cookie.value = null
  }

  return { consent, hasDecided, allowed, acceptAll, rejectAll, savePreferences, reset }
}
