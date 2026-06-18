// useWebRTC.ts
//
// 云手机远程控制的 WebRTC 连接管理。信令协议对齐中台官方实现（cp-glory-service）：
//   webrtc-auth 拿 signalUrl/authToken → WebSocket join → 收 offer → 建 RTCPeerConnection
//   → 媒体轨道渲染到 <video>，控制经远端建立的 DataChannel 下行（sendDC）。
import { computed, type Ref, ref } from 'vue'
import phoneApi from '@/api/modules/phone'
import type { WebRTCAuth } from '@/types/phone'
import { useCameraInjection } from './useCameraInjection'

export type ConnState = 'disconnected' | 'connecting' | 'connected' | 'failed'
export type LatencyLevel = 'good' | 'fair' | 'poor'

// pickRttMs 从一次 getStats() 报告里取出 WebRTC 往返时延（毫秒）。
// 优先用 transport 选中的候选对（媒体实际走的那条 ICE 路径）的 currentRoundTripTime，
// 退而求其次用 nominated/succeeded 的候选对，再不行用 remote-inbound-rtp 的 roundTripTime。
// WebRTC 里这些字段单位是「秒」，统一 ×1000 取整为毫秒；取不到返回 null。
export function pickRttMs(stats: RTCStatsReport): number | null {
  const pairs = new Map<string, any>()
  let selectedPairId: string | undefined
  let nominatedRtt: number | null = null
  let remoteInboundRtt: number | null = null

  stats.forEach((r: any) => {
    if (r.type === 'transport' && r.selectedCandidatePairId)
      selectedPairId = r.selectedCandidatePairId
    else if (r.type === 'candidate-pair') {
      pairs.set(r.id, r)
      if (nominatedRtt == null && (r.nominated || r.selected) && r.state === 'succeeded' && typeof r.currentRoundTripTime === 'number')
        nominatedRtt = r.currentRoundTripTime * 1000
    }
    else if (r.type === 'remote-inbound-rtp' && remoteInboundRtt == null && typeof r.roundTripTime === 'number') {
      remoteInboundRtt = r.roundTripTime * 1000
    }
  })

  if (selectedPairId) {
    const p = pairs.get(selectedPairId)
    if (p && typeof p.currentRoundTripTime === 'number')
      return Math.round(p.currentRoundTripTime * 1000)
  }
  const ms = nominatedRtt ?? remoteInboundRtt
  return ms == null ? null : Math.round(ms)
}

// rttToLevel 把时延毫秒映射成三档网络状况。阈值按云手机串流体验经验取。
export function rttToLevel(ms: number | null): LatencyLevel | null {
  if (ms == null)
    return null
  if (ms < 80)
    return 'good'
  if (ms < 200)
    return 'fair'
  return 'poor'
}

export interface UseWebRTCOptions {
  id: Ref<number>
  videoRef: Ref<HTMLVideoElement | null>
  onError?: (msg: string) => void
}

export function useWebRTC(opts: UseWebRTCOptions) {
  const { id, videoRef, onError } = opts

  const connState = ref<ConnState>('disconnected')
  const connStatusText = ref('未连接')

  let ws: WebSocket | null = null
  let pc: RTCPeerConnection | null = null
  let dc: RTCDataChannel | null = null
  let connTimeout: ReturnType<typeof setTimeout> | null = null

  // 网络往返时延（毫秒，null=未测得）+ 三档状况，由 pc.getStats() 周期采样。
  const rttMs = ref<number | null>(null)
  const latencyLevel = computed(() => rttToLevel(rttMs.value))
  let statsTimer: ReturnType<typeof setInterval> | null = null

  async function sampleRtt() {
    if (!pc)
      return
    try {
      rttMs.value = pickRttMs(await pc.getStats())
    }
    catch {
      /* getStats 偶发失败忽略，保留上次值 */
    }
  }
  function startStats() {
    stopStats()
    sampleRtt()
    statsTimer = setInterval(sampleRtt, 2000)
  }
  function stopStats() {
    if (statsTimer) {
      clearInterval(statsTimer)
      statsTimer = null
    }
    rttMs.value = null
  }

  // 串流参数：分辨率 / 质量(qualityLevel) / 帧率，可在连接前后调整（改后需重连生效）。
  const resOptions = [
    { label: '360x640', w: 360, h: 640 },
    { label: '720x1280', w: 720, h: 1280 },
    { label: '1080x1920', w: 1080, h: 1920 },
  ]
  // 画质三挡：仅展示档位名（高清 / 标清 / 流畅），不暴露具体 qualityLevel 数值。
  const qualityOptions = [
    { value: 50, label: '高清' },
    { value: 30, label: '标清' },
    { value: 10, label: '流畅' },
  ]
  const fpsOptions = Array.from({ length: 11 }, (_, i) => 10 + i * 5) // 10..60 步进 5

  const selectedRes = ref('720x1280')
  const selectedQuality = ref(30)
  const selectedFps = ref(30)

  const deviceWidth = computed(() => resOptions.find(r => r.label === selectedRes.value)?.w ?? 720)
  const deviceHeight = computed(() => resOptions.find(r => r.label === selectedRes.value)?.h ?? 1280)

  const isMuted = ref(true)
  function toggleMute() {
    if (!videoRef.value)
      return
    isMuted.value = !isMuted.value
    videoRef.value.muted = isMuted.value
    if (!isMuted.value) {
      videoRef.value.play().catch(() => {
        if (videoRef.value)
          videoRef.value.muted = true
        isMuted.value = true
      })
    }
  }

  let hasAutoUnmuted = false
  function tryAutoUnmute() {
    if (hasAutoUnmuted || !videoRef.value || connState.value !== 'connected')
      return
    hasAutoUnmuted = true
    videoRef.value.muted = false
    isMuted.value = false
    videoRef.value.play().catch(() => {
      if (videoRef.value)
        videoRef.value.muted = true
      isMuted.value = true
    })
  }

  const connected = computed(() => connState.value === 'connected')

  // 控制通道（DataChannel）是否就绪——摄像头注入依赖它发 camera_control + binary_pcm。
  const controlReady = ref(false)

  // 屏幕方向：竖屏(false)/横屏(true)。以实时推流尺寸为唯一真相——设备或应用「自动转屏」时
  // 推流分辨率交换，syncOrientation 经 video 的 resize/loadedmetadata 自动更新本值（无需手点旋转）。
  const landscape = ref(false)

  // 按实时推流尺寸自动判定屏幕方向（宽>高=横屏）。在 pc.ontrack 里挂到 video 的
  // resize/loadedmetadata 事件，设备内应用转横屏导致分辨率交换时即自动跟随。
  function syncOrientation() {
    const v = videoRef.value
    if (v && v.videoWidth && v.videoHeight)
      landscape.value = v.videoWidth > v.videoHeight
  }

  function rotateDevice(): boolean {
    if (!dc || dc.readyState !== 'open')
      return false
    // 目标方向取当前实时方向的反向。-90=横屏，0=竖屏（与 SDK rotate_device 一致）。
    // 旋转后推流分辨率交换，landscape 由 syncOrientation 自动校正，故此处不再乐观翻转。
    const angle = landscape.value ? 0 : -90
    try {
      dc.send(JSON.stringify({ type: 'rotate_device', angle }))
      return true
    }
    catch {
      return false
    }
  }

  // 摄像头/麦克风注入（上行）。读取本 composable 的私有 pc/dc 与串流参数。
  const camera = useCameraInjection({
    getPc: () => pc,
    getDc: () => dc,
    getFps: () => selectedFps.value,
    getDims: () => ({ w: deviceWidth.value, h: deviceHeight.value }),
  })

  // sendDC 经远端建立的数据通道下发一条 JSON 控制消息（触摸/按键/系统键）。
  function sendDC(msg: Record<string, unknown>): boolean {
    if (!dc || dc.readyState !== 'open')
      return false
    try {
      dc.send(JSON.stringify(msg))
      return true
    }
    catch {
      return false
    }
  }

  async function connect() {
    if (connState.value === 'connecting' || connState.value === 'connected')
      return
    connState.value = 'connecting'
    connStatusText.value = '获取认证…'

    let auth: WebRTCAuth | null = null
    try {
      const res = await phoneApi.webrtcAuth(id.value)
      auth = res.data
    }
    catch (e) {
      connState.value = 'failed'
      connStatusText.value = '认证失败'
      onError?.(String(e))
      return
    }
    if (!auth || !auth.signalUrl) {
      connState.value = 'failed'
      connStatusText.value = '信令不可用'
      return
    }

    const roomId = `${auth.vmId}:${auth.cpId}`
    const authToken = auth.authToken
    let turnServer: string | null = null
    let turnToken: string | null = null
    let androidSessionId: string | null = null

    connTimeout = setTimeout(() => {
      if (connState.value === 'connecting') {
        connState.value = 'failed'
        connStatusText.value = '连接超时'
        cleanup()
      }
    }, 30000)

    connStatusText.value = '连接信令…'
    ws = new WebSocket(auth.signalUrl)

    ws.onopen = () => {
      connStatusText.value = '等待设备…'
      ws?.send(JSON.stringify({
        type: 'join',
        clientType: 'web',
        roomId,
        data: authToken,
        meta: {
          clipboardAutosync: false,
          resolution: { width: deviceWidth.value, height: deviceHeight.value, fps: selectedFps.value },
          encoderConfig: { targetBitrate: 2_000_000, keyFrameInterval: 10, qualityLevel: selectedQuality.value, targetFps: selectedFps.value },
        },
      }))
    }

    ws.onmessage = async (event: MessageEvent) => {
      let msg: any
      try {
        msg = JSON.parse(event.data)
      }
      catch {
        return
      }
      if (msg.type === 'welcome' || msg.type === 'joined') {
        if (msg.data?.server)
          turnServer = msg.data.server
        if (msg.data?.token)
          turnToken = msg.data.token
        return
      }
      if (msg.type === 'offer') {
        androidSessionId = msg.from || null
        connStatusText.value = 'WebRTC 握手…'
        const iceServers: RTCIceServer[] = [{ urls: 'stun:stun.l.google.com:19302' }]
        if (turnServer)
          iceServers.push({ urls: turnServer, username: turnToken || authToken || '', credential: roomId })
        pc = new RTCPeerConnection({ iceServers })
        pc.ontrack = (e: RTCTrackEvent) => {
          if (videoRef.value && e.streams[0]) {
            videoRef.value.srcObject = e.streams[0]
            videoRef.value.muted = true
            isMuted.value = true
            // 自动转屏检测：推流分辨率变化（如应用切横屏）即重新判定方向。
            videoRef.value.addEventListener('resize', syncOrientation)
            videoRef.value.addEventListener('loadedmetadata', syncOrientation)
            videoRef.value.play().catch(() => {})
          }
        }
        pc.onicecandidate = (e) => {
          if (e.candidate && ws?.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({
              type: 'ice-candidate',
              roomId,
              to: androidSessionId,
              data: {
                type: 'candidate',
                label: e.candidate.sdpMLineIndex,
                id: e.candidate.sdpMid,
                candidate: e.candidate.candidate,
              },
            }))
          }
        }
        pc.ondatachannel = (e: RTCDataChannelEvent) => {
          dc = e.channel
          dc.onopen = () => { connStatusText.value = '控制就绪'; controlReady.value = true }
          dc.onclose = () => { connStatusText.value = '控制断开'; controlReady.value = false }
        }
        pc.onconnectionstatechange = () => {
          const st = pc?.connectionState
          if (st === 'connected') {
            if (connTimeout) {
              clearTimeout(connTimeout)
              connTimeout = null
            }
            connState.value = 'connected'
            connStatusText.value = '已连接'
            startStats()
          }
          else if (st === 'failed') {
            connState.value = 'failed'
            connStatusText.value = '连接失败'
          }
          else if ((st === 'disconnected' || st === 'closed') && connState.value === 'connected') {
            connState.value = 'disconnected'
            connStatusText.value = '已断开'
          }
        }
        // 预留上行视频轨（占位黑屏），须在 createAnswer 之前，answer SDP 才带上行 m-line。
        camera.attachPlaceholder()
        await pc.setRemoteDescription(new RTCSessionDescription(msg.data))
        const answer = await pc.createAnswer()
        await pc.setLocalDescription(answer)
        ws?.send(JSON.stringify({ type: 'answer', roomId, to: androidSessionId, data: answer }))
        return
      }
      if (msg.type === 'ice-candidate' && msg.data?.type === 'candidate') {
        try {
          await pc?.addIceCandidate(new RTCIceCandidate({ sdpMLineIndex: msg.data.label, candidate: msg.data.candidate }))
        }
        catch {
          /* ignore */
        }
        return
      }
      if (msg.type === 'remote-closed') {
        connState.value = 'disconnected'
        connStatusText.value = '推流结束'
        cleanup()
      }
    }
    ws.onerror = () => {
      connState.value = 'failed'
      connStatusText.value = '信令错误'
    }
    ws.onclose = () => {
      if (connState.value === 'connecting') {
        connState.value = 'failed'
        connStatusText.value = '信令断开'
      }
    }
  }

  function cleanup() {
    stopStats()
    camera.reset()
    controlReady.value = false
    landscape.value = false
    if (connTimeout) {
      clearTimeout(connTimeout)
      connTimeout = null
    }
    try { dc?.close() }
    catch { /* ignore */ }
    dc = null
    try { pc?.close() }
    catch { /* ignore */ }
    pc = null
    try { ws?.close() }
    catch { /* ignore */ }
    ws = null
    if (videoRef.value) {
      videoRef.value.removeEventListener('resize', syncOrientation)
      videoRef.value.removeEventListener('loadedmetadata', syncOrientation)
      videoRef.value.srcObject = null
    }
    hasAutoUnmuted = false
  }

  function disconnect() {
    cleanup()
    connState.value = 'disconnected'
    connStatusText.value = '未连接'
  }

  function retry() {
    disconnect()
    setTimeout(() => connect(), 500)
  }

  return {
    connState,
    connStatusText,
    connected,
    controlReady,
    landscape,
    rotateDevice,
    syncOrientation,
    rttMs,
    latencyLevel,
    // 摄像头/麦克风注入（上行，相互独立）
    videoInjecting: camera.videoEnabled,
    audioInjecting: camera.audioEnabled,
    videoInjectBusy: camera.videoBusy,
    audioInjectBusy: camera.audioBusy,
    videoInputs: camera.videoList,
    audioInputs: camera.audioList,
    selectedVideoId: camera.selectedVideoId,
    selectedAudioId: camera.selectedAudioId,
    mirrorEnabled: camera.mirrorEnabled,
    cameraPreviewStream: camera.previewStream,
    refreshMediaDevices: camera.refreshDeviceList,
    toggleVideoInject: camera.toggleVideo,
    toggleAudioInject: camera.toggleAudio,
    setVideoDevice: camera.setVideoDevice,
    setAudioDevice: camera.setAudioDevice,
    setMirror: camera.setMirror,
    isMuted,
    resOptions,
    qualityOptions,
    fpsOptions,
    selectedRes,
    selectedQuality,
    selectedFps,
    deviceWidth,
    deviceHeight,
    connect,
    disconnect,
    retry,
    cleanup,
    toggleMute,
    tryAutoUnmute,
    sendDC,
  }
}
