import { describe, expect, it } from 'vitest'
import { fmtCents, fmtDiscountBps } from './money'

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
