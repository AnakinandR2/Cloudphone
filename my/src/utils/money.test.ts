import { describe, expect, it } from 'vitest'
import { computeFeeCents, fmtCents, fmtDiscountBps, fmtFeeHint } from './money'

describe('money', () => {
  it('分→元两位小数', () => {
    expect(fmtCents(0)).toBe('0.00')
    expect(fmtCents(3000)).toBe('30.00')
    expect(fmtCents(12650)).toBe('126.50')
  })
  it('折扣 bps→折', () => {
    expect(fmtDiscountBps(10000)).toBe('') // 无折扣不显示
    expect(fmtDiscountBps(8500)).toBe('8.5折')
    expect(fmtDiscountBps(7000)).toBe('7折')
  })
})

describe('computeFeeCents（与后端 computeFee 同公式同舍入）', () => {
  it('纯比例：2% of ¥50 = ¥1', () => {
    expect(computeFeeCents(5000, 200, 0)).toBe(100)
  })
  it('纯固定：¥1', () => {
    expect(computeFeeCents(5000, 0, 100)).toBe(100)
  })
  it('叠加：2% + ¥1，面额 ¥100 → ¥3', () => {
    expect(computeFeeCents(10000, 200, 100)).toBe(300)
  })
  it('全 0 = 无手续费', () => {
    expect(computeFeeCents(10000, 0, 0)).toBe(0)
  })
  it('舍入边界：四舍五入到分（奇数分进位）', () => {
    // 333 * 200 / 10000 = 6.66 → 7
    expect(computeFeeCents(333, 200, 0)).toBe(7)
    // 25 * 200 / 10000 = 0.5 → 1（round-half-up）
    expect(computeFeeCents(25, 200, 0)).toBe(1)
  })
  it('基数 <= 0 一律返回 0（含固定部分）', () => {
    expect(computeFeeCents(0, 200, 100)).toBe(0)
    expect(computeFeeCents(-100, 200, 100)).toBe(0)
  })
})

describe('fmtFeeHint（手续费构成标注）', () => {
  it('比例 + 固定叠加', () => {
    expect(fmtFeeHint(200, 100)).toBe('2% + ¥1.00')
  })
  it('仅比例', () => {
    expect(fmtFeeHint(250, 0)).toBe('2.5%')
  })
  it('仅固定', () => {
    expect(fmtFeeHint(0, 100)).toBe('¥1.00')
  })
  it('都为 0 返回空串', () => {
    expect(fmtFeeHint(0, 0)).toBe('')
  })
})
