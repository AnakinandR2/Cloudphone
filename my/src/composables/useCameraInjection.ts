// useCameraInjection.ts
//
// 把操作员本地摄像头 / 麦克风注入云手机（客户端 → 设备的媒体上行）。
// 协议见 docs/WebRTC远控协议规范-v1.2.md §5.5，实现对齐官方 cphone-webrtc-sdk。
//
// 摄像头与麦克风**相互独立**：可只注入摄像头、只注入麦克风、两者都注入或都不注入，
// 各自可选设备。（协议上麦克风走 binary_pcm DataChannel，摄像头走 replaceTrack 上行视频轨。）
//
// 视频三要素：
//   1. 连接建立时 addTrack(16×16 黑屏占位轨)，预留上行 video sender（设备 offer 为 sendrecv）。
//   2. 注入/关闭用 sender.replaceTrack 免重协商（严禁 removeTrack/addTrack）。
//   3. DataChannel 发 {type:"camera_control", action:"open"|"close"} 通知设备路由。
// 音频：getUserMedia 采麦克风，PCM 经 DataChannel 二进制帧 binary_pcm 传输（非 WebRTC 音频轨）。
import { ref } from 'vue'

export interface UseCameraInjectionOptions {
  getPc: () => RTCPeerConnection | null
  getDc: () => RTCDataChannel | null
  getFps: () => number
  getDims: () => { w: number, h: number }
}

// pickInjectResolution 按连接分辨率给出竖屏注入分辨率（SDK 实测值）。
export function pickInjectResolution(w: number, h: number): { width: number, height: number } {
  if ((w === 720 && h === 1544) || (w === 1080 && h === 2316))
    return { width: 290, height: 618 }
  return { width: 352, height: 640 }
}

// buildBinaryPcmPacket 按 binary_pcm 协议封包（纯函数）：
// 10 字节 ASCII 前缀 "binary_pcm" + 24 字节小端头 + s16le payload。
// float32：单声道 [s0,s1,...]，立体声交织 [L0,R0,L1,R1,...]。
export function buildBinaryPcmPacket(
  float32: Float32Array,
  sampleRate: number,
  channels: number,
  seq: number,
  timestamp: number,
): ArrayBuffer {
  const PREFIX = 'binary_pcm'
  const MAGIC = 0x50434D31 // "PCM1"
  const HEADER_LEN = 24
  const FORMAT_S16LE = 1
  const sampleCount = float32.length
  const payloadLen = sampleCount * 2

  const buf = new ArrayBuffer(10 + HEADER_LEN + payloadLen)
  const view = new DataView(buf)
  let o = 0
  for (let i = 0; i < PREFIX.length; i++) view.setUint8(o++, PREFIX.charCodeAt(i))
  view.setUint32(o, MAGIC, true); o += 4
  view.setUint8(o++, 1) // version
  view.setUint8(o++, 0) // flags
  view.setUint16(o, HEADER_LEN, true); o += 2
  view.setUint32(o, seq >>> 0, true); o += 4
  view.setUint32(o, timestamp >>> 0, true); o += 4
  view.setUint16(o, sampleRate, true); o += 2
  view.setUint8(o++, channels)
  view.setUint8(o++, FORMAT_S16LE)
  view.setUint32(o, payloadLen, true); o += 4
  for (let i = 0; i < sampleCount; i++) {
    const s = Math.max(-1, Math.min(1, float32[i]))
    view.setInt16(o, s < 0 ? s * 0x8000 : s * 0x7FFF, true)
    o += 2
  }
  return buf
}

export function useCameraInjection(opts: UseCameraInjectionOptions) {
  const { getPc, getDc, getFps, getDims } = opts

  // 视频 / 音频独立状态。
  const videoEnabled = ref(false)
  const audioEnabled = ref(false)
  const videoBusy = ref(false)
  const audioBusy = ref(false)
  const videoList = ref<MediaDeviceInfo[]>([])
  const audioList = ref<MediaDeviceInfo[]>([])
  const selectedVideoId = ref('')
  const selectedAudioId = ref('')
  const mirrorEnabled = ref(false)
  const previewStream = ref<MediaStream | null>(null)

  // 预留的上行 video sender + 占位轨。
  let cameraSender: RTCRtpSender | null = null
  let placeholderTrack: MediaStreamTrack | null = null
  let placeholderStream: MediaStream | null = null

  // 当前采集的视频/音频原始流（独立）。
  let videoStream: MediaStream | null = null
  let audioStream: MediaStream | null = null

  // 镜像管线句柄（mirror 开启且视频注入时存在）。
  let mirrorVideo: HTMLVideoElement | null = null
  let mirrorRaf = 0
  let mirrorStream: MediaStream | null = null

  // 音频（binary_pcm）相关。
  let audioContext: AudioContext | null = null
  let audioSource: MediaStreamAudioSourceNode | null = null
  let audioProcessor: ScriptProcessorNode | null = null
  let pcmSeq = 0

  // attachPlaceholder 在 pc 建好、createAnswer 之前调用：加 16×16 黑屏占位轨，
  // 让 answer SDP 带上行 video m-line（sendrecv）。幂等。
  function attachPlaceholder() {
    const pc = getPc()
    if (!pc || cameraSender)
      return
    try {
      const canvas = document.createElement('canvas')
      canvas.width = 16
      canvas.height = 16
      const ctx = canvas.getContext('2d')
      if (ctx) {
        ctx.fillStyle = 'black'
        ctx.fillRect(0, 0, 16, 16)
      }
      const stream = canvas.captureStream(1)
      const track = stream.getVideoTracks()[0]
      placeholderTrack = track
      placeholderStream = stream
      cameraSender = pc.addTrack(track, stream)
    }
    catch {
      /* 占位轨创建失败不阻断连接；注入功能届时不可用 */
    }
  }

  // createMirroredStream：mirror=false 直通；true 走 canvas 水平翻转管线（自拍式）。
  function createMirroredStream(stream: MediaStream): MediaStream {
    if (!mirrorEnabled.value)
      return stream
    const track = stream.getVideoTracks()[0]
    const s = track.getSettings()
    const w = s.width ?? 352
    const h = s.height ?? 640

    const video = document.createElement('video')
    video.srcObject = new MediaStream([track])
    video.muted = true
    video.playsInline = true
    video.play().catch(() => {})

    const canvas = document.createElement('canvas')
    canvas.width = w
    canvas.height = h
    const ctx = canvas.getContext('2d')!
    const draw = () => {
      ctx.save()
      ctx.scale(-1, 1)
      ctx.drawImage(video, -w, 0, w, h)
      ctx.restore()
      mirrorRaf = requestAnimationFrame(draw)
    }
    draw()

    mirrorVideo = video
    mirrorStream = canvas.captureStream(getFps())
    return mirrorStream
  }

  function stopMirror() {
    if (mirrorRaf) {
      cancelAnimationFrame(mirrorRaf)
      mirrorRaf = 0
    }
    if (mirrorVideo) {
      mirrorVideo.srcObject = null
      mirrorVideo = null
    }
    if (mirrorStream) {
      mirrorStream.getTracks().forEach(t => t.stop())
      mirrorStream = null
    }
  }

  // startAudioCapture：采麦克风 PCM，经 DataChannel 发 binary_pcm 帧。
  function startAudioCapture(stream: MediaStream) {
    const dc = getDc()
    if (stream.getAudioTracks().length === 0 || !dc || dc.readyState !== 'open')
      return
    try {
      const Ctor = window.AudioContext || (window as any).webkitAudioContext
      audioContext = new Ctor()
      audioSource = audioContext.createMediaStreamSource(stream)
      audioProcessor = audioContext.createScriptProcessor(1024, 2, 2)
      audioProcessor.onaudioprocess = (e) => {
        const ch = getDc()
        if (!ch || ch.readyState !== 'open' || !audioEnabled.value)
          return
        const input = e.inputBuffer
        const n = input.length
        let samples: Float32Array
        if (input.numberOfChannels === 1) {
          samples = new Float32Array(n)
          samples.set(input.getChannelData(0))
        }
        else {
          const L = input.getChannelData(0)
          const R = input.getChannelData(1)
          samples = new Float32Array(n * 2)
          for (let i = 0; i < n; i++) {
            samples[i * 2] = L[i]
            samples[i * 2 + 1] = R[i]
          }
        }
        const packet = buildBinaryPcmPacket(samples, audioContext!.sampleRate, input.numberOfChannels, pcmSeq++, Date.now() % 0xFFFFFFFF)
        try { ch.send(packet) }
        catch { /* 通道瞬断忽略 */ }
      }
      audioSource.connect(audioProcessor)
      // 连到静默 destination，避免本地扬声器回声。
      audioProcessor.connect(audioContext.createMediaStreamDestination())
    }
    catch {
      /* 音频采集失败不影响视频注入 */
    }
  }

  function stopAudioCapture() {
    try {
      if (audioProcessor) {
        audioProcessor.disconnect()
        audioProcessor.onaudioprocess = null
        audioProcessor = null
      }
      if (audioSource) {
        audioSource.disconnect()
        audioSource = null
      }
      if (audioContext) {
        audioContext.close().catch(() => {})
        audioContext = null
      }
    }
    catch { /* ignore */ }
  }

  // refreshDeviceList：枚举 videoinput / audioinput。未授权时 label 可能为空，授权后再调可回填。
  async function refreshDeviceList() {
    if (!navigator.mediaDevices?.enumerateDevices)
      return
    try {
      const devices = await navigator.mediaDevices.enumerateDevices()
      videoList.value = devices.filter(d => d.kind === 'videoinput')
      audioList.value = devices.filter(d => d.kind === 'audioinput')
      if (!selectedVideoId.value && videoList.value[0]?.deviceId)
        selectedVideoId.value = videoList.value[0].deviceId
      if (!selectedAudioId.value && audioList.value[0]?.deviceId)
        selectedAudioId.value = audioList.value[0].deviceId
    }
    catch { /* ignore */ }
  }

  // openVideo：采本地摄像头并注入（不含音频）。失败时回滚并抛出（由调用方 toast）。
  async function openVideo() {
    const pc = getPc()
    const dc = getDc()
    if (!pc || !cameraSender)
      throw new DOMException('WebRTC 连接未建立或上行轨未预留', 'InvalidStateError')
    if (!dc || dc.readyState !== 'open')
      throw new DOMException('控制通道未就绪', 'InvalidStateError')

    videoBusy.value = true
    try {
      stopMirror()
      if (videoStream) {
        videoStream.getTracks().forEach(t => t.stop())
        videoStream = null
      }
      const { width, height } = pickInjectResolution(getDims().w, getDims().h)
      const id = selectedVideoId.value
      videoStream = await navigator.mediaDevices.getUserMedia({
        video: {
          ...(id ? { deviceId: { exact: id } } : {}),
          width: { ideal: width },
          height: { ideal: height },
          frameRate: { ideal: getFps() },
        },
        audio: false,
      })
      const processed = createMirroredStream(videoStream)
      await cameraSender.replaceTrack(processed.getVideoTracks()[0])
      dc.send(JSON.stringify({ type: 'camera_control', action: 'open' }))
      previewStream.value = processed
      videoEnabled.value = true
      refreshDeviceList()
    }
    catch (err) {
      stopMirror()
      if (videoStream) {
        videoStream.getTracks().forEach(t => t.stop())
        videoStream = null
      }
      previewStream.value = null
      videoEnabled.value = false
      throw err
    }
    finally {
      videoBusy.value = false
    }
  }

  // closeVideo：切回占位轨、停采集、通知设备。幂等。
  async function closeVideo() {
    try {
      if (cameraSender && placeholderTrack)
        await cameraSender.replaceTrack(placeholderTrack)
    }
    catch { /* ignore */ }
    stopMirror()
    if (videoStream) {
      videoStream.getTracks().forEach(t => t.stop())
      videoStream = null
    }
    previewStream.value = null
    const dc = getDc()
    if (dc && dc.readyState === 'open') {
      try { dc.send(JSON.stringify({ type: 'camera_control', action: 'close' })) }
      catch { /* ignore */ }
    }
    videoEnabled.value = false
  }

  // openAudio：采本地麦克风并注入（不含视频）。失败时回滚并抛出。
  async function openAudio() {
    const dc = getDc()
    if (!dc || dc.readyState !== 'open')
      throw new DOMException('控制通道未就绪', 'InvalidStateError')

    audioBusy.value = true
    try {
      stopAudioCapture()
      if (audioStream) {
        audioStream.getTracks().forEach(t => t.stop())
        audioStream = null
      }
      const id = selectedAudioId.value
      audioStream = await navigator.mediaDevices.getUserMedia({
        video: false,
        audio: id ? { deviceId: { exact: id } } : true,
      })
      audioEnabled.value = true
      startAudioCapture(audioStream)
      refreshDeviceList()
    }
    catch (err) {
      stopAudioCapture()
      if (audioStream) {
        audioStream.getTracks().forEach(t => t.stop())
        audioStream = null
      }
      audioEnabled.value = false
      throw err
    }
    finally {
      audioBusy.value = false
    }
  }

  // closeAudio：停采集、释放麦克风。幂等。
  async function closeAudio() {
    audioEnabled.value = false
    stopAudioCapture()
    if (audioStream) {
      audioStream.getTracks().forEach(t => t.stop())
      audioStream = null
    }
  }

  async function toggleVideo() {
    if (videoEnabled.value)
      await closeVideo()
    else
      await openVideo()
  }

  async function toggleAudio() {
    if (audioEnabled.value)
      await closeAudio()
    else
      await openAudio()
  }

  // 设备/镜像切换：对应流已注入时重采再生效；未注入仅记录选择。
  async function setVideoDevice(deviceId: string) {
    selectedVideoId.value = deviceId
    if (videoEnabled.value)
      await openVideo()
  }

  async function setAudioDevice(deviceId: string) {
    selectedAudioId.value = deviceId
    if (audioEnabled.value)
      await openAudio()
  }

  async function setMirror(on: boolean) {
    mirrorEnabled.value = on
    if (videoEnabled.value)
      await openVideo()
  }

  // reset：连接清理时调用，彻底释放一切（含占位轨）。
  function reset() {
    stopAudioCapture()
    stopMirror()
    if (videoStream) {
      videoStream.getTracks().forEach(t => t.stop())
      videoStream = null
    }
    if (audioStream) {
      audioStream.getTracks().forEach(t => t.stop())
      audioStream = null
    }
    if (placeholderStream) {
      placeholderStream.getTracks().forEach(t => t.stop())
      placeholderStream = null
    }
    placeholderTrack = null
    cameraSender = null
    previewStream.value = null
    videoEnabled.value = false
    audioEnabled.value = false
    pcmSeq = 0
  }

  return {
    videoEnabled,
    audioEnabled,
    videoBusy,
    audioBusy,
    videoList,
    audioList,
    selectedVideoId,
    selectedAudioId,
    mirrorEnabled,
    previewStream,
    attachPlaceholder,
    reset,
    refreshDeviceList,
    toggleVideo,
    toggleAudio,
    setVideoDevice,
    setAudioDevice,
    setMirror,
  }
}
