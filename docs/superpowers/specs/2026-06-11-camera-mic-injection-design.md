# 摄像头/麦克风注入（直播）设计

- 日期：2026-06-11
- 目标：操作员浏览器采集本机摄像头 + 麦克风，上行注入云手机虚拟摄像头/麦克风，使云手机的摄像头/麦克风输入变成电脑的输入。
- 参考：`docs/WebRTC远控协议规范-v1.2.md` §5.5（摄像头/麦克风注入）+ `docs/cphone-webrtc-sdk/src/device/device-manager.js`（官方 SDK 参考实现）。

## 1. 背景与协议要点

方向是**客户端 → 设备的媒体上行**（与「接收设备屏幕」相反）。协议三要素（§5.5）：

1. **预留上行视频轨**：连接建立时在 `RTCPeerConnection` 上 `addTrack(16×16 黑屏占位轨)`，创建对应 SDP video m-line（设备 offer 为 sendrecv）的 sender。占位轨流量极小（1–5 Kbps）。
2. **注入/关闭用 `replaceTrack` 免重协商**：注入 = `getUserMedia` 采本地摄像头 → `sender.replaceTrack(摄像头轨)`；关闭 = `replaceTrack(占位黑屏轨)`。**严禁 removeTrack/addTrack**（会触发 SDP 重协商）。
3. **DataChannel 控制信令**：`{ "type":"camera_control", "action":"open"|"close" }` 通知设备把上行轨路由到虚拟摄像头。

**麦克风**（随摄像头，无独立开关）：`getUserMedia({audio:true})` 采麦克风，PCM 经 **DataChannel 二进制帧 `binary_pcm`** 传输（不是 WebRTC 音频轨）。

**分辨率**：竖屏注入 352×640；当连接分辨率为 720×1544 或 1080×2316 时取 290×618；`frameRate` 跟随连接 fps。

**镜像**：自拍式翻转通过 canvas 处理摄像头流后再 replaceTrack。

**硬性时序**：占位轨必须在 `createAnswer` **之前** addTrack，answer SDP 才带上行轨。

## 2. 接入方案

**方式 B（自研，扩展现有 useWebRTC）**。仓库已有裸 `RTCPeerConnection` 的 `useWebRTC.ts`（协议与本规范一致），pc/dc 都在手里，扩展即可；不引入官方 SDK（后者是含 server.js/UI 生成器的完整 app，非干净库）。

## 3. 架构与文件边界

### 3.1 新建 `my/src/composables/useCameraInjection.ts`

独立 composable，专管上行媒体注入；由 useWebRTC 组合（保持 useWebRTC 聚焦，注入纯逻辑可独立测试）。

构造入参（访问器，读 useWebRTC 的私有 pc/dc/参数）：

```ts
useCameraInjection({
  getPc: () => RTCPeerConnection | null,
  getDc: () => RTCDataChannel | null,
  getFps: () => number,
  getDims: () => { w: number, h: number },
  onError?: (msg: string) => void,
})
```

对外状态与方法：

- 状态：`cameraEnabled: Ref<boolean>`、`cameraList: Ref<MediaDeviceInfo[]>`、`selectedCameraId: Ref<string>`、`mirrorEnabled: Ref<boolean>`、`previewStream: Ref<MediaStream|null>`、`cameraBusy: Ref<boolean>`。
- 方法：`attachPlaceholder()`、`reset()`、`refreshCameraList()`、`openCamera()`、`closeCamera()`、`toggleCamera()`、`setCamera(deviceId)`、`setMirror(on)`。

内部实现（严格对齐 SDK device-manager.js）：

- `attachPlaceholder()`：`canvas(16×16 黑).captureStream(1)` → `pc.addTrack(track, stream)` → 存 `cameraSender` 与 `placeholderTrack`。幂等（已存在则跳过）。
- `openCamera()`：`getUserMedia({ video:{ deviceId?, width:{ideal}, height:{ideal}, frameRate:{ideal:fps} }, audio:true })` → 可选镜像 → `cameraSender.replaceTrack(track)` → `dc.send(JSON {camera_control, open})` → `startAudioCapture(stream)`；设 `previewStream`、`cameraEnabled=true`。
- `closeCamera()`：`stopAudioCapture()` → `cameraSender.replaceTrack(placeholderTrack)` → 停本地轨 → `dc.send(JSON {camera_control, close})` → `cameraEnabled=false`、`previewStream=null`。
- `setCamera/setMirror`：已开启时重采流再 replaceTrack；未开启仅存选择。
- 音频：`AudioContext` + `createScriptProcessor(1024, 2, 2)`，`onaudioprocess` 取 PCM（单声道直传、立体声交织）→ `buildBinaryPcmPacket` → `dc.send(packet)`；连到静默 destination 避免回声。
- `reset()`：closeCamera + 释放 placeholder/canvas + 清 sender 引用。

纯函数（模块级导出，便于单测）：

- `buildBinaryPcmPacket(float32, sampleRate, channels, seq) : ArrayBuffer` —— 10 字节 ASCII 前缀 `binary_pcm` + 24 字节小端头（magic `PCM1`=0x50434d31、version=1、flags=0、headerLen=24、seq、timestamp、sampleRate、channels、format=1(s16le)、payloadLen）+ s16le payload。
- `pickInjectResolution(w, h) : {width, height}` —— 720×1544/1080×2316 → 290×618，否则 352×640。
- `createMirroredStream(stream, mirror) : MediaStream` —— mirror=false 直通；true 走 canvas 水平翻转管线。

### 3.2 修改 `my/src/composables/useWebRTC.ts`

- 实例化 `useCameraInjection({...accessors})`。
- pc 建好、`setRemoteDescription/createAnswer` 之前调 `camera.attachPlaceholder()`。
- `cleanup()` 调 `camera.reset()`。
- 从 `useWebRTC` 返回里透传 camera 的状态与方法。

### 3.3 修改 `my/src/views/phone/RemoteControlView.vue`

- `Panel` 类型加 `'camera'`；工具栏加 Video 图标按钮（沿用 activePanel 模式）。
- 新增 camera 面板内容：①开启/关闭注入主按钮（busy 态 loading）；②摄像头设备 `<NativeSelect>`；③镜像开关（Switch）；④自拍预览 `<video>` 小窗（`previewStream`，muted、按镜像视觉翻转）；⑤提示「麦克风随摄像头一并注入」。
- 仅 `connected && dc 就绪` 可开启；面板打开时 `refreshCameraList()`。
- i18n（zh-CN + en）新增 camera 文案。

## 4. 数据流

```
操作员点「开启注入」
  → useCameraInjection.openCamera()
     getUserMedia(video+audio)
     → [镜像?] createMirroredStream
     → cameraSender.replaceTrack(videoTrack)        // 上行视频轨注入
     → dc.send({type:camera_control, action:open})  // 通知设备路由
     → startAudioCapture: ScriptProcessor → binary_pcm → dc.send(packet)  // 上行音频
  → previewStream 渲染到自拍预览小窗
关闭注入 / 断开连接 → closeCamera()/reset() 反向清理
```

## 5. 错误处理

- `getUserMedia` 拒绝按类型 toast：`NotAllowedError`→权限被拒；`NotFoundError`/`OverconstrainedError`→未找到可用摄像头；非安全上下文（`window.isSecureContext===false`）→提示需 HTTPS。
- DC 未就绪：注入按钮禁用，不发 camera_control。
- `enumerateDevices` 首次无 label：openCamera 授权后 `refreshCameraList()` 回填。
- 任一步失败：回滚（停已采流、cameraEnabled=false、previewStream=null）。

## 6. 测试

- 纯函数单测（`useCameraInjection.test.ts`）：
  - `buildBinaryPcmPacket`：前缀 10 字节 ASCII、magic/version/format 字段、sampleRate/channels、payload 长度、s16le 边界（+1→0x7fff、-1→-0x8000、0→0）。
  - `pickInjectResolution`：352×640 默认；720×1544 与 1080×2316 → 290×618。
- 媒体类 API（getUserMedia/canvas/AudioContext）jsdom 不支持，不做单测，靠手动联调（协议规范 §5.5 实测路径）。
- 前端 `pnpm build` 通过。

## 7. 范围与约束（明确不做）

- **仅单控 RemoteControlView**；群控不做（多路同采一个物理摄像头有冲突）。
- 设备端反向 `camera_control`（设备请求开摄像头）不做，仅操作员主动开关。
- `ScriptProcessorNode` 已废弃但与已验证 SDK 路径一致，先用；将来需要再换 AudioWorklet。
- 生产环境需 HTTPS 才能 getUserMedia（开发 localhost 可用）。

## 8. 验收标准

1. 前端 `pnpm build` 与单测通过。
2. 单控页连接后：打开 camera 面板 → 选设备/镜像 → 开启注入 → 自拍预览出画 → 云手机相机 App 看到电脑摄像头画面、麦克风为电脑输入。
3. 关闭注入或断开连接：占位轨切回、本地摄像头/麦克风释放、`camera_control close` 已发。
4. 占位轨在 answer SDP 之前 addTrack（上行 m-line 为 sendrecv）。
