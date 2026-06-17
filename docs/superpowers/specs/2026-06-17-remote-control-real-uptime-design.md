# 远程控制真实开机时长显示 — 设计

日期：2026-06-17
状态：已通过设计评审，待写实现计划

## 背景与问题

远控窗口 [RemoteControlView.vue](../../../my/src/views/phone/RemoteControlView.vue) 顶部有一个「使用计时」，当前实现（约 384–397 行）是 `elapsed` 从 0 开始、每秒 +1 的**纯前端模拟**，统计的是"本次打开远控窗口的时长"，**与云手机真实开机时长无关**：打开一台已运行 3 小时的手机，计时也只从 00:00:00 起。

后端已同步中台运行日志（§2.9）：

- 本地 `RunSession`（[run_session.go](../../../backend/modules/phone/internal/run_session.go)）：worker 周期同步，`PowerOnAt` 为真实开机时间，`PowerOffAt IS NULL` 表示运行中。
- 实时接口 `GET /:id/run-logs`（[op_api.go](../../../backend/modules/phone/internal/op_api.go) `RunLogsCloudPhone` → `PhoneService.RunLogs` → `s.ops.RunLogs`）：直连中台分页查，最新一条 `powerOnTime` 即真实开机时间。

目标：用中台运行日志的真实开机时间，把远控计时改为**真实开机总时长**。

## 需求决策（已确认）

- **计时口径**：显示真实开机总时长（自中台 `powerOnTime` 起算到现在），而非"本次远控会话时长"。
- **数据源**：打开远控时**实时查中台运行日志**取最新一条的开机时间（最准、刚开机即有），不依赖本地 `RunSession` 同步延迟。
- **取不到时**：显示占位 `--:--:--`（刚开机日志未生成 / 最新日志已关机 / 中台查询失败）。

## 方案

采用**新增轻量后端接口**：服务端实时查中台运行日志并**由服务端计算开机时长秒数**返回，前端只读秒数本地累加。相比前端直接解析 `run-logs` 字符串，规避客户端时区/时钟偏差，契约更干净，且不拖慢/污染 `GET /:id` detail。

### 1. 接口契约

`GET /api/v1/phone/:id/runtime`（前台，需登录，按属主隔离）

响应 `data`（`RuntimeInfo`）：

```jsonc
{
  "running": true,                          // 当前是否运行中（最新运行日志未关机）
  "power_on_at": "2026-06-17T11:00:00+08:00", // RFC3339，仅 running 时给（便于展示/调试）
  "uptime_seconds": 10860                    // 服务端算：now − powerOnAt，clamp ≥ 0
}
```

非运行 / 取不到：`{ "running": false, "uptime_seconds": 0 }`（`power_on_at` 省略）。
中台调用出错：返回错误（`framework.FailErr`），前端 catch 兜底为占位。

### 2. 后端

- **路由**（[module.go](../../../backend/modules/phone/internal/module.go) 用户组）：
  `g.GET("/:id/runtime", RuntimeCloudPhone)`。
- **handler** `RuntimeCloudPhone`（op_api.go）：复用 `opCloudPhone(c)` 取 `uid,id` → `PhoneService.Runtime(uid, id)` → `framework.OKWithData`。
- **service** `Runtime(userID, id int) (*RuntimeInfo, error)`：
  1. `resolveCp(userID, id)` 校属主并拿 `cpID`。
  2. 实时 `s.ops.RunLogs(ctx, cpID, 1, 1)` 取该 cp 最新一条日志。
  3. 取 `Data[0]`：用 `parseRunLogTime` 解析 `PowerOnTime`；`PowerOffTime` 为空/「运行中」且开机时间可解析 → 运行中。
  4. 运行中 → `uptime = max(0, now − powerOnAt)`（秒），返回 `{running:true, power_on_at: RFC3339, uptime_seconds}`。
  5. 否则（无日志 / 最新已关机 / 解析失败）→ `{running:false, uptime_seconds:0}`。
  6. 中台 `RunLogs` 出错 → 返回 err（handler 走 `FailErr`）。
- **DTO** `RuntimeInfo`（复用 `runLogTimeLayout` / `time.Local`）。

### 3. 前端

- [api/modules/phone.ts](../../../my/src/api/modules/phone.ts)：加 `runtime: (id) => api.get<unknown, R<RuntimeInfo>>(\`phone/${id}/runtime\`)`。
- [types/phone.ts](../../../my/src/types/phone.ts)：加 `RuntimeInfo`（snake_case 对齐后端）。
- [mock/phone.ts](../../../my/src/mock/phone.ts)：加 `/v1/phone/:id/runtime` mock（返回一个已运行若干秒的运行中会话）。
- [RemoteControlView.vue](../../../my/src/views/phone/RemoteControlView.vue)：
  - 新增 `hasRealtime` 标志（默认 false）。
  - `onMounted` 调 `detail` 后再调 `runtime(id)`：`running` → `elapsed = uptime_seconds`、`hasRealtime = true`，启动现有每秒 +1 的 `setInterval`；非运行/出错 → `hasRealtime = false`，**不启动 `setInterval`**（`elapsedText` 直接显示占位）。
  - `elapsedText`：`!hasRealtime` → `'--:--:--'`，否则按 `HH:MM:SS` 格式化。
  - 注释由"本次使用计时（前端模拟）"改为"真实开机时长（中台运行日志锚定，服务端给秒数）"。
  - i18n `phone.rc.timerTip` 文案改为"已开机 {time}"（zh）/ "Powered on for {time}"（en），双语同步。

### 4. 边界 / 错误处理

- 取不到真实时间 → `--:--:--`。
- 时区/时钟：服务端给 `uptime_seconds`，前端纯本地累加，无客户端偏差。
- 计时锚定一次（mount 时）；单次远控会话内本地累加足够精确。

### 5. 测试

- **后端** `Runtime` service 测试（用现有 `fakePort.RunLogs`）：
  - 最新日志 `powerOnTime` 在过去且 `powerOffTime` 为空 → `running=true`、`uptime_seconds>0`。
  - 最新日志已关机 → `running=false`、`uptime_seconds=0`。
  - 无日志 → `running=false`。
- **前端**：`pnpm build`（vue-tsc）+ 现有 vitest 通过。

## 不做（YAGNI）

- 不在云手机列表/卡片上展示真实时长（本期仅远控窗口）。
- 不做计时器周期性再同步 / 跨重连重新锚定（单次会话本地累加已足够）。
- 不引入新的本地表或缓存；不改 `GET /:id` detail。
