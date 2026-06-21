import { describe, expect, it } from 'vitest'
import { computeRemoteWindowSize } from './remoteWindowFit'

const BASE = {
  panelW: 0,
  sidebarW: 56,
  longEdge: 800,
  availW: 4000, // 屏幕足够大，先不触发缩放
  availH: 4000,
  chromeW: 16,
  chromeH: 80,
}

describe('computeRemoteWindowSize · 按显示方向定窗体', () => {
  it('竖屏(landscape=false, 推流720×1280)：窄而高，长边=高', () => {
    const r = computeRemoteWindowSize({ ...BASE, streamW: 720, streamH: 1280, landscape: false })
    // videoH=longEdge=800, videoW=800/(1280/720)=450
    expect(r.outerH).toBe(800 + 80) // 880
    expect(r.outerW).toBe(450 + 56 + 16) // 522
    expect(r.outerH).toBeGreaterThan(r.outerW) // 竖屏形态
  })

  it('横屏(landscape=true, 推流1280×720)：宽而扁，长边=宽', () => {
    const r = computeRemoteWindowSize({ ...BASE, streamW: 1280, streamH: 720, landscape: true })
    // videoW=longEdge=800, videoH=800/(1280/720)=450
    expect(r.outerW).toBe(800 + 56 + 16) // 872
    expect(r.outerH).toBe(450 + 80) // 530
    expect(r.outerW).toBeGreaterThan(r.outerH) // 横屏形态
  })

  it('强制横屏：竖屏推流(720×1280)+landscape=true 与原生横屏推流得到同样的窗体', () => {
    // 桌面锁定竖屏、用户手动旋转：推流仍是竖屏像素，但窗体按横屏定。
    const forced = computeRemoteWindowSize({ ...BASE, streamW: 720, streamH: 1280, landscape: true })
    const native = computeRemoteWindowSize({ ...BASE, streamW: 1280, streamH: 720, landscape: true })
    expect(forced.outerW).toBe(native.outerW)
    expect(forced.outerH).toBe(native.outerH)
    expect(forced.outerW).toBeGreaterThan(forced.outerH)
  })

  it('竖↔横切换：长边在屏上长度保持一致（真正“旋转”而非缩放）', () => {
    const portrait = computeRemoteWindowSize({ ...BASE, streamW: 720, streamH: 1280, landscape: false })
    const landscape = computeRemoteWindowSize({ ...BASE, streamW: 1280, streamH: 720, landscape: true })
    // 竖屏的画面高(=长边) === 横屏的画面宽(=长边)
    expect(portrait.outerH - BASE.chromeH).toBe(landscape.outerW - BASE.chromeW - BASE.sidebarW)
  })

  it('推流尺寸缺失时回退 9:16 长短比', () => {
    const r = computeRemoteWindowSize({ ...BASE, streamW: 0, streamH: 0, landscape: false })
    expect(r.outerW).toBe(Math.round(800 * 9 / 16) + 56 + 16) // 450+72
  })

  it('横屏画面过宽时整窗等比缩小，不超出屏幕可用宽', () => {
    // availW 很窄，强制缩放
    const r = computeRemoteWindowSize({ ...BASE, streamW: 1280, streamH: 720, landscape: true, availW: 500, availH: 4000 })
    expect(r.outerW).toBeLessThanOrEqual(500)
    // 仍保持横屏比例（宽>高）
    expect(r.outerW).toBeGreaterThan(r.outerH)
  })

  it('面板展开时窗宽计入面板，画面尺寸不变', () => {
    const noPanel = computeRemoteWindowSize({ ...BASE, streamW: 720, streamH: 1280, landscape: false })
    const withPanel = computeRemoteWindowSize({ ...BASE, streamW: 720, streamH: 1280, landscape: false, panelW: 380 })
    expect(withPanel.outerW - noPanel.outerW).toBe(380)
    expect(withPanel.outerH).toBe(noPanel.outerH)
  })
})
