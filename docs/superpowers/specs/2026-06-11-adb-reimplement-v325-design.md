# ADB 功能重构（对齐中台 v3.25 现状版）设计

- 日期：2026-06-11
- 范围：全量对齐 v3.25 + 重新启用前端
- 参考文档：`docs/云手机中台-渠道接入手册-v3.25-现状版.md` §3.5（ADB 管理）、§2.6（查询云手机 v2）

## 1. 背景

仓库已有一套 ADB 实现，对齐的是更早版本的中台 spec，目前前端被特性开关 `ADB_AVAILABLE=false` 禁用。
本次按 v3.25 现状版重构，并按产品要求**大幅收窄功能**：去掉白名单管理，只保留「开启 / 续期 / 关闭 + 连接引导」。

中台现状关键约束（来自 v3.25）：

- §3.5.1 统一操作 `POST /open/api/vendor/v1/adb/operate`，operation = `enable` / `disable` / `update_whitelist`（全小写）。
- 🟠 P1-A33：HTTP 200 + `code="200"` 但 `data.allSuccess=false`，真实错误埋在 `data.errorMessage` / `containerDetails[].error`，必须深入 data 判真假。
- §3.5.2 连接信息从 §2.6 v2 查询（`POST /open/api/vendor/v1/cloud-phone/page`）「接管入口」组字段获取：`adbAddress`（IP:Port）、`adbToken`、`adbTokenExpiredAt`。
- §3.5.3 查白名单 `GET /adb/whitelist/cp/{cpId}` 存在 🚨 P0-A27 跨租户读越权；🟠 P1-A10 不存在 cpId → HTTP 400 + `code="DATA_NOT_EXIST"`。
- ADB 安全建议：TTL 默认 86400 是上限，生产建议 86400 + 主动续期；连接方式为两步——先 `adb connect <地址>`，再 `adb shell xlogin <Token>`。
- 中台已不再提供 `/adb/token/enable` `/adb/token/disable`（仓库里这两段是死代码）。

## 2. 产品形态（前端）

ADB drawer 只有两种状态：

**未开启**：单个「开启 ADB」按钮（ttl 固定 86400 秒）。

**已开启**，展示：

1. 连接地址 + 端口：`adbAddress`（`IP:Port`），带复制按钮。
2. 登录 Token：`adbToken`，可隐藏/显示 + 复制。
3. 剩余有效时间：由 `adbTokenExpiredAt` 实时倒计时（每秒刷新）；到期变红并提示「已过期，请续期」。
4. 「续期」按钮：再次调用 enable（ttl=86400）刷新过期时间，回查后刷新展示。
5. 两步连接引导（各带复制按钮）：
   - 第一步：`adb connect <adbAddress>`
   - 第二步：`adb shell xlogin <adbToken>`
6. 「关闭 ADB」按钮（二次确认，destructive）。

**不做**：白名单查看 / 编辑、TTL 多选、推流地址展示。

### 续期语义

中台无独立续期端点，唯一手段是重发 `operate enable`。续期 = 再次 enable（ttl=86400）。
token 是否轮换由中台决定；续期后前端回查 `AdbInfo` 并刷新地址/Token/过期时间。

## 3. 后端改动

### 3.1 SDK 层 `backend/framework/midplat/adb.go`

- 删除遗留 token 全套死代码：`ADBTokenRequest` / `ADBTokenContainer` / `ADBTokenResponse` / `EnableADBToken` / `DisableADBToken` / `doADBToken`。
- 删除白名单全套：`AdbWhitelistEntry` / `GetAdbWhitelist`。
- `AdbOperateRequest` 去掉 `WhiteIP`（不再用 update_whitelist）；保留 `Operation` / `Containers` / `TTL`。
- `AdbOperateResult` 保持不变（`allSuccess` / `errorMessage` / `containerDetails` 用于错误提取）。
- `CloudPhoneAdbInfo` 保留 `CpID / Status / AdbAddress / AdbToken / AdbTokenExpiredAt`；不加 streamingServer。注释从「§2.16」改正为「§2.6 v2」。
- `GetCloudPhoneAdbInfo`：当中台对不存在 cpId 返回 `DATA_NOT_EXIST`（P1-A10）时，降级为 `&CloudPhoneAdbInfo{CpID: cpID}`（视为未开启），不冒泡成 500。

### 3.2 Client 层 `backend/framework/midplat/client.go` + `types.go`

现状：4xx 时（client.go:192）先 return `HTTPError`，未解析包络，`code="DATA_NOT_EXIST"` 被埋进 Body 字符串。

改造：

- `HTTPError` 增加 `Code string` 字段（向后兼容；`Error()` 顺带带上 code）。
- `doJSON` 在 4xx 时 best-effort `json.Unmarshal` 包络，把 `env.Code` / `env.Message` 填进 `HTTPError`（解析失败则留空，行为同现状）。
- 新增包级 helper `func IsDataNotExist(err error) bool`：识别 `*HTTPError`（Code）与 `*APIError`（Code）中的 `DATA_NOT_EXIST`。

这是 client.go 唯一横切改动；其它调用方继续拿 `HTTPError` 不受影响。

### 3.3 Service 层 `backend/modules/phone/internal/service.go`

- 保留 `AdbConnInfo`（`enabled / adbAddress / adbToken / adbTokenExpiredAt / status`）；注释 §2.16→§2.6 改正。
- 保留 `AdbInfo(userID,id)`、`AdbDisable(userID,id)`。
- `AdbEnable` 去掉 `whiteIP` 参数，签名变 `AdbEnable(userID, id, ttl int)`；内部 `s.ops.AdbOperate(ctx, cpID, "enable", ttl)`。
- 删除 `AdbWhitelist` / `AdbUpdateWhitelist`。
- `allSuccess=false`（P1-A33）继续由 `adbErr()` 提取 `errorMessage` / `containerDetails[].error` → `apperr.Internal`（保持）。
- `midplatPort` 接口与 `sdkAdapter`：`AdbOperate` 去掉 `whiteIP` 参数；删除 `AdbWhitelist` 方法。
- 续期不新增 service 方法，前端复用 enable。

### 3.4 路由层 `backend/modules/phone/internal/op_api.go`

- 保留：`GET /phone/{id}/adb`（info）、`POST /phone/{id}/adb/enable`（body `{ttl?}`，默认 86400）、`POST /phone/{id}/adb/disable`。
- 删除：`GET /phone/{id}/adb/whitelist`、`POST /phone/{id}/adb/whitelist`。
- `EnableAdbCloudPhone` 请求体去掉 `whiteIp`，只留 `ttl?`（缺省/0 → 86400）。

### 3.5 安全姿态（保持）

P0-A27 跨租户读越权：所有 ADB 操作经 `resolveCp` 做「本人拥有 + 已开通 + 中台已配置」三重前置校验，cpId 来自自家库而非用户输入。本次不回退该防御；白名单读接口已整体移除，进一步缩小攻击面。

## 4. 前端改动

- `my/src/views/phone/AdbDrawer.vue`：
  - `ADB_AVAILABLE=true`，删除「尚未调通」注释与禁用提示。
  - 移除白名单 textarea / 列表、TTL 多选 select。
  - 重构为「开启 / 已开启（地址 + Token + 倒计时 + 续期 + 两步引导 + 关闭）」。
  - 倒计时：基于 `adbTokenExpiredAt` 每秒计算剩余；≤0 显示红色「已过期，请续期」。drawer 关闭时清理定时器。
- `my/src/api/modules/phone.ts`：删除 `adbWhitelist` / `adbUpdateWhitelist`；`adbEnable(id, { ttl?: number })`。续期复用 `adbEnable`。
- `my/src/types/phone.ts`：删除 `AdbWhitelistEntry`；`AdbInfo` 保持 `enabled / adbAddress / adbToken / adbTokenExpiredAt / status`。
- i18n（`zh-CN.ts` + `en.ts`）：删白名单相关键；新增/调整：`renew`（续期）、`remaining`（剩余有效时间）、`expired`（已过期，请续期）、`step1`/`step2`（两步连接引导）、`connectStep`（先 connect 再 xlogin 的说明）。保留 address / token / show / hide / copied 等。

## 5. 测试

- `backend/modules/phone/internal/ops_test.go`：
  - `fakePort` 去掉 `AdbWhitelist`，`AdbOperate` 去掉 whiteIP 参数。
  - 补 `AdbEnable` 在 `allSuccess=false` 时返回 `errorMessage` → service 返回 Internal 错误且消息为该 errorMessage。
  - 补 `AdbInfo` 在底层 `DATA_NOT_EXIST` 降级为「未开启」（enabled=false）。
- `backend/framework/midplat`（如有单测）：`IsDataNotExist` 对 `HTTPError{Code:"DATA_NOT_EXIST"}` / `APIError{Code:"DATA_NOT_EXIST"}` 返回 true，其它返回 false。
- 后端 `go build ./... && go vet ./... && go test ./...` 通过。
- 前端 `pnpm build`（`vue-tsc -b && vite build`）通过。

## 6. 验收标准

1. 后端编译 / vet / 测试全过；前端 build 过。
2. 全仓无残留：`adb/token/`、`whitelist`（ADB 相关）、`update_whitelist`、`AdbWhitelistEntry`。
3. drawer：未开启→点开启→展示地址/Token/倒计时/两步引导；点续期→过期时间刷新；点关闭→回到未开启。
4. 不存在/未开通 cpId 查 ADB 不报 500，呈现为「未开启」。
5. 中台 `allSuccess=false` 时前端能看到中台返回的具体错误文案。
