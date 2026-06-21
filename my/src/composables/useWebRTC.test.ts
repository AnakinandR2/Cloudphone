import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { pickRttMs, rttToLevel, useWebRTC } from './useWebRTC'

// 隔离 api：构造 useWebRTC 不应触碰真实后端（connect() 才会用到）。
vi.mock('@/api/modules/phone', () => ({
  default: { webrtcAuth: vi.fn() },
}))

function setup(videoRef = ref<HTMLVideoElement | null>(null)) {
  return useWebRTC({ id: ref(1), videoRef, onError: vi.fn() })
}

describe('useWebRTC · 选项与派生', () => {
  it('分辨率/画质/帧率选项符合预设', () => {
    const rtc = setup()
    expect(rtc.resOptions.map(r => r.label)).toEqual(['360x640', '720x1280', '1080x1920'])
    expect(rtc.qualityOptions.map(q => q.value)).toEqual([50, 30, 10])
    expect(rtc.fpsOptions[0]).toBe(10)
    expect(rtc.fpsOptions.at(-1)).toBe(60)
    expect(rtc.fpsOptions).toHaveLength(11)
  })

  it('deviceWidth/Height 跟随 selectedRes 派生', () => {
    const rtc = setup()
    expect(rtc.deviceWidth.value).toBe(720) // 默认 720x1280
    expect(rtc.deviceHeight.value).toBe(1280)

    rtc.selectedRes.value = '1080x1920'
    expect(rtc.deviceWidth.value).toBe(1080)
    expect(rtc.deviceHeight.value).toBe(1920)
  })

  it('未知分辨率回退到 720x1280', () => {
    const rtc = setup()
    rtc.selectedRes.value = 'bogus'
    expect(rtc.deviceWidth.value).toBe(720)
    expect(rtc.deviceHeight.value).toBe(1280)
  })
})

describe('useWebRTC · 状态与控制', () => {
  it('connected 反映 connState', () => {
    const rtc = setup()
    expect(rtc.connected.value).toBe(false)
    rtc.connState.value = 'connected'
    expect(rtc.connected.value).toBe(true)
  })

  it('数据通道未建立时 sendDC 返回 false', () => {
    const rtc = setup()
    expect(rtc.sendDC({ type: 'mouse_down' })).toBe(false)
  })

  it('默认静音；videoRef 为空时 toggleMute 安全无副作用', () => {
    const rtc = setup()
    expect(rtc.isMuted.value).toBe(true)
    rtc.toggleMute() // videoRef 为 null → 直接返回
    expect(rtc.isMuted.value).toBe(true)
  })

  it('有 video 元素时 toggleMute 取消静音并尝试播放', () => {
    const play = vi.fn().mockResolvedValue(undefined)
    const videoRef = ref({ muted: true, play } as unknown as HTMLVideoElement)
    const rtc = setup(videoRef)
    rtc.toggleMute()
    expect(rtc.isMuted.value).toBe(false)
    expect(videoRef.value!.muted).toBe(false)
    expect(play).toHaveBeenCalled()
  })
})

describe('useWebRTC · 屏幕方向（跟随推流 + 粘性手动）', () => {
  it('首次手动前：displayLandscape 跟随推流，cssRotation 恒 0（无需补偿）', () => {
    const videoRef = ref({ videoWidth: 720, videoHeight: 1280 } as unknown as HTMLVideoElement)
    const rtc = setup(videoRef)
    rtc.syncOrientation()
    expect(rtc.streamLandscape.value).toBe(false)
    expect(rtc.displayLandscape.value).toBe(false) // desired=null → 跟随推流
    expect(rtc.cssRotation.value).toBe(0)

    // 应用自动转横屏 → 推流宽>高，display 跟随，仍不需 CSS 旋转
    videoRef.value!.videoWidth = 1280
    videoRef.value!.videoHeight = 720
    rtc.syncOrientation()
    expect(rtc.streamLandscape.value).toBe(true)
    expect(rtc.displayLandscape.value).toBe(true)
    expect(rtc.cssRotation.value).toBe(0)
  })

  it('推流尺寸缺失时 syncOrientation 不改变 streamLandscape', () => {
    const videoRef = ref({ videoWidth: 0, videoHeight: 0 } as unknown as HTMLVideoElement)
    const rtc = setup(videoRef)
    rtc.syncOrientation()
    expect(rtc.streamLandscape.value).toBe(false)
  })

  it('桌面锁定竖屏手动旋转：display 转横屏、CSS 补偿 -90（前端把竖屏画面转过来）', () => {
    const videoRef = ref({ videoWidth: 720, videoHeight: 1280 } as unknown as HTMLVideoElement)
    const rtc = setup(videoRef)
    rtc.syncOrientation()

    expect(rtc.rotateDevice()).toBe(true) // 即使 dc 未就绪，前端方向也会变
    expect(rtc.desiredLandscape.value).toBe(true)
    expect(rtc.displayLandscape.value).toBe(true)
    // 推流仍是竖屏像素、显示要横屏 → 前端 CSS 旋转补齐
    expect(rtc.cssRotation.value).toBe(-90)
  })

  it('手动转横屏后设备真的转了：推流追上 → CSS 归 0，绝不双重旋转', () => {
    const videoRef = ref({ videoWidth: 720, videoHeight: 1280 } as unknown as HTMLVideoElement)
    const rtc = setup(videoRef)
    rtc.syncOrientation()
    rtc.rotateDevice() // desired=横屏
    expect(rtc.cssRotation.value).toBe(-90)

    // 设备/相机真的转了横屏，推流尺寸交换
    videoRef.value!.videoWidth = 1280
    videoRef.value!.videoHeight = 720
    rtc.syncOrientation()
    expect(rtc.displayLandscape.value).toBe(true) // 手动选择粘性保留
    expect(rtc.cssRotation.value).toBe(0) // 推流已横屏，无需再 CSS 旋转
  })

  it('手动选择粘性：设备自转回竖屏，display 仍保持上次手动的横屏（按用户选择）', () => {
    const videoRef = ref({ videoWidth: 1280, videoHeight: 720 } as unknown as HTMLVideoElement)
    const rtc = setup(videoRef)
    rtc.syncOrientation() // streamLandscape=true, display 跟随=true
    rtc.rotateDevice() // desired= !true = false（手动转竖屏）
    expect(rtc.desiredLandscape.value).toBe(false)
    expect(rtc.displayLandscape.value).toBe(false)
    expect(rtc.cssRotation.value).toBe(-90) // 想竖屏、推流是横屏 → CSS 补偿

    // 推流自己变回竖屏
    videoRef.value!.videoWidth = 720
    videoRef.value!.videoHeight = 1280
    rtc.syncOrientation()
    expect(rtc.displayLandscape.value).toBe(false) // 粘性
    expect(rtc.cssRotation.value).toBe(0)
  })
})

describe('pickRttMs · 从 getStats 报告取 RTT', () => {
  // RTCStatsReport 是 Map 形态（forEach/get），测试直接用 Map 构造。
  function report(entries: any[]): RTCStatsReport {
    return new Map(entries.map(e => [e.id, e])) as unknown as RTCStatsReport
  }

  it('优先用 transport 选中的候选对 currentRoundTripTime（秒→毫秒）', () => {
    const stats = report([
      { id: 't1', type: 'transport', selectedCandidatePairId: 'cp-sel' },
      { id: 'cp-sel', type: 'candidate-pair', currentRoundTripTime: 0.042 },
      { id: 'cp-other', type: 'candidate-pair', currentRoundTripTime: 0.999, nominated: true, state: 'succeeded' },
    ])
    expect(pickRttMs(stats)).toBe(42)
  })

  it('无 transport 选中时退回 nominated+succeeded 的候选对', () => {
    const stats = report([
      { id: 'cp1', type: 'candidate-pair', nominated: true, state: 'succeeded', currentRoundTripTime: 0.118 },
    ])
    expect(pickRttMs(stats)).toBe(118)
  })

  it('再退回 remote-inbound-rtp 的 roundTripTime', () => {
    const stats = report([
      { id: 'r1', type: 'remote-inbound-rtp', roundTripTime: 0.2 },
    ])
    expect(pickRttMs(stats)).toBe(200)
  })

  it('取不到任何 RTT 时返回 null', () => {
    expect(pickRttMs(report([{ id: 'x', type: 'codec' }]))).toBeNull()
    expect(pickRttMs(report([]))).toBeNull()
  })
})

describe('rttToLevel · 三档映射', () => {
  it('按阈值分档', () => {
    expect(rttToLevel(null)).toBeNull()
    expect(rttToLevel(0)).toBe('good')
    expect(rttToLevel(79)).toBe('good')
    expect(rttToLevel(80)).toBe('fair')
    expect(rttToLevel(199)).toBe('fair')
    expect(rttToLevel(200)).toBe('poor')
    expect(rttToLevel(500)).toBe('poor')
  })
})
