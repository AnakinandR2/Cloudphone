# 远控旋转重做 + 窗口过宽修复 设计文档

日期：2026-06-20
范围：`my/` C 端云手机远程控制（独立弹窗）

## 背景：三个问题

1. **窗口越来越宽（反馈环 bug）**。已是横屏的手机进远控后窗口被撑到满屏。日志定位：
   `chromeW = window.outerWidth - window.innerWidth`，但 `resizeTo` 后 `outerWidth` 立即更新、
   `innerWidth` 滞后一帧；`@resize` 又让 `onVideoReady` 连发多次。于是第二次起读到
   「新 outer − 旧 inner = 虚高 chrome（14→434→854）」，窗口 `1030→1450→1870` 滚雪球
   （`1870×dpr1.5=2805` 物理像素 = 截图满屏宽）。`resizeTo` 本身工作正常，是测量自污染。
2. **桌面等不可旋转场景，旋转按钮无视觉反馈**。当前画面方向只跟随推流尺寸；锁定竖屏的桌面
   推流不交换尺寸，点旋转「没反应」，不符合认知。希望前端始终把界面转过来。
3. **旋转按钮图标随状态变化**。希望换成固定的「设备旋转」图标。

## 决策（已与用户确认）

- 旋转按钮设的「手动方向」**粘性保留**（不随设备自转清除）；**首次手动前**仍自动跟随推流。
- 旋转按钮**同时**：① 前端转画面；② 给设备发 `rotate_device`（能转的 App 真转）。
- 图标固定，用 `tablet-smartphone`。

## 方向状态模型（`useWebRTC.ts`）

- `streamLandscape: Ref<boolean>` —— 由 `syncOrientation` 按推流尺寸（宽>高）判定（原 `landscape`）。
- `desiredLandscape: Ref<boolean | null>` —— `null`=跟随推流（默认）；旋转按钮置为固定 `true/false`（粘性）。
- `displayLandscape = computed(desiredLandscape ?? streamLandscape)` —— 实际渲染方向。
- `cssRotation = computed(displayLandscape === streamLandscape ? 0 : -90)` —— 仅当「想要的方向」与
  「推流给的方向」不一致时，才在前端把 `<video>` CSS 旋转 -90° 补齐。推流追上后自动归 0，**永不双重旋转**。
- `rotateDevice()`：`desiredLandscape = !displayLandscape`；若 dc 就绪则 `send({type:'rotate_device', angle: target ? -90 : 0})`；恒返回成功（前端方向总会变）。
- `cleanup()`：`desiredLandscape=null`、`streamLandscape=false`。
- 对外暴露：`displayLandscape`、`cssRotation`、`rotateDevice`、`syncOrientation`（保留）。

## 坐标补偿（`useRemoteInput.ts`）

新增入参 `cssRotation: Ref<number>`。`resolvePos` 改为**中心枢轴反旋转**，把显示点反算回推流像素：

1. `rect = video.getBoundingClientRect()`（CSS 变换后的外接框），中心 `c=(rect.left+rect.width/2, rect.top+rect.height/2)`（旋转不变）。
2. 屏幕向量 `s=(clientX-cx, clientY-cy)`；按 `-cssRotation` 反旋转到元素布局系：
   `lx = sx·cosθ + sy·sinθ`，`ly = −sx·sinθ + sy·cosθ`（θ=cssRotation，y 向下）。
3. 布局盒尺寸用 `video.offsetWidth/offsetHeight`（不受 transform 影响）；局部坐标
   `localX=offsetW/2+lx, localY=offsetH/2+ly`；越界返回 null。
4. 布局盒→推流像素（object-contain，盒按推流比例 → 通常精确无黑边）：
   `displayScale=min(offsetW/vw, offsetH/vh)`，扣黑边偏移后 `px,py` 钳制 `[0,vw-1]/[0,vh-1]`。
5. 下发 `{x:round(px), y:round(py), width:vw, height:vh, rotation:0}` —— 设备永远收到干净的
   推流系坐标（沿用已验证修复的 rotation=0 语义）。`cssRotation=0` 时该式退化为原有竖屏逻辑。

## 窗口尺寸（`remoteWindowFit.ts` + `RemoteControlView.vue`）

- `computeRemoteWindowSize` 增加显式入参 `landscape`（= `displayLandscape`），不再由推流 aspect 推断方向；
  短边按推流长短边比例 `longRatio=max(r,1/r)` 计算：`landscape ? (videoW=longEdge, videoH=longEdge/longRatio) : (videoH=longEdge, videoW=longEdge/longRatio)`。
- **chrome 反馈环修复**：窗口装饰尺寸只在首次稳定态测量一次并缓存复用，不再每次 `resizeTo` 后重读 `outer-inner`。
- fit 调用按 `(streamW, streamH, panel, displayLandscape)` 去重，避免重复 `resizeTo`。

## 画面显示（`RemoteControlView.vue`）

- `cssRotation===0`：沿用现有 in-flow + `object-contain` + `aspectRatio` 布局。
- `cssRotation!==0`：`<video>` 绝对居中，`width=frameH, height=frameW`（用 `ResizeObserver` 测 frame 尺寸），
  `object-contain` + `transform: rotate(cssRotation)` → 竖屏推流铺满横屏画面区。
- 旋转按钮图标换 `tablet-smartphone`，去掉 `:class="landscape ? '-rotate-90'"` 状态类。
- 删除全部 `[RC-fit]/[RC-open]` 临时调试日志（含 `PhoneView.vue`）。

## 测试

- `remoteWindowFit.test.ts`：改为传 `landscape`，覆盖竖/横、强制横屏（竖屏推流+landscape=true）、缩放、面板。
- `useWebRTC.test.ts`：`desiredLandscape` 粘性、`displayLandscape`/`cssRotation` 派生、`rotateDevice` 发指令与切换。
- `useRemoteInput.test.ts`：`cssRotation=-90` 时中心枢轴反算（角点映射）、`cssRotation=0` 维持原行为。

## 不做（YAGNI）

- 不引入手动 CSS 旋转下拉、不支持 90/180/270 任意角（只 0 / -90）。
- 不做窗口居中布局大改（`resizeTo` 已可用，chrome 修复即可）。
</content>
</invoke>
