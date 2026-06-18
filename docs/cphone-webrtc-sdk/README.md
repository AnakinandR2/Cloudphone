# cphone-webrtc-sdk

云手机中台 **WebRTC 远程画面 / 控制 SDK**（渠道接入用)。

加载后在 `window.CphoneWebRTC` 暴露 API,封装了:WebRTC 信令握手、视频流渲染、鼠标/键盘/触摸输入、剪贴板、旋转、多设备群控等。渠道只需调 `init()` + `connectAll()` 即可拿到远程画面与控制能力,**无需自己实现 WebRTC 信令**。

> 本 SDK 由中台 `XXSDK` 重命名而来(全局对象 `window.XXSDK` → `window.CphoneWebRTC`)。功能代码基本未改;2026-06-11 基线升级到新版 xx-sdk,**新增摄像头 / 麦克风注入**(8 个方法,见快速开始第 5 步 + API 文档 §4.7),并对 `deviceId` 做了归一化(`1`/`"1"`/`"device1"` 均可)。

---

## 目录结构

```
cphone-webrtc-sdk/
├── index.js                 ← 入口:加载后挂载 window.CphoneWebRTC(渠道引入这个)
├── package.json
├── src/
│   ├── main.js              ← 应用主逻辑(initApp / 全局函数)
│   ├── core/config.js       ← 默认配置常量(ICE / 编码器 / 分辨率)
│   ├── device/device-manager.js  ← WebRTC 连接 + 信令(核心)
│   ├── input/               ← 鼠标 / 键盘 / 坐标变换
│   ├── ui/                  ← 内置 UI(可选,渠道可自渲染)
│   ├── math/ utils/         ← 矩阵 / 存储工具
│   └── server.js            ← ⚠️ 仅 demo 用的本地 Node 服务,生产不需要
└── public/                  ← 内置 demo UI 的 css/img 资源(自渲染时可不用)
```

> **核心 vs demo**:渠道接入只需 `index.js` + `src/`(除 `server.js`)。`src/server.js`、`src/ui/page-generator*`、`public/` 是 SDK 自带的演示页面/本地服务,渠道用自己的页面时可不引入。

---

## 快速开始

### 1. 引入 SDK

```html
<!-- 把 cphone-webrtc-sdk/ 放到你的静态资源目录,引入 index.js -->
<script type="module" src="/sdk/cphone-webrtc-sdk/index.js"></script>

<!-- ⭐ 关键:视频元素 id 必须是 screen{deviceId},如 device1 → id="screen1"。
     SDK 的 device-manager 把视频流绑到 #screen{num};id 不对 = 能连上但黑屏。 -->
<video id="screen1" autoplay playsinline muted></video>
```

加载成功后控制台输出 `CphoneWebRTC module loaded successfully`,全局对象为 `window.CphoneWebRTC`。

> 🔴 **必读(2026-06-11 浏览器实测验证)**:**视频元素 id 必须是 `screen{deviceId}`**(device1 对应 `id="screen1"`)。SDK 内部 `device-manager` 把媒体流 `srcObject` 绑到 `document.querySelector('#screen{num}')`。**id 写错(如用了 remoteVideo1)→ WebRTC 会连接成功、控制台显示"视频连接成功",但画面黑屏**(实测踩坑)。多设备则 `screen1 / screen2 / ...`。

### 2. 拿鉴权票据(调云手机中台 OpenAPI)

```js
// POST /open/api/vendor/v1/cloud-phone/webrtc-auth  { cpIds:[cpId] }
const auth = (await api.post('/cloud-phone/webrtc-auth', { cpIds:[cpId] })).data[0]
// auth = { cpId, vmId, signalUrl, authToken, zoneId, pushStreamUrl, hasRunningTask, hasUpcomingTask }
```

> 鉴权接口详见《渠道接入手册》§3.6.1。`hasRunningTask=true` 时建议自动锁屏避免干扰自动化任务。

### 3. 初始化并连接

```js
window.CphoneWebRTC.init({
  roomList: [{
    deviceId: 1,
    token: auth.authToken,
    wsUrl: auth.signalUrl,
    roomId: `${auth.vmId}:${auth.cpId}`,   // ⭐ 必须 vmId:cpId
    isMain: true,
    width: 720, height: 1280, fps: 30, qualityLevel: 30,
    isGroupControl: false,
    isKickOff: false,                       // 通道满时是否踢掉旧连接
  }],
  onInitSuccess: () => window.CphoneWebRTC.connectAll(),
  onConnectSuccess: (deviceId, connected, status, cpId) => { /* status:"视频连接成功" */ },
  onConnectFailed: (deviceId, message, cpId) => { /* 弹窗重连 */ },
  onKickOff: (deviceId, kicked) => { /* 本端被踢下线 */ },
})
```

### 4. 控制设备

```js
// 物理按键 / 系统键
window.CphoneWebRTC.sendControlMessage(1, "button_home")    // 见 API 文档控制字典
// 旋转 / 销毁
window.CphoneWebRTC.rotateDevice("device1")
window.CphoneWebRTC.destroy(1)   // 不传 deviceId = 销毁全部
```

### 5. 摄像头 / 麦克风注入(可选,把本地摄像头喂进云手机)

```js
const cams = await window.CphoneWebRTC.getCameraList(1)        // 列本地摄像头
await window.CphoneWebRTC.openCamera(1, cams[0]?.deviceId)     // 打开并注入(含麦克风)
await window.CphoneWebRTC.toggleCameraMirror(1)               // 自拍镜像翻转
await window.CphoneWebRTC.closeCamera(1)                       // 关闭
```
> 让云手机里的 App(TikTok 等)以为是真摄像头/麦克风。需先连接成功。完整 8 个方法见 API 文档 §4.7。

完整 API 见 **《cphone-webrtc-sdk-API文档-v1.2.md》**(同 `v3.25-对外交付/` 目录)。
不想用本 SDK 的渠道,可照 **《WebRTC远控协议规范-v1.2.md》** 自研裸 WebRTC 接入(协议完全一致)。

---

## 注意事项

1. **ICE 强制 relay**:`src/core/config.js` 的 `ICE_CONFIG.iceTransportPolicy = "relay"` —— 强制走 TURN 中继。TURN server 地址 + 短期凭证由信令 `joined.data.server / data.token` 下发,SDK 自动处理。
2. **config.js 里的 `WS_URL` 默认值是内网 demo 地址**,实际连接用 `roomList[].wsUrl`(即 webrtc-auth 返回的 signalUrl),默认值不生效。
3. **authToken 一次性**:连接断开后重连需重新调 webrtc-auth 拿新 token。
4. **通道数限制**:同一 cp 默认 `WEBRTC_CHANNELS_NUM=1`,第二个连接需 `isKickOff:true` 踢掉旧的。
5. **浏览器兼容**:依赖标准 WebRTC(RTCPeerConnection / DataChannel),现代 Chromium / Firefox / Safari 支持;具体兼容矩阵以中台官方说明为准。
