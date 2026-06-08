// 整数分 → 元（两位小数，千分位）
export function fmtCents(cents: number): string {
  return (cents / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

// 折扣基点(10000=全价) → 「8.5折」；全价返回空串
export function fmtDiscountBps(bps: number): string {
  if (bps >= 10000) return ''
  const zhe = +(bps / 1000).toFixed(1) // 8500 → 8.5；整数自动去掉 .0（7000→7）
  return `${zhe}折`
}
