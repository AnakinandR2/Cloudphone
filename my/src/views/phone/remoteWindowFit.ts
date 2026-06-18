// 远控弹窗「窗体旋转」尺寸计算（纯函数，便于单测）。
//
// 远控是 window.open 出来的独立弹窗，没有父窗口替它旋转，必须自己 resizeTo。
// 设备/应用转横屏时推流分辨率交换（宽>高），此处把设备「长边」映射到固定的屏上长度，
// 横竖屏时长短边互换 → 弹窗形态随之「旋转」（竖屏=窄高，横屏=宽扁），画面铺满无大黑边。

export interface RemoteWindowFitInput {
  /** 实时推流宽（取不到时传 0，回退竖屏 9:16）。 */
  streamW: number
  /** 实时推流高（取不到时传 0）。 */
  streamH: number
  /** 右侧展开面板宽度（无面板传 0）。 */
  panelW: number
  /** 图标操作列宽度。 */
  sidebarW: number
  /** 设备长边映射到屏上的目标长度（竖屏的画面高 / 横屏的画面宽）。 */
  longEdge: number
  /** 屏幕可用宽。 */
  availW: number
  /** 屏幕可用高。 */
  availH: number
  /** 窗口宽度方向的「外壳」尺寸（outerWidth - innerWidth）。 */
  chromeW: number
  /** 窗口高度方向的「外壳」尺寸（outerHeight - innerHeight，含标题栏/地址栏）。 */
  chromeH: number
}

export interface RemoteWindowFitResult {
  /** resizeTo 用的外层宽。 */
  outerW: number
  /** resizeTo 用的外层高。 */
  outerH: number
  /** 是否横屏（宽>=高）。 */
  landscape: boolean
}

/**
 * 按实时推流比例算出远控弹窗应有的外层宽高。
 * 整窗（画面 + 侧栏 + 面板）等比缩放以不超出屏幕可用区。
 */
export function computeRemoteWindowSize(i: RemoteWindowFitInput): RemoteWindowFitResult {
  const aspect = i.streamW > 0 && i.streamH > 0 ? i.streamW / i.streamH : 9 / 16
  const landscape = aspect >= 1

  // 长边固定 = longEdge，短边按比例。横屏时长边是宽、竖屏时长边是高 → 形态互换。
  let videoW: number
  let videoH: number
  if (landscape) {
    videoW = i.longEdge
    videoH = i.longEdge / aspect
  }
  else {
    videoH = i.longEdge
    videoW = i.longEdge * aspect
  }

  // 等比缩放使整窗不超出屏幕可用区（横屏画面很宽时尤其重要）。
  const maxVideoW = Math.max(1, i.availW - i.chromeW - i.sidebarW - i.panelW)
  const maxVideoH = Math.max(1, i.availH - i.chromeH)
  const scale = Math.min(1, maxVideoW / videoW, maxVideoH / videoH)
  videoW = Math.round(videoW * scale)
  videoH = Math.round(videoH * scale)

  const innerW = videoW + i.sidebarW + i.panelW
  const innerH = videoH
  return {
    outerW: innerW + i.chromeW,
    outerH: innerH + i.chromeH,
    landscape,
  }
}
