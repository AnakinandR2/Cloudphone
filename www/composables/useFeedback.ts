// 文章反馈组合式：访客身份/meta 的纯函数（可单测）+ 命中本站 /_content/feedback/* 的调用封装。
// 密钥只在服务端；这里只调用本站 BFF。
import type { AuthUser } from '~/composables/useAuthUser'
import type { FeedbackSummary } from '~/types/content'

const VISITOR_KEY = 'gp_visitor_id'

// ---------------------------------------------------------------------------
// 纯函数辅助（无副作用，便于单测）
// ---------------------------------------------------------------------------

/** 生成一个匿名访客 id（浏览器优先用 crypto.randomUUID）。 */
export function genVisitorId(): string {
  try {
    if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID()
  } catch { /* 忽略，走回退 */ }
  return 'v-' + Math.random().toString(36).slice(2) + Date.now().toString(36)
}

/**
 * 决定本次使用的 visitor_id。
 *  - 已登录：`u:<id>`（跨设备稳定、同一人去重），不落 localStorage（可由用户派生）；
 *  - 未登录：复用已存 id；都没有则用 gen() 新生成并标记需持久化。
 */
export function buildVisitorId(
  user: Pick<AuthUser, 'id'> | null,
  stored: string | null,
  gen: () => string = genVisitorId,
): { id: string; persist: boolean } {
  if (user) return { id: `u:${user.id}`, persist: false }
  if (stored) return { id: stored, persist: false }
  return { id: gen(), persist: true }
}

/**
 * 登录用户的「业务自定义标识 meta」——中台 meta 是字符串，故 JSON 序列化。
 * 未登录返回 undefined（不带 meta）。
 */
export function buildMeta(user: Pick<AuthUser, 'id' | 'nickname' | 'phone'> | null): string | undefined {
  if (!user) return undefined
  return JSON.stringify({ user_id: user.id, nickname: user.nickname || '', phone: user.phone || '' })
}

/** 赞踩切换：再次点击同向 → 0（撤销），否则 → 点击值。 */
export function nextVote(current: number, clicked: 1 | -1): number {
  return current === clicked ? 0 : clicked
}

// ---------------------------------------------------------------------------
// 客户端：访客 id 解析（读/写 localStorage）
// ---------------------------------------------------------------------------

/** 解析当前访客 id；匿名首访时生成并持久化。仅客户端调用。 */
export function ensureVisitorId(user: AuthUser | null): string {
  let stored: string | null = null
  try { stored = localStorage.getItem(VISITOR_KEY) } catch { /* 隐私模式等，忽略 */ }
  const { id, persist } = buildVisitorId(user, stored)
  if (persist) {
    try { localStorage.setItem(VISITOR_KEY, id) } catch { /* 忽略 */ }
  }
  return id
}

// ---------------------------------------------------------------------------
// 调用封装（命中本站 BFF）
// ---------------------------------------------------------------------------

export function useFeedbackApi(space: string, slug: () => string) {
  const { locale } = useI18n()
  const base = () => `/_content/feedback/${encodeURIComponent(slug())}`

  function fetchSummary(visitorId: string) {
    return $fetch<FeedbackSummary>(`${base()}/summary`, {
      query: { space, lang: locale.value, visitor_id: visitorId },
    })
  }
  function submitReaction(p: { vote?: number; rating?: number; visitorId: string; meta?: string }) {
    return $fetch<FeedbackSummary>(`${base()}/reaction`, {
      method: 'POST',
      query: { space },
      body: { vote: p.vote, rating: p.rating, visitor_id: p.visitorId, lang: locale.value, meta: p.meta },
    })
  }
  function submitFeedback(p: { content: string; contact?: string; visitorId: string; meta?: string }) {
    return $fetch<FeedbackSummary>(`${base()}/feedback`, {
      method: 'POST',
      query: { space },
      body: { content: p.content, contact: p.contact, visitor_id: p.visitorId, lang: locale.value, meta: p.meta },
    })
  }
  return { fetchSummary, submitReaction, submitFeedback }
}
