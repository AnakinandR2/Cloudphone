# 云手机中台 — WebRTC 远控协议规范(v1.2)

> **文档版本**:v1.2(2026-06-11)
> **适用读者**:需要自研接入云手机中台 WebRTC 远程画面/控制的渠道开发团队
> **本文档定位**:**信令 + 数据通道控制协议的完整规范,渠道商照本可不依赖任何官方 SDK 自研实现**
> **配套文档**:《云手机中台-渠道接入手册-v3.25.13》§3.6(本规范是 §3.6.2 的完整展开)
>
> **v1.2 更新(2026-06-11)**:📷 新增**摄像头 / 麦克风注入协议**(见 §5.6)—— 客户端→设备方向的视频轨(replaceTrack)+ `camera_control` 信令 + `binary_pcm` 音频。
> **v1.1**:经 aiortc(标准 WebRTC 栈)对真机端到端验证通过(ICE/TURN 中继 + DTLS + 解码 30 帧 720×1280);设备同时推**视频 + 音频**两条媒体轨(见 §3 步骤 9)。

---

## 0. 文档来源与可信度声明

本规范**不是中台官方下发的协议文档**,而是产品技术专家**从两个真实对接项目的代码 + 测试环境实测白盒还原**得到的:

| 来源 | 性质 | 用途 |
|---|---|---|
| `tk-manager-web`(渠道前端,直连中台) | 用中台官方 `XXSDK` 接入 | 印证消息常量、roomId 格式、控制词汇 |
| `web` + `cloudphone-glory-service`(自研前端 + Go 后端) | **裸 `RTCPeerConnection` 手写信令**,未用 XXSDK | ⭐ **协议白盒还原的主依据** |
| 测试环境实测(`webrtc-auth` 等接口) | Python probe | 印证鉴权出参字段 |

**关键发现**:两个项目**客户端 SDK 不一致**(一个用 XXSDK、一个裸 WebRTC 手写),但**底层信令协议 + 数据通道控制词汇 100% 一致**。这证明本协议**可白盒还原、可自研实现** —— `web` 项目就是活的参考实现。

> 📦 **2026-06-11 更新**:上文 `tk-manager-web` 用的官方 `XXSDK` 已确认可对外,并重命名为 **`cphone-webrtc-sdk`**(全局对象 `window.CphoneWebRTC`)随本批次交付(`v3.25-对外交付/cphone-webrtc-sdk/` + 《cphone-webrtc-sdk-API文档-v1.2.md》)。下文凡提 XXSDK 即指该 SDK。

> ⚠️ **可信度边界**:本规范基于上述两个项目当前(2026-04~06)版本的实现还原。SDP/ICE 标准部分遵循 W3C WebRTC,可信;中台**自定义信令消息**(`join`/`welcome`/控制字典)以本规范为准,但若与中台后续官方文档冲突,**以官方为准**。建议接入前向中台索取/比对官方《WebRTC 信令通道协议规范》。

---

## 1. 接入总览

### 1.1 角色与连接拓扑

```
┌─────────────┐   ① webrtc-auth (HTTPS)   ┌──────────────┐
│  渠道客户端  │ ────────────────────────► │  云手机中台   │
│ (浏览器/SDK) │ ◄──────────────────────── │  OpenAPI     │
└──────┬──────┘   authToken/signalUrl/      └──────────────┘
       │          vmId/cpId
       │ ② WebSocket 信令 (wss://signalUrl)
       ▼
┌─────────────┐   join/offer/answer/ice   ┌──────────────┐
│  信令服务器  │ ◄────────────────────────►│  云手机(设备) │
└─────────────┘                            │  Android 端   │
       │ ③ WebRTC P2P(媒体流 + 数据通道)    └──────┬───────┘
       └────────────────────────────────────────────┘
                  视频流(设备→客户端)
                  控制消息(客户端→设备,经 DataChannel)
```

**三步接入**:
1. **HTTPS 鉴权** → 调 `webrtc-auth` 拿 `authToken` + `signalUrl`(详见 §2)
2. **WebSocket 信令** → 连 `signalUrl`,完成 SDP/ICE 协商(详见 §3、§4)
3. **WebRTC P2P** → 接收视频流 + 经 DataChannel 下发控制(详见 §5)

### 1.2 关键约定速记

| 约定 | 值 | 说明 |
|---|---|---|
| **roomId 格式** | `{vmId}:{cpId}` | 冒号拼接,信令全程用它标识房间 |
| **谁是 offerer** | **设备端(Android)** | ⚠️ 云手机先发 offer,客户端是 answerer |
| **谁开 DataChannel** | **设备端** | 客户端 `ondatachannel` 被动接收 |
| **控制消息方向** | 客户端 → 设备 | 经 DataChannel 发 JSON |
| **视频流方向** | 设备 → 客户端 | `ontrack` 接收 |

---

## 2. 第一步:鉴权(webrtc-auth)

详见《渠道接入手册》§3.6.1。此处给协议规范必需的摘要。

| | |
|---|---|
| **HTTP** | `POST /open/api/vendor/v1/cloud-phone/webrtc-auth` |
| **入参** | `{ "cpIds": ["cp-xxx"] }` |

**实测出参**(2026-06-11,`data` 为数组,每个 cp 一条):

| 字段 | 实测值示例 | 信令阶段用途 |
|---|---|---|
| `cpId` | `cp-81095411499002` | 拼 roomId |
| `vmId` | `c19c8b69-...` | 拼 roomId |
| `signalUrl` | `wss://test-webrtc-signaling.cphone.cn/v1` | ② WebSocket 连接地址 |
| `authToken` | `ODI4OmEzY2Vm...`(base64 一次性票据) | join 消息的 `data` 字段 |
| `zoneId` | `7` | 可用区(SDK 内部可能用)|
| `pushStreamUrl` | `""`(测试环境为空)| RTMP 推流场景才有值 |
| `hasRunningTask` | `false` | ⭐ true=该 cp 正跑自动化任务,客户端应**自动锁屏**防干扰 |
| `hasUpcomingTask` | `false` | ⭐ true=即将跑任务,客户端**弹窗提示** |

> 🔴 **安全提醒(P0-A23)**:本接口当前**不校验 cpId 归属**,存在跨租户屏幕接管风险。渠道侧务必只传自家 §2.6 查询返回的 cpId,等中台网关中间件上线后闭塞。

---

## 3. 第二步:信令握手时序

完整时序(基于 `web/useWebRTC.ts` 还原):

```
客户端                          信令服务器                      云手机(设备)
  │                                │                                │
  │ 1. WebSocket 连接 signalUrl    │                                │
  │ ──────────────────────────────►│                                │
  │                                │                                │
  │ 2. send join                   │                                │
  │ {type:join, roomId, data:      │                                │
  │  authToken, meta:{...}}        │                                │
  │ ──────────────────────────────►│                                │
  │                                │ 唤起/通知设备入房               │
  │                                │ ──────────────────────────────►│
  │ 3. recv welcome/joined         │                                │
  │ {data:{server:TURN,            │                                │
  │  token:TURN短期凭证}}          │                                │
  │ ◄──────────────────────────────│                                │
  │                                │ 4. 设备生成 SDP offer           │
  │ 5. recv offer                  │ ◄──────────────────────────────│
  │ {from:设备sid, data:SDP}       │                                │
  │ ◄──────────────────────────────│                                │
  │                                │                                │
  │ 6. 建 RTCPeerConnection        │                                │
  │    setRemoteDescription(offer) │                                │
  │    createAnswer                │                                │
  │ 7. send answer                 │                                │
  │ {type:answer, roomId,          │                                │
  │  to:设备sid, data:SDP}         │                                │
  │ ──────────────────────────────►│ ──────────────────────────────►│
  │                                │                                │
  │ 8. ICE candidate 双向交换       │                                │
  │ {type:ice-candidate, ...}      │                                │
  │ ◄─────────────────────────────►│ ◄─────────────────────────────►│
  │                                │                                │
  │ 9. P2P 建立 → ontrack 收视频+音频两条流 + ondatachannel 收控制通道 │
  │ ◄══════════════════════════════════════════════════════════════►│
  │                                │                                │
  │   (会话进行中:DataChannel 下发控制,视频流持续)                  │
  │                                │                                │
  │ N. recv remote-closed → 设备推流结束,清理                        │
  │ ◄──────────────────────────────│                                │
```

---

## 4. 信令消息详解(WebSocket over signalUrl)

所有消息均为 **JSON 文本帧**。以下字段基于 `web` 项目实测实现。

### 4.1 `join`(客户端 → 服务器,入房)

```json
{
  "type": "join",
  "clientType": "web",
  "roomId": "c19c8b69-...:cp-81095411499002",
  "data": "ODI4OmEzY2Vm...",
  "meta": {
    "clipboardAutosync": true,
    "resolution": { "width": 720, "height": 1280, "fps": 30 },
    "encoderConfig": {
      "targetBitrate": 2000000,
      "keyFrameInterval": 10,
      "qualityLevel": 30,
      "targetFps": 30
    }
  }
}
```

| 字段 | 说明 |
|---|---|
| `data` | ⭐ **webrtc-auth 返回的 authToken**,作为入房鉴权 |
| `roomId` | `{vmId}:{cpId}` |
| `meta.resolution` | 期望分辨率 + 帧率(可选值见《手册》§3.6.4 画质配置)|
| `meta.encoderConfig.qualityLevel` | 清晰度(10/30/50,对应极速/流畅/标清)|
| `meta.encoderConfig.targetBitrate` | 目标码率(bps),实测 2_000_000(2Mbps)|

### 4.2 `welcome` / `joined`(服务器 → 客户端,入房确认 + TURN 下发)

```json
{ "type": "joined", "data": { "server": "turn:turn.cphone.cn:3478", "token": "{roomId}_<HMAC>_<nanos>" } }
```

| 字段 | 说明 |
|---|---|
| `data.server` | TURN 服务器地址(可空,内网环境可能不下发)|
| `data.token` | ⭐ **TURN 短期凭证**,见 §6 的鉴权坑 |

### 4.3 `offer`(服务器/设备 → 客户端,SDP 邀约)

```json
{ "type": "offer", "from": "<设备会话ID>", "data": { "type": "offer", "sdp": "v=0\r\no=..." } }
```

- ⚠️ **offer 由设备端发起**,客户端是 answerer
- `from` = 设备会话 ID,后续 `answer` / `ice-candidate` 的 `to` 字段要回填它
- `data` = 标准 RTCSessionDescription

### 4.4 `answer`(客户端 → 服务器,SDP 应答)

```json
{ "type": "answer", "roomId": "...", "to": "<设备会话ID>", "data": { "type": "answer", "sdp": "..." } }
```

### 4.5 `ice-candidate`(双向)

```json
{
  "type": "ice-candidate",
  "roomId": "...",
  "to": "<对端会话ID>",
  "data": { "type": "candidate", "label": 0, "id": "0", "candidate": "candidate:..." }
}
```

| `data` 字段 | 对应 W3C |
|---|---|
| `label` | `sdpMLineIndex` |
| `id` | `sdpMid` |
| `candidate` | `candidate` 字符串 |

### 4.6 `remote-closed`(服务器 → 客户端)

```json
{ "type": "remote-closed" }
```

设备端推流结束,客户端应清理连接。

---

## 5. 第三步:数据通道控制协议(DataChannel)

P2P 建立后,**设备端创建 DataChannel**(客户端 `pc.ondatachannel` 接收)。控制消息为 **JSON 文本**,客户端 → 设备。

### 5.1 按键 / 系统按钮

```json
{ "type": "button_power" }
```

| `type` | 含义 |
|---|---|
| `button_power` | 电源 |
| `button_home` | 主页 |
| `button_back` | 返回 |
| `button_recent` | 最近任务 |
| `button_volume_up` | 音量 + |
| `button_volume_down` | 音量 − |

> 💡 **静音实现**:无"读系统音量"接口,客户端通过连发多次 `button_volume_down` 实现静音,缓存档位用于恢复(`web` 项目做法)。

### 5.2 键盘

| `type` | 附加字段 | 含义 |
|---|---|---|
| `key_back` / `key_home` / `key_enter` | — | 对应物理键 |
| `key_backspace` / `key_delete` / `key_tab` / `key_space` | — | — |
| `key_arrow` | `direction: up\|down\|left\|right` | 方向键 |
| `key_char` | `char: "a"`, `code: "KeyA"` | 单字符输入 |

### 5.3 鼠标 / 触摸(归一化坐标)

```json
{ "type": "mouse_down", "x": 360, "y": 640, "button": 0, "rotation": 0,
  "width": 720, "height": 1280, "messageId": "<唯一ID>" }
```

| `type` | 关键字段 | 说明 |
|---|---|---|
| `mouse_down` | x,y,button,width,height,messageId | 按下;`messageId` 标识本次按下 |
| `mouse_move` | x,y,deltaX,deltaY,downEventId,messageId | 移动/拖拽;`downEventId` 关联对应 down |
| `mouse_up` | x,y,button,downEventId,messageId | 抬起 |
| `mouse_double_click` | x,y,width,height,downEventId,messageId | 双击 |
| 滚动 | x,y,deltaY,intensity(1/2/3),downEventId,messageId | 滚轮;intensity 按 deltaY 分级 |

> ⚠️ **坐标系**:`x/y` 是设备坐标,需带上 `width/height`(当前流分辨率)让设备端换算。`rotation` 当前固定 0。触摸事件复用 `mouse_*` 类型。

### 5.4 剪贴板(双向)

| 方向 | 消息 |
|---|---|
| 客户端 → 设备:读设备剪贴板 | `{ "type": "clipboard", "action": "get" }` |
| 客户端 → 设备:写设备剪贴板 | `{ "type": "clipboard", "action": "set", "content": "..." }` |
| 设备 → 客户端:剪贴板内容回传 | `{ "type": "clipboard_content", "content": "..." }` |

### 5.5 摄像头 / 麦克风注入(camera injection)📷 自研要点

把**操作员本地摄像头 + 麦克风**注入云手机虚拟摄像头(App 当成真摄像头)。与"接收设备屏幕"方向相反 —— 这是**客户端 → 设备**的媒体上行。

**协议三要素**:

1. **预留上行视频轨**(连接建立时):在 `RTCPeerConnection` 上 `addTrack(占位黑屏轨)` 创建一个 sender(对应 SDP video m-line 的 `sendrecv`,实测设备 offer 正是 sendrecv)。占位轨流量极小(1-5 Kbps)。
2. **注入 / 关闭**(用 `replaceTrack` 免重协商):
   - 注入:`getUserMedia({video:{...}, audio:true})` 采集本地摄像头 → `sender.replaceTrack(摄像头轨)`
   - 关闭:`sender.replaceTrack(占位黑屏轨)`
   - ⚠️ **用 replaceTrack 不要 removeTrack/addTrack**,后者会触发 SDP 重协商
3. **DataChannel 控制信令**:
   ```json
   { "type": "camera_control", "action": "open" }   // 或 "close"
   ```
   通知设备端把上行视频轨路由到虚拟摄像头。

**麦克风(随摄像头)**:`getUserMedia` 带 `audio:true` 采麦克风,音频 PCM 经 **DataChannel 二进制帧 `binary_pcm`** 传输(非 WebRTC 音频轨)。无"只注入摄像头不注入麦克风"的独立开关。

**镜像**:自拍式镜像翻转通过 canvas 处理摄像头流(`createMirroredStream`)后再 replaceTrack。

**分辨率适配**(SDK 实测):竖屏注入分辨率取 352×640(或 720×1544/1080×2316 时取 290×618),`frameRate` 跟随连接的 fps。

**设备侧依赖**:云手机设备端必须支持 `camera_control` + 虚拟摄像头路由(infra 层);中台已部署。

> ✅ **实测验证**:真实 Chrome(`--use-fake-device-for-media-stream` 假摄像头)对真机完成:占位轨预留 → getCameraList → getUserMedia → replaceTrack → camera_control 发设备 → enabled=true → closeCamera 切回占位。全链路通过。

---

## 6. ⚠️ TURN 短期凭证鉴权(最易踩的坑)

`web` 项目代码注释明确记录了一个**历史踩坑**,渠道自研务必注意:

> 公网 TURN 启用了**短期凭证鉴权**:`RTCIceServer.username` 必须是**信令下发的 `joined.data.token`**(格式 `{roomId}_<HMAC>_<nanos>`),**不是** webrtc-auth REST 返回的 `authToken`。内网 TURN 时期不鉴权,所以历史代码用错也能跑,迁公网就会失败。

正确构造 iceServers:

```js
const iceServers = [{ urls: 'stun:stun.l.google.com:19302' }]
if (turnServer) {           // turnServer = joined.data.server
  iceServers.push({
    urls: turnServer,
    username: turnToken,     // ⭐ joined.data.token,不是 authToken!
    credential: roomId,
  })
}
```

| 配错点 | 现象 |
|---|---|
| `username` 用了 authToken | 内网能连、**公网 TURN 中继失败**(候选拿不到)|
| 不下发 TURN(纯 STUN) | NAT 穿透失败时无中继兜底,连接率下降 |

---

## 7. 多客户端互踢(isKickOff)

同一 cp 的 WebRTC 通道数有限(实测 `WEBRTC_CHANNELS_NUM=1`,即同时只允许 1 个连接)。

**判断是否需要踢掉已有连接**(从《手册》§2.6 `/cloud-phone/page` 读):

| 字段 | 实测 | 含义 |
|---|---|---|
| `webrtcChannelsNum` | `1` | 最大通道数 |
| `webrtcCount` | `0` | 当前已用 |

- 当 `webrtcCount === webrtcChannelsNum`(已满)→ 连接时置 `isKickOff=true`,踢掉旧连接
- 被踢端通过 SDK 的 `onKickOff` 回调(或自研监听 `remote-closed`)感知下线

---

## 8. 画质配置(供画质下拉框)

详见《手册》§3.6.4。`POST /open/api/vendor/v1/vendor-config/list` + `{module:"WEBRTC连接"}` 返回 10 条:

| code | 实测值 |
|---|---|
| `WEBRTC_RESOLUTION_9_16_OPTIONAL` | `360*640,720*1280,1080*1920` |
| `WEBRTC_RESOLUTION_9_19_3_OPTIONAL` | `720*1544,1080*2316` |
| `WEBRTC_FPS_OPTIONAL` | `10,15,...,60` |
| `WEBRTC_CLARITY_OPTIONAL` | `10,30,50` |

> 按 cp 实际屏幕方向(9:16 竖屏 / 19.3:9 全面屏)取对应分辨率组,填入 join 的 `meta.resolution`。

---

## 9. 连接结果回报

详见《手册》§3.6.5。连接成功/失败后回报中台:

```
POST /open/api/vendor/v1/cloud-phone/connect-notify
{ "cpList": [ { "cpId": "cp-xxx", "state": true } ] }   // state: true=成功 false=失败
```

建议在 P2P `connectionState==='connected'` 时报 `true`、握手/连接失败时报 `false`。

---

## 10. 参考实现(裸 RTCPeerConnection,基于 web 项目)

最小可用伪代码(JS),证明**不依赖官方 SDK 也能接入**:

```js
// ① 鉴权
const { signalUrl, authToken, vmId, cpId } = (await webrtcAuth(cpId)).data[0]
const roomId = `${vmId}:${cpId}`

// ② 信令
const ws = new WebSocket(signalUrl)
let pc, turnServer, turnToken, deviceSid

ws.onopen = () => ws.send(JSON.stringify({
  type: 'join', clientType: 'web', roomId, data: authToken,
  meta: { clipboardAutosync: true,
    resolution: { width, height, fps },
    encoderConfig: { targetBitrate: 2_000_000, keyFrameInterval: 10, qualityLevel, targetFps: fps } }
}))

ws.onmessage = async (ev) => {
  const msg = JSON.parse(ev.data)
  if (msg.type === 'joined' || msg.type === 'welcome') {
    turnServer = msg.data?.server; turnToken = msg.data?.token
  }
  if (msg.type === 'offer') {
    deviceSid = msg.from
    const iceServers = [{ urls: 'stun:stun.l.google.com:19302' }]
    if (turnServer) iceServers.push({ urls: turnServer, username: turnToken, credential: roomId }) // ⚠️ §6
    pc = new RTCPeerConnection({ iceServers })
    pc.ontrack = (e) => { videoEl.srcObject = e.streams[0] }
    pc.onicecandidate = (e) => e.candidate && ws.send(JSON.stringify({
      type: 'ice-candidate', roomId, to: deviceSid,
      data: { type: 'candidate', label: e.candidate.sdpMLineIndex, id: e.candidate.sdpMid, candidate: e.candidate.candidate } }))
    pc.ondatachannel = (e) => { window.dc = e.channel /* 控制通道,见 §5 */ }
    await pc.setRemoteDescription(new RTCSessionDescription(msg.data))
    const answer = await pc.createAnswer(); await pc.setLocalDescription(answer)
    ws.send(JSON.stringify({ type: 'answer', roomId, to: deviceSid, data: answer }))
  }
  if (msg.type === 'ice-candidate' && msg.data?.type === 'candidate')
    await pc.addIceCandidate(new RTCIceCandidate({ sdpMLineIndex: msg.data.label, candidate: msg.data.candidate }))
  if (msg.type === 'remote-closed') { pc?.close(); ws.close() }
}

// ③ 控制(P2P 建立后)
function sendButton(b) { dc.send(JSON.stringify({ type: `button_${b}` })) }  // 见 §5
```

---

## 11. 两种接入方式(✅ 官方 SDK cphone-webrtc-sdk 已交付)

> ✅ **2026-06-11 中台确认:官方 SDK 可对外交付,已重命名为 `cphone-webrtc-sdk`(`window.CphoneWebRTC`)随本批次交付。** 因此**推荐渠道优先用方式 A(官方 SDK)快速接入**;方式 B(自研协议)作为深度定制 / 不愿引入外部 SDK 时的等价替代(协议完全一致,见前文 §3~§9)。

| 维度 | 方式 A:官方 SDK(cphone-webrtc-sdk)⭐推荐 | 方式 B:自研(本规范)|
|---|---|---|
| 代表项目 | `tk-manager-web` | `web` |
| 接入成本 | **低**(`CphoneWebRTC.init` + `connectAll`)| 中(自己实现信令 + DC)|
| 控制 API | `CphoneWebRTC.sendControlMessage(1, "button_power")` | `dc.send({type:"button_power"})` |
| 依赖 | cphone-webrtc-sdk(✅ 本批次已交付 + API 文档)| 仅标准 WebRTC API,零依赖 |
| 可定制性 | 受 SDK 封装限制 | 完全可控 |
| 适用 | **快速上线(默认推荐)** | 需深度定制、或不愿引入外部 SDK |

### 11.1 方式 A — 官方 SDK 接入(`cphone-webrtc-sdk`)

> ✅ **SDK 已对外交付**:`v3.25-对外交付/cphone-webrtc-sdk/`(由中台 `XXSDK` 重命名,全局对象 `window.CphoneWebRTC`)+ 完整 **《cphone-webrtc-sdk-API文档-v1.2.md》**(从源码逐行提取)。以下为速记,完整 API 见该文档。

**初始化 + 连接**:
```js
window.CphoneWebRTC.init({
  roomList: [{
    deviceId: 1,                       // 设备序号(多设备分屏时递增)
    token: authToken,                  // ← §2 webrtc-auth 返回的 authToken
    wsUrl: signalUrl,                  // ← §2 webrtc-auth 返回的 signalUrl
    roomId: `${vmId}:${cpId}`,         // ⭐ 必须 vmId:cpId
    isMain: true,
    width, height, fps,                // ← §8 画质配置
    qualityLevel,                      // 清晰度 10/30/50
    isGroupControl: false,             // 是否群控
    isKickOff,                         // ← §7 是否踢掉已有连接
  }],
  onInitSuccess: () => CphoneWebRTC.connectAll(),
  onConnectSuccess: (deviceId, connected, status) => { /* status: "已连接" / "视频连接成功" */ },
  onConnectFailed: (deviceId, message, cpId) => { /* 弹窗重连 */ },
  onKickOff: (deviceId, isKickOff) => { /* 本端被踢下线 */ },
})
```

**控制指令**:
```js
CphoneWebRTC.sendControlMessage(1, "button_power")   // button_power/home/back/recent/volume_up/volume_down(同 §5.1 字典)
```

**画面 / 旋转辅助**(观察到的方法,具体语义以官方文档为准):
```js
CphoneWebRTC.rotateDevice("device1")            // 旋转设备
CphoneWebRTC.getDeviceIsLandscape(1)            // 是否横屏
CphoneWebRTC.getVideoResolution(1)              // 当前视频分辨率(0=未旋转)
CphoneWebRTC.getRotation("device1")             // 当前旋转角度
CphoneWebRTC.destroy()                          // 销毁连接
```

> 📌 **渠道接入 checklist(方式 A)**:① 引入已交付的 cphone-webrtc-sdk(见其 README + API 文档);② 调 §2 `webrtc-auth` 拿 token/signalUrl;③ `CphoneWebRTC.init`(roomId 用 `vmId:cpId`);④ 任务锁定信号 `hasRunningTask/hasUpcomingTask` 自行处理(见 §2);⑤ 连接成功/失败回报 §9 `connect-notify`。

### 11.2 方式 B — 自研接入

照本规范 §3~§9 + §10 参考实现,用裸 `RTCPeerConnection` 自研。`web` 项目已验证可行,零外部依赖。

---

## 12. FAQ / 排障

| 现象 | 排查 |
|---|---|
| 连接卡在"等待设备" | 设备未入房/未起推流;确认 cp 是 NORMAL 且 join 的 authToken 有效(authToken 一次性,过期需重新 webrtc-auth)|
| 内网正常公网连不上 | ⚠️ §6 TURN 凭证:`username` 必须用 `joined.data.token` 不是 authToken |
| 视频黑屏但已连接 | `ontrack` 未挂流 / video 元素 `muted` 未设;先 muted 自动播放再用户手势解除静音 |
| 控制无响应 | DataChannel 未 open(`dc.readyState!=='open'`);控制消息 type/字段名拼错(见 §5)|
| 同一 cp 第二个客户端进不来 | §7 通道已满,需 `isKickOff` 踢旧连接 |
| 19.3:9 全面屏画面带黑边 | join 的 `meta.resolution` 高度要按设备真实比例换算(`web` 项目:`nH/nW>2.0` 时高=宽×真实比)|
| 远控干扰了自动化任务 | §2 `hasRunningTask=true` 时应自动锁屏,见手册 §3.6.1 |

---

> **本规范随对接迭代更新。发现与实际行为不符,请联系中台对接人核对官方协议文档。**
