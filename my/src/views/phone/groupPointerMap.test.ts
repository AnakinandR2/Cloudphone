import { describe, expect, it } from 'vitest'
import { displayFractionToDevice } from './groupPointerMap'

describe('displayFractionToDevice · 群控归一化坐标还原', () => {
  it('竖屏（cssRotation 0、设备竖屏）：df 直接乘所选竖屏分辨率', () => {
    const r = displayFractionToDevice(0.5, 0.95, 0, false, 720, 1280)
    expect(r).toEqual({ x: 360, y: 1216, width: 720, height: 1280 })
  })

  it('设备真横屏（cssRotation 0、streamLandscape）：长短边交换，df 乘横屏分辨率', () => {
    const r = displayFractionToDevice(0.5, 0.5, 0, true, 720, 1280)
    expect(r).toEqual({ x: 640, y: 360, width: 1280, height: 720 })
  })

  it('前端强制横屏（cssRotation -90、推流仍竖屏）：中心→中心、设备坐标仍是竖屏分辨率', () => {
    const r = displayFractionToDevice(0.5, 0.5, -90, false, 720, 1280)
    expect(r).toEqual({ x: 360, y: 640, width: 720, height: 1280 })
  })

  it('前端强制横屏：显示左上角 → 推流右上角(x 最大、y=0)', () => {
    const r = displayFractionToDevice(0, 0, -90, false, 720, 1280)
    expect(r.x).toBe(719) // 1-0=1 → 720 钳到 719
    expect(r.y).toBe(0)
    expect(r.width).toBe(720)
    expect(r.height).toBe(1280)
  })

  it('前端强制横屏：显示上边中点 → 推流右侧中点', () => {
    const r = displayFractionToDevice(0.5, 0, -90, false, 720, 1280)
    expect(r.x).toBe(719) // 1-0=1 → 钳到 719
    expect(r.y).toBe(640) // dfx 0.5 → y 中点
  })

  it('钳制在 [0, dev-1]', () => {
    const r = displayFractionToDevice(1, 1, 0, false, 720, 1280)
    expect(r.x).toBe(719)
    expect(r.y).toBe(1279)
  })
})
