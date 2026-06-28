// 字节 → 人类可读（1024 进制）。
const UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']

export function fmtBytes(bytes: number, fractionDigits = 1): string {
  if (!Number.isFinite(bytes) || bytes <= 0)
    return '0 B'
  let i = 0
  let n = bytes
  while (n >= 1024 && i < UNITS.length - 1) {
    n /= 1024
    i++
  }
  // 字节单位不显示小数。
  const digits = i === 0 ? 0 : fractionDigits
  return `${n.toFixed(digits)} ${UNITS[i]}`
}
