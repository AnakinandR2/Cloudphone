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
  sendDC: (msg: Record<string, unknown>) => boolean
  tryAutoUnmute: () => void
}

export function useRemoteInput(opts: UseRemoteInputOptions) {
  const { videoRef, connState, deviceWidth, deviceHeight, sendDC, tryAutoUnmute } = opts

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

  // 把浏览器坐标（相对视口）换算到设备像素空间。<video> 用 object-contain，需先扣掉黑边偏移再缩放。
  function getCoords(e: MouseEvent | Touch): { x: number, y: number } | null {
    if (!videoRef.value)
      return null
    const rect = videoRef.value.getBoundingClientRect()
    const cx = e.clientX
    const cy = e.clientY
    const dx = cx - rect.left
    const dy = cy - rect.top
    if (dx < 0 || dx > rect.width || dy < 0 || dy > rect.height)
      return null
    const vw = videoRef.value.videoWidth || deviceWidth.value
    const vh = videoRef.value.videoHeight || deviceHeight.value
    const scale = Math.max(rect.width / vw, rect.height / vh)
    const dw = vw * scale
    const dh = vh * scale
    const ox = (rect.width - dw) / 2
    const oy = (rect.height - dh) / 2
    const srcX = (dx - ox) / scale
    const srcY = (dy - oy) / scale
    return {
      x: Math.max(0, Math.min(Math.round(srcX), deviceWidth.value - 1)),
      y: Math.max(0, Math.min(Math.round(srcY), deviceHeight.value - 1)),
    }
  }

  function onMouseDown(e: MouseEvent) {
    if (connState.value !== 'connected')
      return
    e.preventDefault()
    tryAutoUnmute()
    const c = getCoords(e)
    if (!c)
      return
    currentDownEventId = genId()
    isDragging = false
    dragStartX = c.x
    dragStartY = c.y
    sendDC({ type: 'mouse_down', x: c.x, y: c.y, button: e.button, rotation: 0, width: deviceWidth.value, height: deviceHeight.value, messageId: currentDownEventId })
  }

  function onMouseMove(e: MouseEvent) {
    if (!currentDownEventId || connState.value !== 'connected')
      return
    e.preventDefault()
    const c = getCoords(e)
    if (!c)
      return
    if (!isDragging && Math.abs(c.x - dragStartX) < 5 && Math.abs(c.y - dragStartY) < 5)
      return
    isDragging = true
    sendDC({ type: 'mouse_move', x: c.x, y: c.y, deltaX: c.x - dragStartX, deltaY: c.y - dragStartY, rotation: 0, width: deviceWidth.value, height: deviceHeight.value, downEventId: currentDownEventId, messageId: genId() })
  }

  function onMouseUp(e: MouseEvent) {
    if (!currentDownEventId || connState.value !== 'connected')
      return
    e.preventDefault()
    const c = getCoords(e)
    if (!c)
      return
    sendDC({ type: 'mouse_up', x: c.x, y: c.y, button: e.button, rotation: 0, width: deviceWidth.value, height: deviceHeight.value, downEventId: currentDownEventId, messageId: genId() })
    currentDownEventId = null
    isDragging = false
  }

  // 指针在按下状态离开视频区域时补一个 mouse_up，避免设备侧卡在「手指按下」。
  function onGlobalMouseUp() {
    if (!currentDownEventId)
      return
    sendDC({ type: 'mouse_up', x: dragStartX, y: dragStartY, button: 0, rotation: 0, width: deviceWidth.value, height: deviceHeight.value, downEventId: currentDownEventId, messageId: genId() })
    currentDownEventId = null
    isDragging = false
  }

  // Touch
  function onTouchStart(e: TouchEvent) {
    if (connState.value !== 'connected' || !e.touches[0])
      return
    e.preventDefault()
    tryAutoUnmute()
    const c = getCoords(e.touches[0])
    if (!c)
      return
    currentDownEventId = genId()
    sendDC({ type: 'mouse_down', x: c.x, y: c.y, rotation: 0, width: deviceWidth.value, height: deviceHeight.value, downEventId: currentDownEventId, messageId: genId() })
  }
  function onTouchMove(e: TouchEvent) {
    if (!currentDownEventId || connState.value !== 'connected' || !e.touches[0])
      return
    e.preventDefault()
    const c = getCoords(e.touches[0])
    if (!c)
      return
    sendDC({ type: 'mouse_move', x: c.x, y: c.y, rotation: 0, width: deviceWidth.value, height: deviceHeight.value, downEventId: currentDownEventId, messageId: genId() })
  }
  function onTouchEnd(e: TouchEvent) {
    if (!currentDownEventId || connState.value !== 'connected')
      return
    e.preventDefault()
    const tt = e.changedTouches[0]
    if (!tt)
      return
    const c = getCoords(tt)
    if (!c)
      return
    sendDC({ type: 'mouse_up', x: c.x, y: c.y, rotation: 0, width: deviceWidth.value, height: deviceHeight.value, downEventId: currentDownEventId, messageId: genId() })
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
