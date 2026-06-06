import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  doneProgress,
  progressValue,
  progressVisible,
  startProgress,
} from './progress'

describe('progress', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    progressValue.value = 0
    progressVisible.value = false
  })
  afterEach(() => {
    vi.clearAllTimers()
    vi.useRealTimers()
  })

  it('startProgress 显示进度条并置初值 8', () => {
    startProgress()
    expect(progressVisible.value).toBe(true)
    expect(progressValue.value).toBe(8)
  })

  it('随定时器推进而增长，但趋近且不超过 ~90', () => {
    startProgress()
    vi.advanceTimersByTime(160)
    expect(progressValue.value).toBeGreaterThan(8)

    // 大量推进后应停在 90 附近（remain<=0 时不再增加）
    vi.advanceTimersByTime(160 * 100)
    expect(progressValue.value).toBeGreaterThanOrEqual(89)
    expect(progressValue.value).toBeLessThan(91)
  })

  it('doneProgress 立即置 100，240ms 后隐藏并归零', () => {
    startProgress()
    doneProgress()
    expect(progressValue.value).toBe(100)
    expect(progressVisible.value).toBe(true)

    vi.advanceTimersByTime(240)
    expect(progressVisible.value).toBe(false)
    expect(progressValue.value).toBe(0)
  })

  it('重复 startProgress 不会叠加多个定时器（速度正常）', () => {
    startProgress()
    startProgress()
    vi.advanceTimersByTime(160)
    // 仅一个定时器在跑：一次 tick 增量 = max(0.5, (90-8)*0.12)=9.84 → 17.84
    expect(progressValue.value).toBeCloseTo(17.84, 2)
  })
})
