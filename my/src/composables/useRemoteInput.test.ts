import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import type { ConnState } from './useWebRTC'
import { useRemoteInput } from './useRemoteInput'

// 构造一个最小可用的 <video> 桩：resolvePos 依赖 getBoundingClientRect（CSS 变换后的外接框）、
// videoWidth/Height（推流尺寸）、offsetWidth/Height（不受 transform 影响的布局盒）。
function makeVideo(
  rect: { left: number, top: number, width: number, height: number },
  vw = 720,
  vh = 1280,
  offset?: { w: number, h: number },
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
    offsetWidth: offset?.w ?? rect.width,
    offsetHeight: offset?.h ?? rect.height,
  } as unknown as HTMLVideoElement
}

function setup(opts?: {
  connState?: ConnState
  rect?: { left: number, top: number, width: number, height: number }
  cssRotation?: number
}) {
  const sendDC = vi.fn().mockReturnValue(true)
  const tryAutoUnmute = vi.fn()
  // 默认显示区 360x640、设备 720x1280 → 缩放系数 0.5、无黑边。
  const videoRef = ref(makeVideo(opts?.rect ?? { left: 0, top: 0, width: 360, height: 640 }))
  const connState = ref<ConnState>(opts?.connState ?? 'connected')
  const deviceWidth = ref(720)
  const deviceHeight = ref(1280)
  const cssRotation = ref(opts?.cssRotation ?? 0)
  const input = useRemoteInput({ videoRef, connState, deviceWidth, deviceHeight, cssRotation, sendDC, tryAutoUnmute })
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

describe('useRemoteInput · 横屏坐标（推流已是横屏像素，rotation 固定 0）', () => {
  // 横屏推流 1280×720、显示区 1:1 无黑边；选择分辨率仍是竖屏 720×1280。
  function landscapeSetup() {
    const sendDC = vi.fn().mockReturnValue(true)
    const tryAutoUnmute = vi.fn()
    const videoRef = ref(makeVideo({ left: 0, top: 0, width: 1280, height: 720 }, 1280, 720))
    const connState = ref<ConnState>('connected')
    const deviceWidth = ref(720)
    const deviceHeight = ref(1280)
    const cssRotation = ref(0)
    const input = useRemoteInput({ videoRef, connState, deviceWidth, deviceHeight, cssRotation, sendDC, tryAutoUnmute })
    return { input, sendDC }
  }

  it('横屏点击按推流像素直接下发：x/y 即点击处、rotation=0、width/height=推流尺寸（对齐官方 SDK §5.3）', () => {
    const { input, sendDC } = landscapeSetup()
    input.onMouseDown(mouse(100, 50))
    expect(sendDC).toHaveBeenCalledTimes(1)
    expect(sendDC.mock.calls[0][0]).toMatchObject({
      type: 'mouse_down',
      x: 100,
      y: 50,
      width: 1280,
      height: 720,
      rotation: 0,
    })
  })
})

describe('useRemoteInput · 前端强制旋转的坐标补偿（cssRotation=-90）', () => {
  // 桌面锁定竖屏、用户手动转横屏：推流仍是竖屏 720×1280，<video> 被 CSS 旋转 -90° 铺满横屏画面区。
  // 布局盒（offsetWidth/Height）是竖屏 270×480；旋转后屏上外接框（getBoundingClientRect）是横屏 480×270。
  // 点击需绕画面中心反旋转 +90° 回到推流系，再按 rotation=0 下发推流像素坐标。
  function rotatedSetup() {
    const sendDC = vi.fn().mockReturnValue(true)
    const tryAutoUnmute = vi.fn()
    const videoRef = ref(makeVideo(
      { left: 0, top: 0, width: 480, height: 270 }, // 旋转后的外接框
      720,
      1280, // 竖屏推流
      { w: 270, h: 480 }, // 旋转前布局盒
    ))
    const connState = ref<ConnState>('connected')
    const deviceWidth = ref(720)
    const deviceHeight = ref(1280)
    const cssRotation = ref(-90)
    const input = useRemoteInput({ videoRef, connState, deviceWidth, deviceHeight, cssRotation, sendDC, tryAutoUnmute })
    return { input, sendDC }
  }

  it('画面中心点击 → 推流中心', () => {
    const { input, sendDC } = rotatedSetup()
    input.onMouseDown(mouse(240, 135)) // 横屏显示区中心
    expect(sendDC.mock.calls[0][0]).toMatchObject({ type: 'mouse_down', x: 360, y: 640, width: 720, height: 1280, rotation: 0 })
  })

  it('横屏显示「上边中点」→ 推流右侧中点（CCW 90° 旋转的几何关系）', () => {
    const { input, sendDC } = rotatedSetup()
    input.onMouseDown(mouse(240, 0)) // 显示区上边中点
    const msg = sendDC.mock.calls[0][0]
    expect(msg.x).toBe(719) // 推流最右（720 钳到 719）
    expect(msg.y).toBe(640) // 竖直方向中点
    expect(msg.rotation).toBe(0)
  })
})

describe('useRemoteInput · 坐标锚定「所选分辨率」空间（不随推流编码尺寸漂移）', () => {
  // 设备坐标系 = join 声明的分辨率(= 所选分辨率)。所选 1080×1920，但编码器实际推流 1072×1920；
  // 必须按所选 1080×1920 下发坐标，而非推流编码的 1072×1920，否则设备端按 1080 解释会错位。
  function changedResSetup() {
    const sendDC = vi.fn().mockReturnValue(true)
    const tryAutoUnmute = vi.fn()
    const videoRef = ref(makeVideo({ left: 0, top: 0, width: 536, height: 960 }, 1072, 1920))
    const connState = ref<ConnState>('connected')
    const deviceWidth = ref(1080) // 所选分辨率（设备坐标系基准）
    const deviceHeight = ref(1920)
    const cssRotation = ref(0)
    const input = useRemoteInput({ videoRef, connState, deviceWidth, deviceHeight, cssRotation, sendDC, tryAutoUnmute })
    return { input, sendDC }
  }

  it('底部中点点击 → 下发所选 1080×1920 空间坐标（而非推流 1072×1920）', () => {
    const { input, sendDC } = changedResSetup()
    input.onMouseDown(mouse(268, 925)) // 显示区底部中点（fraction ≈ 0.5, 0.96）
    const msg = sendDC.mock.calls[0][0]
    expect(msg.width).toBe(1080)
    expect(msg.height).toBe(1920)
    expect(msg.x).toBe(540) // 0.5 × 1080
    expect(msg.y).toBe(1850) // (1850/1920) × 1920
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
