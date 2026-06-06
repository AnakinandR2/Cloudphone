import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  formatDate,
  formatDateTime,
  formatDateTimeFull,
  formatRelativeTime,
} from './date'

// 用本地时区构造 Date（月份 0-based），再按本地时区读回，规避 TZ 差异
const sample = new Date(2026, 5, 2, 4, 9, 7) // 2026-06-02 04:09:07

describe('formatDateTime', () => {
  it('格式化为 YYYY-MM-DD HH:mm（补零）', () => {
    expect(formatDateTime(sample)).toBe('2026-06-02 04:09')
  })

  it('空值 / 非法日期返回 "-"', () => {
    expect(formatDateTime(null)).toBe('-')
    expect(formatDateTime(undefined)).toBe('-')
    expect(formatDateTime('')).toBe('-')
    expect(formatDateTime('not-a-date')).toBe('-')
  })

  it('接受时间戳与字符串', () => {
    expect(formatDateTime(sample.getTime())).toBe('2026-06-02 04:09')
  })
})

describe('formatDateTimeFull', () => {
  it('格式化为 YYYY-MM-DD HH:mm:ss', () => {
    expect(formatDateTimeFull(sample)).toBe('2026-06-02 04:09:07')
  })

  it('非法返回 "-"', () => {
    expect(formatDateTimeFull('xxx')).toBe('-')
  })
})

describe('formatDate', () => {
  it('格式化为 YYYY-MM-DD', () => {
    expect(formatDate(sample)).toBe('2026-06-02')
  })
})

describe('formatRelativeTime', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 5, 2, 12, 0, 0))
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  function ago(ms: number) {
    return new Date(Date.now() - ms)
  }

  it('小于 1 分钟 → 刚刚', () => {
    expect(formatRelativeTime(ago(30 * 1000))).toBe('刚刚')
  })

  it('分钟级', () => {
    expect(formatRelativeTime(ago(5 * 60 * 1000))).toBe('5 分钟前')
  })

  it('小时级', () => {
    expect(formatRelativeTime(ago(3 * 60 * 60 * 1000))).toBe('3 小时前')
  })

  it('天级（<7 天）', () => {
    expect(formatRelativeTime(ago(2 * 24 * 60 * 60 * 1000))).toBe('2 天前')
  })

  it('超过 7 天回退到 formatDateTime', () => {
    const d = new Date(2026, 4, 20, 8, 30, 0) // 2026-05-20 08:30
    expect(formatRelativeTime(d)).toBe('2026-05-20 08:30')
  })

  it('空值返回 "-"', () => {
    expect(formatRelativeTime(null)).toBe('-')
  })
})
