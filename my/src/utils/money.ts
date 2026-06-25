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

// 外加手续费 = 比例(基数×bps，四舍五入到分) + 固定。与后端 computeFee 同公式同舍入。
// 基数<=0（含 0 元订单）返回 0（含固定部分一并不收）。
// 比例用 Math.round(base*bps/10000) 与后端 applyBps(round-half-up) 对齐。
export function computeFeeCents(baseCents: number, percentBps: number, fixedCents: number): number {
  if (baseCents <= 0) return 0
  return Math.round((baseCents * percentBps) / 10000) + fixedCents
}

// 满额免手续费谓词：阈值>0 且 基数>=阈值。
// 与后端 computeFee 的免除分支同语义；base<=0 时无手续费、本就不涉及，此处仍按字面返回（threshold>0 且 0>=threshold 不成立 → false）。
export function feeWaived(baseCents: number, freeThresholdCents: number): boolean {
  return freeThresholdCents > 0 && baseCents >= freeThresholdCents
}

// 手续费构成标注，如「2% + ¥1」；任一项为 0 只显示另一项；都为 0 返回空串。
export function fmtFeeHint(percentBps: number, fixedCents: number): string {
  const parts: string[] = []
  if (percentBps > 0) parts.push(`${+(percentBps / 100).toFixed(2)}%`)
  if (fixedCents > 0) parts.push(`¥${fmtCents(fixedCents)}`)
  return parts.join(' + ')
}
