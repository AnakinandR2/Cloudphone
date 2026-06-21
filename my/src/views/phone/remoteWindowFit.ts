// 远控弹窗「窗体旋转」尺寸计算（纯函数，便于单测）。
//
// 远控是 window.open 出来的独立弹窗，没有父窗口替它旋转，必须自己 resizeTo。
// 显示方向由调用方决定（displayLandscape：设备自转跟随 / 用户手动强制），与推流尺寸解耦——
// 桌面锁定竖屏时用户手动转横屏，推流仍是竖屏像素，但窗体仍应按横屏定（前端 CSS 转画面铺满）。
// 此处把「长边」映射到固定的屏上长度，按显示方向把长短边摆成竖屏(窄高)/横屏(宽扁)。

export interface RemoteWindowFitInput {
  /** 实时推流宽（取不到时传 0，回退 9:16 长短比）。 */
  streamW: number
  /** 实时推流高（取不到时传 0）。 */
  streamH: number
  /** 显示方向：true=横屏(宽扁)、false=竖屏(窄高)。= displayLandscape。 */
  landscape: boolean
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
}

/**
 * 按显示方向 + 推流长短比算出远控弹窗应有的外层宽高。
 * 整窗（画面 + 侧栏 + 面板）等比缩放以不超出屏幕可用区。
 */
export function computeRemoteWindowSize(i: RemoteWindowFitInput): RemoteWindowFitResult {
  // 推流长短边之比（恒 ≥1，与方向无关）；取不到回退 16/9。
  const ratio = i.streamW > 0 && i.streamH > 0 ? i.streamW / i.streamH : 9 / 16
  const longRatio = Math.max(ratio, 1 / ratio)

  // 长边固定 = longEdge，短边 = 长边 / 长短比。横屏长边是宽、竖屏长边是高 → 形态由 landscape 决定。
  let videoW: number
  let videoH: number
  if (i.landscape) {
    videoW = i.longEdge
    videoH = i.longEdge / longRatio
  }
  else {
    videoH = i.longEdge
    videoW = i.longEdge / longRatio
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
  }
}
