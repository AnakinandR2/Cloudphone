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

// ===== 旋转坐标变换（移植自官方 SDK 的仿射矩阵，保证与设备端一致）=====
// 仿射矩阵以数组 [a,b,c,d,e,f] 表示：x' = a·x + c·y + e，y' = b·x + d·y + f。
type Affine = [number, number, number, number, number, number]
function mMul(m: Affine, n: Affine): Affine {
  return [
    m[0] * n[0] + m[2] * n[1],
    m[1] * n[0] + m[3] * n[1],
    m[0] * n[2] + m[2] * n[3],
    m[1] * n[2] + m[3] * n[3],
    m[0] * n[4] + m[2] * n[5] + m[4],
    m[1] * n[4] + m[3] * n[5] + m[5],
  ]
}
function rotAffine(rotation: number): Affine {
  switch (rotation) {
    case -90: return [0, -1, 1, 0, 0, 1]
    case 90: return [0, 1, -1, 0, 1, 0]
    case 180: return [-1, 0, 0, -1, 1, 1]
    case 270: return [0, -1, 1, 0, 0, 1]
    default: return [1, 0, 0, 1, 0, 0]
  }
}

// mapDeviceCoords 把视频像素坐标(vx,vy) 映射到设备坐标（含分辨率缩放 + 旋转）。
// videoW/H：当前推流分辨率；targetW/H：设备当前横竖屏分辨率；rotation：0 或 -90。
export function mapDeviceCoords(
  vx: number,
  vy: number,
  videoW: number,
  videoH: number,
  targetW: number,
  targetH: number,
  rotation: number,
): { x: number, y: number } {
  if (rotation === 0 && videoW === targetW && videoH === targetH)
    return { x: Math.round(vx), y: Math.round(vy) }
  // transform = ndcToPixels(target) · rotate · ndcFromPixels(video)
  const ndcFrom: Affine = [1 / videoW, 0, 0, -1 / videoH, 0, 1]
  const ndcTo: Affine = [targetW, 0, 0, -targetH, 0, targetH]
  const t = mMul(mMul(ndcTo, rotAffine(rotation)), ndcFrom)
  return { x: Math.round(vx * t[0] + vy * t[2] + t[4]), y: Math.round(vx * t[1] + vy * t[3] + t[5]) }
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

  // 上一次下发的方向元信息，供 onGlobalMouseUp 等无事件场景复用。
  let lastMeta = { width: 0, height: 0, rotation: 0 }

  // 把浏览器坐标换算到「设备坐标 + 当前分辨率 + 旋转」。
  // <video> 用 object-contain，先扣黑边得到视频像素坐标，再按横竖屏做旋转矩阵变换。
  function resolvePos(e: MouseEvent | Touch): { x: number, y: number, width: number, height: number, rotation: number } | null {
    if (!videoRef.value)
      return null
    const rect = videoRef.value.getBoundingClientRect()
    const dx = e.clientX - rect.left
    const dy = e.clientY - rect.top
    if (dx < 0 || dx > rect.width || dy < 0 || dy > rect.height)
      return null
    const vw = videoRef.value.videoWidth || deviceWidth.value
    const vh = videoRef.value.videoHeight || deviceHeight.value
    const scale = Math.max(rect.width / vw, rect.height / vh)
    const ox = (rect.width - vw * scale) / 2
    const oy = (rect.height - vh * scale) / 2
    // 视频像素坐标（限定在画面内）。
    const px = Math.max(0, Math.min((dx - ox) / scale, vw - 1))
    const py = Math.max(0, Math.min((dy - oy) / scale, vh - 1))

    // 当前横竖屏：推流宽>高即横屏。目标分辨率取所选分辨率的横/竖排列，旋转角对齐设备。
    const landscape = vw > vh
    const selW = deviceWidth.value
    const selH = deviceHeight.value
    const tw = landscape ? Math.max(selW, selH) : Math.min(selW, selH)
    const th = landscape ? Math.min(selW, selH) : Math.max(selW, selH)
    const rotation = landscape ? -90 : 0

    const d = mapDeviceCoords(px, py, vw, vh, tw, th, rotation)
    lastMeta = { width: tw, height: th, rotation }
    return {
      x: Math.max(0, Math.min(d.x, tw - 1)),
      y: Math.max(0, Math.min(d.y, th - 1)),
      width: tw,
      height: th,
      rotation,
    }
  }

  function onMouseDown(e: MouseEvent) {
    if (connState.value !== 'connected')
      return
    e.preventDefault()
    tryAutoUnmute()
    const p = resolvePos(e)
    if (!p)
      return
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
