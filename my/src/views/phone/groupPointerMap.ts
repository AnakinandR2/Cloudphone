// 群控指针映射（纯函数，便于单测）。
//
// 群控：主控格把点击换算成「显示画面归一化坐标」df ∈ [0,1] 广播给全体，各格按自身方向/分辨率
// 还原到设备坐标后下发。设备坐标系 = join 时声明的分辨率(= 所选分辨率)，按设备实际方向摆长短边；
// 与单控 useRemoteInput 同口径（rotation 固定 0，前端 CSS 旋转在此完全补偿）。

export interface DeviceCoords {
  x: number
  y: number
  width: number
  height: number
}

/**
 * 显示画面归一化坐标 (dfx,dfy) → 设备坐标。
 * @param cssRotation 本格前端施加的画面旋转（0 / -90）。-90 表示显示是推流逆时针转 90°。
 * @param streamLandscape 设备实际方向（推流宽>高=横屏）。
 * @param selShort 所选分辨率短边。
 * @param selLong 所选分辨率长边。
 */
export function displayFractionToDevice(
  dfx: number,
  dfy: number,
  cssRotation: number,
  streamLandscape: boolean,
  selShort: number,
  selLong: number,
): DeviceCoords {
  // 显示系 → 推流系归一化：cssRotation=-90 时显示画面是推流逆时针转 90°，
  // 显示左上角 = 推流右上角 → (dfx,dfy) 映射为 (1-dfy, dfx)。
  let sfx = dfx
  let sfy = dfy
  if (cssRotation === -90) {
    sfx = 1 - dfy
    sfy = dfx
  }
  // 设备坐标系按设备实际方向摆长短边（横屏=长边为宽）。
  const width = streamLandscape ? selLong : selShort
  const height = streamLandscape ? selShort : selLong
  const x = Math.round(Math.max(0, Math.min(sfx * width, width - 1)))
  const y = Math.round(Math.max(0, Math.min(sfy * height, height - 1)))
  return { x, y, width, height }
}
