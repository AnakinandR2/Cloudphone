// useWebRTC.ts
//
// 云手机远程控制的 WebRTC 连接管理。信令协议对齐中台官方实现（cp-glory-service）：
//   webrtc-auth 拿 signalUrl/authToken → WebSocket join → 收 offer → 建 RTCPeerConnection
//   → 媒体轨道渲染到 <video>，控制经远端建立的 DataChannel 下行（sendDC）。
import { computed, type Ref, ref } from 'vue'
import phoneApi from '@/api/modules/phone'
import type { WebRTCAuth } from '@/types/phone'

export type ConnState = 'disconnected' | 'connecting' | 'connected' | 'failed'

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
          dc.onopen = () => { connStatusText.value = '控制就绪' }
          dc.onclose = () => { connStatusText.value = '控制断开' }
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
    if (videoRef.value)
      videoRef.value.srcObject = null
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
