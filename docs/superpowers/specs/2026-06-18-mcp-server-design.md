# MCP 服务设计（开放 API 的 MCP 通道）

日期：2026-06-18
状态：已确认，待落地

## 背景与目标

开放 API（`/api/open/v1`）已交付 18 个端点并以 `gp_live_` 前缀的 API Key 鉴权。本期把这套能力 1:1 包装成 MCP 工具，让 Claude Desktop / Cursor 等 MCP 客户端可直接编排云手机平台（增删查、电源、应用、脚本、代理）。

不做：MCP 端的 OAuth、独立令牌、独立 DTO 体系（YAGNI，全部复用开放 API 既有形态）。

## 关键决策

1. **形态**：Go 进程内 + Streamable HTTP。在现有后端挂一个 MCP 端点，直接调用各模块 facade，不走 HTTP 回环。
2. **工具范围**：18 个开放端点全部 1:1 映射为 MCP 工具，AI 可完成全闭环操作。
3. **鉴权**：复用现有 API Key。同一把 `gp_live_` 密钥既能调 REST 也能接 MCP。

## 架构

- 新模块 `backend/modules/mcp`（与 `openapi` 平级），实现位于 `internal/`，公开包 `mcp.go` 仅供 blank-import 自注册。
- 库：`github.com/mark3labs/mcp-go`（成熟、原生支持 Streamable HTTP 与 `WithHTTPContextFunc`）。
- 端点：`POST /api/mcp`，经 `framework.RegisterRootRoutes` 挂在引擎根（与 `/api/open/v1` 同机制，不在 `/api/v1` 下）。
- 依赖方向：`mcp → openapi`（鉴权）、`mcp → phone / app / automation / proxy`（业务 facade），均为单向，遵守 `framework/arch_test.go` 边界。

## 鉴权链路

1. mcp-go 的 `WithHTTPContextFunc` 从 HTTP 请求头取密钥：优先 `Authorization: Bearer <key>`，回退 `X-API-Key`。
2. 调 `openapi.Authenticate(key) (userID int, err error)` 解析归属用户，把 `userID` 注入 tool 的 `context.Context`。
3. 每个 tool handler 从 ctx 取 `userID`，再调对应 facade；归属隔离与状态门禁仍在 facade 内部生效。
4. 密钥缺失/无效/已撤销：工具调用返回 MCP error（`isError`），不泄露内部细节。

为此在 `openapi` 公开包补一个轻门面 `Authenticate(key string) (int, error)`，包装 internal 的 `Service.Authenticate`（当前仅 middleware 内部使用）。

## 工具清单（18 个，1:1）

入参/出参直接对照 `modules/openapi/internal/open_api.go` 的 DTO（含 example），返回值为 facade 结果结构体的 JSON 文本。

云手机：
- `list_phones(page?, size?)`
- `get_phone(id)`
- `create_phone(name, proxyId?)` — region/imageId 由后端自动选，不暴露
- `destroy_phone(id)`
- `power_phone(id, operation)` — operation: on/off
- `restart_phone(id)`

应用：
- `list_apps(source?)` — source: all/mine/store
- `list_installed_apps(id)`
- `install_app(id, appIds)`
- `uninstall_app(id, appIds?, packages?)`

脚本/任务：
- `list_scripts`
- `run_script(id, scriptId, ...)`
- `get_task(taskId)`

代理：
- `list_proxies`
- `create_proxy(name, protocol, host, port, username?, password?, region?, remark?)`
- `update_proxy(id, ...)`
- `delete_proxy(id)`
- `bind_proxy(id, proxyId)`

每个工具有中文 description；参数 schema 用 mcp-go 的 `mcp.WithString/WithNumber/...` 声明，必填项标 `Required()`。

## 前端

将 `my/src/views/automation/ApiMcpView.vue` 中「MCP（即将推出）」卡片改为真实接入卡：
- 展示 MCP 端点 URL（同 baseUrl 推导，`/api/mcp`）。
- 提示复用上方 API Key。
- 给一段 Claude Desktop / Cursor 的 `mcpServers` 配置 JSON（带 Copy 按钮）。
- 移除 `soon` Badge。
- 补 i18n（`apimcp.mcp*` 系列）。

## 测试

- Go：用 mcp-go in-process client（或直接调 tool handler 函数）+ 现有 sqlite 测试库与 fake 中台。
  - `list_phones` 返回本人手机。
  - 代理闭环：`create_proxy` → `bind_proxy` 后 `cloud_phones.proxy_id` 落库。
  - 脚本闭环：`run_script` → `get_task` 拿到任务。
  - 无效密钥 → 工具返回 error。
  - 复用 openapi 测试的建表（`framework.RunSetup` + 各模块 init）与 fake 中台。

## 取舍记录

- 复用密钥而非 OAuth/独立令牌：降低本期工量，用户零新概念。
- 复用开放 API 的入参/出参形态：两条通道语义一致，维护一处。
- 18 个全给：保证 AI 可完成创建手机→绑代理→装应用→跑脚本→查任务的完整链路。
