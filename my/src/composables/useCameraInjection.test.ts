import { describe, expect, it } from 'vitest'
import { buildBinaryPcmPacket, pickInjectResolution } from './useCameraInjection'

describe('pickInjectResolution · 注入分辨率', () => {
  it('全面屏 720×1544 / 1080×2316 取 290×618', () => {
    expect(pickInjectResolution(720, 1544)).toEqual({ width: 290, height: 618 })
    expect(pickInjectResolution(1080, 2316)).toEqual({ width: 290, height: 618 })
  })
  it('其余取默认 352×640', () => {
    expect(pickInjectResolution(720, 1280)).toEqual({ width: 352, height: 640 })
    expect(pickInjectResolution(360, 640)).toEqual({ width: 352, height: 640 })
    expect(pickInjectResolution(1080, 1920)).toEqual({ width: 352, height: 640 })
  })
})

describe('buildBinaryPcmPacket · binary_pcm 封包', () => {
  it('前缀 / 头部字段 / payload 长度正确', () => {
    const samples = new Float32Array([0, 1, -1]) // 3 个采样
    const buf = buildBinaryPcmPacket(samples, 48000, 1, 7, 0x1234)
    const view = new DataView(buf)

    // 总长 = 10(前缀) + 24(头) + 3*2(payload)
    expect(buf.byteLength).toBe(10 + 24 + 6)

    // 前缀 "binary_pcm"
    const prefix = Array.from({ length: 10 }, (_, i) => String.fromCharCode(view.getUint8(i))).join('')
    expect(prefix).toBe('binary_pcm')

    // 头部（小端）：magic=PCM1, version=1, flags=0, headerLen=24
    expect(view.getUint32(10, true)).toBe(0x50434D31)
    expect(view.getUint8(14)).toBe(1)
    expect(view.getUint8(15)).toBe(0)
    expect(view.getUint16(16, true)).toBe(24)
    expect(view.getUint32(18, true)).toBe(7) // seq
    expect(view.getUint32(22, true)).toBe(0x1234) // timestamp
    expect(view.getUint16(26, true)).toBe(48000) // sampleRate
    expect(view.getUint8(28)).toBe(1) // channels
    expect(view.getUint8(29)).toBe(1) // format s16le
    expect(view.getUint32(30, true)).toBe(6) // payloadLen
  })

  it('s16le 量化边界：0→0, +1→0x7fff, -1→-0x8000', () => {
    const buf = buildBinaryPcmPacket(new Float32Array([0, 1, -1]), 44100, 1, 0, 0)
    const view = new DataView(buf)
    const base = 34 // 10 + 24
    expect(view.getInt16(base, true)).toBe(0)
    expect(view.getInt16(base + 2, true)).toBe(0x7FFF)
    expect(view.getInt16(base + 4, true)).toBe(-0x8000)
  })

  it('立体声通道数与采样率如实写入', () => {
    const buf = buildBinaryPcmPacket(new Float32Array([0.5, -0.5]), 44100, 2, 1, 1)
    const view = new DataView(buf)
    expect(view.getUint16(26, true)).toBe(44100)
    expect(view.getUint8(28)).toBe(2)
  })
})
