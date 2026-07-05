// 整数分 → 元（两位小数，千分位）
export function fmtCents(cents: number): string {
  return (cents / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

export type AdjustAmountError = '' | 'required' | 'precision'

// 校验「余额调整金额(元)」并转成整数分（CP-0070 / #59）：
// - 空/NaN/结果为 0 分 → required；
// - 超过两位小数（精确到分）→ precision（拒绝，绝不静默截断成 x.xx 或四舍五入进位）；
// - 允许负数（负数=扣减/退款，是余额调整的正常入口，与充值不同）。
// error==='' 时 cents 为有效整数分（可正可负）；否则 cents=0。纯函数，便于单测。
export function validateAdjustYuan(yuan: number | undefined | null): { cents: number, error: AdjustAmountError } {
  if (yuan == null || Number.isNaN(yuan))
    return { cents: 0, error: 'required' }
  const cents = yuan * 100
  // 浮点容差内非整数分 → 判定为超过两位小数（如 0.11231231 → 11.23…、0.116 → 11.6）。
  if (Math.abs(cents - Math.round(cents)) > 1e-6)
    return { cents: 0, error: 'precision' }
  const rounded = Math.round(cents)
  if (rounded === 0)
    return { cents: 0, error: 'required' } // 0 元不允许调整（与后端「调整金额不能为0」一致）
  return { cents: rounded, error: '' }
}

// 折扣基点(10000=全价) → 「8.5折」；全价返回空串
export function fmtDiscountBps(bps: number): string {
  if (bps >= 10000) return ''
  const zhe = +(bps / 1000).toFixed(1) // 8500 → 8.5；整数自动去掉 .0（7000→7）
  return `${zhe}折`
}
