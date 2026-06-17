# 开放 API 与 API 密钥（API & MCP 第一期）— 设计

- 日期：2026-06-17
- 状态：已评审，待实现
- 背景：my 的「API 与 MCP」目前为前端原型（`ApiMcpView.vue`，静态密钥表 + MCP 配置示例）。本设计落地**第一期：API 密钥 + 开放 REST API**。MCP 服务端为下一期单独 spec。

## 1. 目标与范围

让客户用 API 密钥以编程方式调用一组**对外解耦、文档化**的云手机能力（开放 API），并在 my 里自助管理密钥（签发 / 查看 / 撤销）。

### 范围（本期）

- API 密钥：签发、列表（掩码）、撤销；明文密钥仅创建时返回一次。
- 开放 REST API：`/api/v1/open/v1/*`，用 API 密钥（Bearer）鉴权，覆盖下列能力。
- my 前端：重做 `ApiMcpView`，API 密钥卡片真实可用；MCP 卡片保留「即将推出」占位。

### 不在本期

- MCP 服务端（下期单独 spec，复用本期密钥鉴权）。
- 按 key 的细粒度 scope / 权限。
- 限流 / 配额。
- 截屏 / sendKeys / 文件 / ADB 等开放端点。
- 自动生成 OpenAPI / Swagger 文档（本期手写 curl 示例）。

## 2. 关键决策（评审锁定）

1. **分期**：先做 API 密钥 + 开放 API；MCP 下期（鉴权复用本期密钥）。
2. **API 形态**：专门的「开放 API」子集 `/api/v1/open/v1/*`，与内部接口**路径/版本解耦**；DTO v1 暂复用现有 service 的 JSON 形状。
3. **v1 能力集**：云手机列表/详情、创建/销毁、开关机/重启、应用装/卸/查已装、自动化跑脚本+查结果。
4. **鉴权**：`Authorization: Bearer <key>` → 中间件解析到属主用户；独立于改密的 `token_version`，有自己的撤销生命周期。
5. **无 per-key scope**（v1）：每把 key 授予该用户的全部开放 API 能力。
6. **架构**：单个新模块 `openapi`，同时拥有「密钥表 + 管理端点 + key 鉴权中间件 + 开放 API 处理器」；开放处理器经 facade 调 `phone/automation/app`。

## 3. 架构

### 3.1 新模块 `openapi`（modulith）

```
backend/modules/openapi/
  openapi.go                 # 公开门面（叶子消费者，对外不导出契约）
  internal/
    model.go                 # APIKey
    repository.go            # GORM
    key_service.go           # 生成/校验/撤销/列表 + last_used 节流
    middleware.go            # Bearer key 鉴权中间件（解析属主 → c.Set("userID")）
    key_api.go              # 密钥管理处理器（JWT 保护，供 my UI）
    open_api.go             # 开放 API 处理器（key 保护，委托各模块 facade）
    module.go               # 注册 / 建表 / 路由
```

依赖方向：`openapi → phone, automation, app`（单向，经各自门面）。

### 3.2 数据表 `api_keys`

| 字段 | 说明 |
|---|---|
| id | 主键 |
| user_id | 属主（index, not null） |
| name | 用户取的名称 |
| key_prefix | 展示用前缀，如 `gp_live_3f2a`（index，便于掩码展示与排查） |
| key_last4 | 明文末 4 位（掩码展示用） |
| key_hash | `sha256(完整密钥)` 十六进制（唯一索引；鉴权按此查） |
| last_used_at | 最近使用时间（nullable，节流更新） |
| revoked_at | 撤销时间（nullable；非空即失效） |
| created_at | 创建时间 |

### 3.3 密钥生命周期

- **生成**：`gp_live_` + 32 字节随机的 base62 编码。计算 `sha256` 存 `key_hash`，存 `key_prefix`(=`gp_live_`+前4位) 与 `key_last4`。**完整明文仅在创建响应里返回一次**，之后不可再取。
- **鉴权**：中间件读 `Authorization: Bearer gp_live_…` → `sha256` → 按 `key_hash` 查；未命中 / `revoked_at` 非空 → 401。命中则 `c.Set("userID", key.UserID)`（与现有 handler 一致），并节流更新 `last_used_at`（距上次 <60s 则跳过写库）。
- **撤销**：置 `revoked_at`（软删）→ 立即失效；不可恢复。

## 4. 开放 API 端点（`/api/v1/open/v1/*`）

路径参数用**本地 phone id**（与列表返回 `id` 一致）；脚本用 automation 脚本 id；任务用中台任务主键。

| 能力 | 方法 路径 | 复用 |
|---|---|---|
| 列表 | `GET /phones?page&size` | `phone.List`（含中台实时状态）|
| 详情 | `GET /phones/:id` | `phone.GetDisplay`（实时状态）|
| 创建 | `POST /phones` `{name,region?,imageId?,proxyId?}` | `phone.Create` |
| 销毁 | `DELETE /phones/:id` | `phone.Destroy`（状态门禁）|
| 电源 | `POST /phones/:id/power` `{operation:"on"|"off"}` | `phone.Power`（handler 转「开机/关机」）|
| 重启 | `POST /phones/:id/restart` | `phone.Restart` |
| 已装应用 | `GET /phones/:id/apps` | `phone.InstalledApps` |
| 装应用 | `POST /phones/:id/apps/install` `{appIds:[]}` | `phone.InstallApp` |
| 卸应用 | `POST /phones/:id/apps/uninstall` `{appIds?,packageNames?}` | `phone.UninstallApp` |
| 跑脚本 | `POST /phones/:id/run-script` `{scriptId}` | `id→cpId` 后 `automation.RunNow` → `{taskId,taskNo}` |
| 查任务 | `GET /tasks/:taskId` | `automation.TaskDetail`（状态/日志/截图/结果）|

### 4.1 需新增的 facade 方法（均为 1 行委托，归属校验已在 service 内）

- `phone`：`List / GetDisplay / Create / Destroy / Power / Restart / InstalledApps / InstallApp / UninstallApp` + `CpIDOf(userID,id)`（run-script 用）。门面用**类型别名**导出返回类型（如 `type CloudPhone = internal.CloudPhone`），使 openapi 可引用。
- `automation`：`RunScript(userID, scriptID, cpIDs)`、`TaskDetail(userID, midTaskID)`。

### 4.2 响应 / 错误约定

- 沿用全站统一信封 `{code,message,data}`（`code:0` 成功）；列表返回 `{list,total}`。后端零改造、与 my 一致。
- 鉴权失败 → 401 + 信封；非本人资源 → 404（不泄露存在性）；状态门禁 / 参数错 → 422 / 400，复用各 service 现有 `apperr`。
- **稳定性折中**：v1 路径与版本解耦在 `/open/v1`，DTO 暂复用现有 JSON 形状；将来收紧字段在 `/open/v2` 做专用 DTO，不影响内部接口。

## 5. 前端（my）

重做 `views/automation/ApiMcpView.vue`：

- **API 密钥卡片**（落地）：
  - DataTable 列表：名称 / 掩码密钥（`gp_live_3f2a••••a17c`）/ 创建时间 / 最近使用 / 状态（启用·已撤销）/ 操作。
  - 「新建密钥」对话框：填名称 → 创建成功弹出**完整密钥一次性展示**（复制 + 提示「仅此一次，关闭后无法再查看」）。
  - 「撤销」：就近 `Popconfirm`（destructive），撤销后行置灰。
  - 底部「如何调用」：开放 API 基址 + `curl` 示例（`Authorization: Bearer <key>` 调 `GET /phones`）。
- **MCP 服务卡片**：「即将推出」占位（静态，不接后端）。菜单标题去「待开发」改「API 与 MCP」。
- `api/modules/apikey.ts`（list/create/revoke）+ `types/apikey.ts` + `mock/apikey.ts` + i18n（补齐 `apimcp.*`，中英）。

## 6. 错误处理

- 创建：名称必填。生成失败（随机源错误）→ 500 友好提示。
- 鉴权中间件：缺/格式错的 Authorization → 401；命中已撤销 → 401。
- 开放处理器：所有目标资源经各 service 现有归属校验，key 仅解析「属主用户」，不放大权限。
- `last_used_at` 写库失败不影响请求（best-effort）。

## 7. 测试

- `openapi/internal` 单测：创建返回完整 key 且库内只存 sha256 + 前缀；鉴权中间件解析属主 / 拒绝已撤销/不存在 key；`last_used_at` 节流；开放处理器委托正确且非本人 → 404。
- `framework/arch_test.go` 自动覆盖 `openapi → phone/automation/app` 单向边界。
- 前端 `pnpm build`（vue-tsc）必过；mock 跑通密钥增删改与一次性展示。

## 8. 接线

- 模块 `init()` 注册 + `RegisterSetup` 建 `api_keys` 表；`main.go` / `apptest/main_test.go` 加 blank import（scaffold:module-imports 锚点）。
- 两组路由：`/api/v1/user/api-keys/*`（`user.AuthMiddleware()`）+ `/api/v1/open/v1/*`（key 鉴权中间件）。
- 无新增 RBAC 权限。

## 9. 安全要点

- 密钥只存 `sha256`，明文仅创建时返回一次。
- 撤销即时生效，独立于改密的 `token_version`。
- 开放接口不放大权限：key 只解析属主用户，能力等同该用户在 my 里能做的对应操作。
- 掩码展示只露前缀 + 末 4 位，足以辨识、不可还原。
