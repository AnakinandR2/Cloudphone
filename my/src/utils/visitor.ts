// 访客软标识：在 localStorage 持久化一个匿名 uuid，用于推广点击归因。
// 即便用户未登录（含营销站游客）也能稳定标识同一浏览器。
const PREFIX = import.meta.env.VITE_APP_STORAGE_PREFIX || 'my'
const KEY = `${PREFIX}_anonymous_id`

function genUUID(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function')
    return crypto.randomUUID()
  // 退化实现（极老浏览器）：足够做去标识。
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0
    const v = c === 'x' ? r : (r & 0x3) | 0x8
    return v.toString(16)
  })
}

/** 取（无则生成并持久化）当前浏览器的匿名访客标识。 */
export function getAnonymousId(): string {
  try {
    let id = localStorage.getItem(KEY)
    if (!id) {
      id = genUUID()
      localStorage.setItem(KEY, id)
    }
    return id
  }
  catch {
    // localStorage 不可用（隐私模式等）：返回临时 id，不持久化。
    return genUUID()
  }
}
