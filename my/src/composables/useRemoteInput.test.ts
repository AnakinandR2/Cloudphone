import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import type { ConnState } from './useWebRTC'
import { mapDeviceCoords, useRemoteInput } from './useRemoteInput'

describe('mapDeviceCoords · 旋转坐标变换', () => {
  it('竖屏（rotation 0、尺寸一致）原样返回', () => {
    expect(mapDeviceCoords(100, 200, 720, 1280, 720, 1280, 0)).toEqual({ x: 100, y: 200 })
  })
  it('竖屏分辨率不一致时等比缩放（360×640 → 720×1280：坐标 ×2）', () => {
    expect(mapDeviceCoords(100, 200, 360, 640, 720, 1280, 0)).toEqual({ x: 200, y: 400 })
  })
  it('横屏（rotation -90，1280×720）中心映射到中心', () => {
    expect(mapDeviceCoords(640, 360, 1280, 720, 1280, 720, -90)).toEqual({ x: 640, y: 360 })
  })
  it('横屏 -90 角点映射（px = W·(1 − vy/H)，py = H·(vx/W)，对齐官方 SDK 矩阵）', () => {
    expect(mapDeviceCoords(0, 0, 1280, 720, 1280, 720, -90)).toEqual({ x: 1280, y: 0 })
    expect(mapDeviceCoords(1280, 720, 1280, 720, 1280, 720, -90)).toEqual({ x: 0, y: 720 })
    expect(mapDeviceCoords(0, 720, 1280, 720, 1280, 720, -90)).toEqual({ x: 0, y: 0 })
  })
})

// 构造一个最小可用的 <video> 桩：getCoords 仅依赖 getBoundingClientRect + videoWidth/Height。
function makeVideo(
  rect: { left: number, top: number, width: number, height: number },
  vw = 720,
  vh = 1280,
) {
  return {
    getBoundingClientRect: () => ({
      ...rect,
      right: rect.left + rect.width,
      bottom: rect.top + rect.height,
      x: rect.left,
      y: rect.top,
      toJSON: () => ({}),
    }),
    videoWidth: vw,
    videoHeight: vh,
  } as unknown as HTMLVideoElement
}

function setup(opts?: { connState?: ConnState, rect?: { left: number, top: number, width: number, height: number } }) {
  const sendDC = vi.fn().mockReturnValue(true)
  const tryAutoUnmute = vi.fn()
  // 默认显示区 360x640、设备 720x1280 → 缩放系数 0.5、无黑边。
  const videoRef = ref(makeVideo(opts?.rect ?? { left: 0, top: 0, width: 360, height: 640 }))
  const connState = ref<ConnState>(opts?.connState ?? 'connected')
  const deviceWidth = ref(720)
  const deviceHeight = ref(1280)
  const input = useRemoteInput({ videoRef, connState, deviceWidth, deviceHeight, sendDC, tryAutoUnmute })
  return { input, sendDC, tryAutoUnmute }
}

function mouse(x: number, y: number, button = 0) {
  return { clientX: x, clientY: y, button, preventDefault: vi.fn() } as unknown as MouseEvent
}

describe('useRemoteInput · 坐标换算', () => {
  it('无黑边时按缩放换算到设备像素', () => {
    const { input, sendDC } = setup()
    input.onMouseDown(mouse(180, 320)) // 0.5 缩放 → 360,640
    expect(sendDC).toHaveBeenCalledTimes(1)
    const msg = sendDC.mock.calls[0][0]
    expect(msg).toMatchObject({ type: 'mouse_down', x: 360, y: 640, width: 720, height: 1280 })
    expect(typeof msg.messageId).toBe('string')
  })

  it('坐标超出视频区域时不下发', () => {
    const { input, sendDC } = setup()
    input.onMouseDown(mouse(500, 320)) // dx=500 > 显示宽 360
    expect(sendDC).not.toHaveBeenCalled()
  })

  it('换算结果钳制在 [0, device-1]', () => {
    const { input, sendDC } = setup()
    input.onMouseDown(mouse(360, 640)) // 边界 → 720/1280 钳到 719/1279
    const msg = sendDC.mock.calls[0][0]
    expect(msg.x).toBe(719)
    expect(msg.y).toBe(1279)
  })
})

describe('useRemoteInput · 连接态门禁', () => {
  it('未连接时忽略鼠标按下，且不触发自动取消静音', () => {
    const { input, sendDC, tryAutoUnmute } = setup({ connState: 'disconnected' })
    input.onMouseDown(mouse(180, 320))
    expect(sendDC).not.toHaveBeenCalled()
    expect(tryAutoUnmute).not.toHaveBeenCalled()
  })

  it('按下时触发 tryAutoUnmute', () => {
    const { input, tryAutoUnmute } = setup()
    input.onMouseDown(mouse(180, 320))
    expect(tryAutoUnmute).toHaveBeenCalledTimes(1)
  })
})

describe('useRemoteInput · 拖拽阈值与抬起', () => {
  it('抖动小于 5px 不发 mouse_move', () => {
    const { input, sendDC } = setup()
    input.onMouseDown(mouse(180, 320))
    sendDC.mockClear()
    input.onMouseMove(mouse(182, 322)) // 设备空间约 4px < 5px
    expect(sendDC).not.toHaveBeenCalled()
  })

  it('超过阈值发 mouse_move 并带 delta', () => {
    const { input, sendDC } = setup()
    input.onMouseDown(mouse(180, 320)) // 起点设备 360,640
    sendDC.mockClear()
    input.onMouseMove(mouse(200, 340)) // 设备 400,680
    expect(sendDC).toHaveBeenCalledTimes(1)
    expect(sendDC.mock.calls[0][0]).toMatchObject({ type: 'mouse_move', x: 400, y: 680, deltaX: 40, deltaY: 40 })
  })

  it('未按下时 mouse_move 被忽略', () => {
    const { input, sendDC } = setup()
    input.onMouseMove(mouse(200, 340))
    expect(sendDC).not.toHaveBeenCalled()
  })

  it('mouse_up 下发后清状态，二次抬起无效', () => {
    const { input, sendDC } = setup()
    input.onMouseDown(mouse(180, 320))
    sendDC.mockClear()
    input.onMouseUp(mouse(180, 320))
    expect(sendDC).toHaveBeenCalledTimes(1)
    expect(sendDC.mock.calls[0][0].type).toBe('mouse_up')

    sendDC.mockClear()
    input.onMouseUp(mouse(180, 320))
    expect(sendDC).not.toHaveBeenCalled()
  })

  it('onGlobalMouseUp 在按下后补一次抬起，未按下时无效', () => {
    const { input, sendDC } = setup()
    input.onMouseDown(mouse(180, 320))
    sendDC.mockClear()
    input.onGlobalMouseUp()
    expect(sendDC).toHaveBeenCalledTimes(1)
    expect(sendDC.mock.calls[0][0].type).toBe('mouse_up')

    sendDC.mockClear()
    input.onGlobalMouseUp()
    expect(sendDC).not.toHaveBeenCalled()
  })
})

describe('useRemoteInput · 触摸与系统键', () => {
  it('touchstart 下发 mouse_down', () => {
    const { input, sendDC } = setup()
    const e = {
      touches: [{ clientX: 180, clientY: 320 }],
      changedTouches: [],
      preventDefault: vi.fn(),
    } as unknown as TouchEvent
    input.onTouchStart(e)
    expect(sendDC).toHaveBeenCalledTimes(1)
    expect(sendDC.mock.calls[0][0]).toMatchObject({ type: 'mouse_down', x: 360, y: 640 })
  })

  it('sendButton 下发 button_<name>', () => {
    const { input, sendDC } = setup()
    input.sendButton('home')
    expect(sendDC).toHaveBeenCalledWith({ type: 'button_home' })
  })
})
