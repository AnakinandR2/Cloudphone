import { describe, expect, it } from 'vitest'
import { fmtCents, fmtDiscountBps, validateAdjustYuan } from './money'

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

// CP-0070 / #59：余额调整金额校验——超两位小数拒绝，绝不静默截断/进位；允许负数（扣减/退款）。
describe('validateAdjustYuan', () => {
  it('两位内小数（正/负）通过并转整数分', () => {
    expect(validateAdjustYuan(0.11)).toEqual({ cents: 11, error: '' })
    expect(validateAdjustYuan(50)).toEqual({ cents: 5000, error: '' })
    expect(validateAdjustYuan(-12.34)).toEqual({ cents: -1234, error: '' }) // 负数=扣减，允许
  })
  it('超两位小数 → precision（不再静默截断为 0.11 或进位为 0.12）', () => {
    expect(validateAdjustYuan(0.11231231)).toEqual({ cents: 0, error: 'precision' })
    expect(validateAdjustYuan(0.116)).toEqual({ cents: 0, error: 'precision' })
    expect(validateAdjustYuan(-1.005)).toEqual({ cents: 0, error: 'precision' })
  })
  it('空/NaN/0 → required', () => {
    expect(validateAdjustYuan(undefined)).toEqual({ cents: 0, error: 'required' })
    expect(validateAdjustYuan(null)).toEqual({ cents: 0, error: 'required' })
    expect(validateAdjustYuan(Number.NaN)).toEqual({ cents: 0, error: 'required' })
    expect(validateAdjustYuan(0)).toEqual({ cents: 0, error: 'required' })
  })
})
