// useRemoteInput.ts
//
// 远程控制的触摸/鼠标/键盘输入处理：把浏览器事件换算到设备像素坐标，经 WebRTC
// 数据通道(sendDC)下发。系统按键(home/back/recent/volume…)走 button_<name>。
// 协议对齐中台官方实现（cp-glory-service）。
import { type Ref } from 'vue'
import type { ConnState } from './useWebRTC'

export interface UseRemoteInputOptions {
  videoRef: Ref<HTMLVideoElement | null>
  connState: Ref<ConnState>
  deviceWidth: Ref<number>
  deviceHeight: Ref<number>
  /** 前端施加在 <video> 上的 CSS 旋转角（0 或 -90）。点击坐标需绕画面中心反旋转回推流系。 */
  cssRotation: Ref<number>
  sendDC: (msg: Record<string, unknown>) => boolean
  tryAutoUnmute: () => void
}

export function useRemoteInput(opts: UseRemoteInputOptions) {
  const { videoRef, connState, deviceWidth, deviceHeight, cssRotation, sendDC, tryAutoUnmute } = opts

  let currentDownEventId: string | null = null
  let isDragging = false
  let dragStartX = 0
  let dragStartY = 0

  function genId(): string {
    const uuid = 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
      const r = (Math.random() * 16) | 0
      return (c === 'x' ? r : (r & 0x3) | 0x8).toString(16)
    })
    const n = new Date()
    return `${uuid}-${n.getFullYear()}${String(n.getMonth() + 1).padStart(2, '0')}${String(n.getDate()).padStart(2, '0')} ${String(n.getHours()).padStart(2, '0')}:${String(n.getMinutes()).padStart(2, '0')}:${String(n.getSeconds()).padStart(2, '0')}:${String(n.getMilliseconds()).padStart(3, '0')}`
  }

  // 上一次下发的方向元信息，供 onGlobalMouseUp 等无事件场景复用。
  let lastMeta = { width: 0, height: 0, rotation: 0 }

  // 把浏览器坐标换算到「推流像素坐标」（即设备当前屏幕坐标），统一以 rotation:0 下发。
  //
  // ⚠️ 方向：推流分辨率「即」设备当前屏幕像素。
  //   · 设备/应用自己转横屏：推流已是横屏像素，点到的就是设备坐标，cssRotation=0，直接换算。
  //   · 桌面锁定竖屏、用户手动转横屏：推流仍是竖屏像素，<video> 被 CSS 旋转 cssRotation° 显示；
  //     需绕画面中心把点击「反旋转」回布局系，再换算到推流像素。
  // 两种情况都把结果按 width/height=当前推流分辨率、rotation=0 下发（对齐协议 §5.3「rotation 固定 0」
  // 与官方 SDK：设备永远收到干净的推流系坐标，CSS 旋转纯属客户端显示、已在此完全补偿）。
  function resolvePos(e: MouseEvent | Touch): { x: number, y: number, width: number, height: number, rotation: number } | null {
    const v = videoRef.value
    if (!v)
      return null
    // getBoundingClientRect 返回 CSS 变换后的外接框；其中心在绕中心旋转下保持不变。
    const rect = v.getBoundingClientRect()
    const cx = rect.left + rect.width / 2
    const cy = rect.top + rect.height / 2
    // 点击相对画面中心的屏幕向量，按 -cssRotation 反旋转回元素布局系（y 向下）。
    const sx = e.clientX - cx
    const sy = e.clientY - cy
    const rad = (cssRotation.value * Math.PI) / 180
    const cos = Math.cos(rad)
    const sin = Math.sin(rad)
    const lx = sx * cos + sy * sin
    const ly = -sx * sin + sy * cos
    // 布局盒尺寸不受 transform 影响（offsetWidth/Height）；换算到盒内局部坐标。
    const boxW = v.offsetWidth || rect.width
    const boxH = v.offsetHeight || rect.height
    const localX = boxW / 2 + lx
    const localY = boxH / 2 + ly
    if (localX < 0 || localX > boxW || localY < 0 || localY > boxH)
      return null
    // 布局盒 → 推流像素（object-contain：盒按推流比例，通常精确无黑边）。
    const vw = v.videoWidth || deviceWidth.value
    const vh = v.videoHeight || deviceHeight.value
    const displayScale = Math.min(boxW / vw, boxH / vh)
    const ox = (boxW - vw * displayScale) / 2
    const oy = (boxH - vh * displayScale) / 2
    // 画面内归一化位置（0..1）。
    const fx = (localX - ox) / displayScale / vw
    const fy = (localY - oy) / displayScale / vh
    // 换算到「所选分辨率」空间下发——设备坐标系 = join 时声明的分辨率(= 所选分辨率)，
    // 设备端按该分辨率的原始像素解释坐标、不按下发的 width/height 缩放；按当前推流方向摆长短边。
    const selLong = Math.max(deviceWidth.value, deviceHeight.value)
    const selShort = Math.min(deviceWidth.value, deviceHeight.value)
    const devW = vw > vh ? selLong : selShort
    const devH = vw > vh ? selShort : selLong
    const x = Math.round(Math.max(0, Math.min(fx * devW, devW - 1)))
    const y = Math.round(Math.max(0, Math.min(fy * devH, devH - 1)))
    lastMeta = { width: devW, height: devH, rotation: 0 }
    return { x, y, width: devW, height: devH, rotation: 0 }
  }

  function onMouseDown(e: MouseEvent) {
    if (connState.value !== 'connected')
      return
    e.preventDefault()
    tryAutoUnmute()
    const p = resolvePos(e)
    if (!p)
      return
    // [临时调试] 切换分辨率后点击错位排查：打印原始点击、视频/布局/推流尺寸、所选分辨率、下发坐标。
    const dv = videoRef.value
    // eslint-disable-next-line no-console
    console.log('[RC-click]', JSON.stringify({
      client: { x: e.clientX, y: e.clientY },
      video: { w: dv?.videoWidth, h: dv?.videoHeight },
      selectedRes: { w: deviceWidth.value, h: deviceHeight.value },
      cssRotation: cssRotation.value,
      sent: { x: p.x, y: p.y, width: p.width, height: p.height, rotation: p.rotation },
    }))
    currentDownEventId = genId()
    isDragging = false
    dragStartX = p.x
    dragStartY = p.y
    sendDC({ type: 'mouse_down', x: p.x, y: p.y, button: e.button, rotation: p.rotation, width: p.width, height: p.height, messageId: currentDownEventId })
  }

  function onMouseMove(e: MouseEvent) {
    if (!currentDownEventId || connState.value !== 'connected')
      return
    e.preventDefault()
    const p = resolvePos(e)
    if (!p)
      return
    if (!isDragging && Math.abs(p.x - dragStartX) < 5 && Math.abs(p.y - dragStartY) < 5)
      return
    isDragging = true
    sendDC({ type: 'mouse_move', x: p.x, y: p.y, deltaX: p.x - dragStartX, deltaY: p.y - dragStartY, rotation: p.rotation, width: p.width, height: p.height, downEventId: currentDownEventId, messageId: genId() })
  }

  function onMouseUp(e: MouseEvent) {
    if (!currentDownEventId || connState.value !== 'connected')
      return
    e.preventDefault()
    const p = resolvePos(e)
    if (!p)
      return
    sendDC({ type: 'mouse_up', x: p.x, y: p.y, button: e.button, rotation: p.rotation, width: p.width, height: p.height, downEventId: currentDownEventId, messageId: genId() })
    currentDownEventId = null
    isDragging = false
  }

  // 指针在按下状态离开视频区域时补一个 mouse_up，避免设备侧卡在「手指按下」。复用上次的方向元信息。
  function onGlobalMouseUp() {
    if (!currentDownEventId)
      return
    sendDC({ type: 'mouse_up', x: dragStartX, y: dragStartY, button: 0, rotation: lastMeta.rotation, width: lastMeta.width, height: lastMeta.height, downEventId: currentDownEventId, messageId: genId() })
    currentDownEventId = null
    isDragging = false
  }

  // Touch
  function onTouchStart(e: TouchEvent) {
    if (connState.value !== 'connected' || !e.touches[0])
      return
    e.preventDefault()
    tryAutoUnmute()
    const p = resolvePos(e.touches[0])
    if (!p)
      return
    currentDownEventId = genId()
    sendDC({ type: 'mouse_down', x: p.x, y: p.y, rotation: p.rotation, width: p.width, height: p.height, downEventId: currentDownEventId, messageId: genId() })
  }
  function onTouchMove(e: TouchEvent) {
    if (!currentDownEventId || connState.value !== 'connected' || !e.touches[0])
      return
    e.preventDefault()
    const p = resolvePos(e.touches[0])
    if (!p)
      return
    sendDC({ type: 'mouse_move', x: p.x, y: p.y, rotation: p.rotation, width: p.width, height: p.height, downEventId: currentDownEventId, messageId: genId() })
  }
  function onTouchEnd(e: TouchEvent) {
    if (!currentDownEventId || connState.value !== 'connected')
      return
    e.preventDefault()
    const tt = e.changedTouches[0]
    if (!tt)
      return
    const p = resolvePos(tt)
    if (!p)
      return
    sendDC({ type: 'mouse_up', x: p.x, y: p.y, rotation: p.rotation, width: p.width, height: p.height, downEventId: currentDownEventId, messageId: genId() })
    currentDownEventId = null
  }

  // sendButton 下发系统/设备按键（home / back / recent / power / volume_up / volume_down），
  // 设备侧解码 button_<name>。
  function sendButton(btn: string) {
    sendDC({ type: `button_${btn}` })
  }

  return {
    onMouseDown,
    onMouseMove,
    onMouseUp,
    onGlobalMouseUp,
    onTouchStart,
    onTouchMove,
    onTouchEnd,
    sendButton,
  }
}
