/**
 * 日期时间格式化工具（原生实现，按本地时区展示）。
 * 兼容后端返回的 ISO 串（含纳秒/时区，如 2026-06-02T04:19:27.285373117+08:00）。
 */

function toDate(date: Date | number | string): Date | null {
  const d = typeof date === 'string' || typeof date === 'number' ? new Date(date) : date
  if (!(d instanceof Date) || Number.isNaN(d.getTime())) {
    return null
  }
  return d
}

/** YYYY-MM-DD HH:mm */
export function formatDateTime(date?: Date | number | string | null): string {
  if (!date) return '-'
  const d = toDate(date)
  if (!d) return '-'
  const Y = d.getFullYear()
  const M = String(d.getMonth() + 1).padStart(2, '0')
  const D = String(d.getDate()).padStart(2, '0')
  const h = String(d.getHours()).padStart(2, '0')
  const m = String(d.getMinutes()).padStart(2, '0')
  return `${Y}-${M}-${D} ${h}:${m}`
}

/** YYYY-MM-DD HH:mm:ss */
export function formatDateTimeFull(date?: Date | number | string | null): string {
  if (!date) return '-'
  const d = toDate(date)
  if (!d) return '-'
  const Y = d.getFullYear()
  const M = String(d.getMonth() + 1).padStart(2, '0')
  const D = String(d.getDate()).padStart(2, '0')
  const h = String(d.getHours()).padStart(2, '0')
  const m = String(d.getMinutes()).padStart(2, '0')
  const s = String(d.getSeconds()).padStart(2, '0')
  return `${Y}-${M}-${D} ${h}:${m}:${s}`
}

/** YYYY-MM-DD */
export function formatDate(date?: Date | number | string | null): string {
  if (!date) return '-'
  const d = toDate(date)
  if (!d) return '-'
  const Y = d.getFullYear()
  const M = String(d.getMonth() + 1).padStart(2, '0')
  const D = String(d.getDate()).padStart(2, '0')
  return `${Y}-${M}-${D}`
}

/** 相对时间（刚刚 / N 分钟前 …），超过 7 天回退到 formatDateTime */
export function formatRelativeTime(date?: Date | number | string | null): string {
  if (!date) return '-'
  const d = toDate(date)
  if (!d) return '-'
  const diffSec = Math.floor((Date.now() - d.getTime()) / 1000)
  if (diffSec < 60) return '刚刚'
  const diffMin = Math.floor(diffSec / 60)
  if (diffMin < 60) return `${diffMin} 分钟前`
  const diffHour = Math.floor(diffMin / 60)
  if (diffHour < 24) return `${diffHour} 小时前`
  const diffDay = Math.floor(diffHour / 24)
  if (diffDay < 7) return `${diffDay} 天前`
  return formatDateTime(d)
}
