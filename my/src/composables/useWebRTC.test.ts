import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { useWebRTC } from './useWebRTC'

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
