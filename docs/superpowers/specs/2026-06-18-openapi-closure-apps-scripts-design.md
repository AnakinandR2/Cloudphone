# 开放 API 闭环：补 /apps 与 /scripts 读端点 — 设计

- 日期：2026-06-18
- 状态：已评审，待实现
- 背景：开放 API 有「装应用 `{appIds}`」「跑脚本 `{scriptId}`」写接口，但没有列应用/列脚本的读接口，调用方拿不到 appId/scriptId → 不闭环。

## 1. 目标

补两个开放读端点，让 install / run-script 在纯 API 场景下可闭环获取所需 ID。

## 2. 闭环审计（背景）

| 写接口 | 需要 ID | 现状 |
|---|---|---|
| 装应用 `{appIds}` | 中台 appId | ❌ 缺列应用 → 本设计补 |
| 跑脚本 `{scriptId}` | 脚本 id | ❌ 缺列脚本 → 本设计补 |
| 卸应用 | 已装应用 | ✅ `GET /phones/:id/apps` |
| 创建 image/proxy/region | — | ⚠️ 可缺省；自定义查询不在本期 |
| 电源/重启/销毁/详情 | phone id | ✅ `GET /phones` |

## 3. 新增端点（`/api/open/v1/*`，key 鉴权）

### 3.1 GET /apps — 可安装应用

- 集合：我的应用库（`store=false`，本人）∪ 应用商店（`store=true`）。
- 过滤：`?source=mine|store|all`（默认 `all`）。
- 返回数组，元素（专用 DTO）：

| 字段 | 说明 |
|---|---|
| `appId` | int64，**install 接口 `{appIds}` 用的值**（= 中台 `CpAppID`） |
| `name` | 应用名 |
| `packageName` | 包名 |
| `version` | 版本 |
| `store` | bool，是否商店应用 |
| `status` | 状态（CREATING/NORMAL） |

### 3.2 GET /scripts — 可运行脚本

- 集合：我的 ∪ 商店，**仅启用**（`status=enabled`）。
- 返回数组，元素（专用 DTO）：

| 字段 | 说明 |
|---|---|
| `scriptId` | uint，**run-script 接口 `{scriptId}` 用的值**（本地脚本 id） |
| `name` | 脚本名 |
| `description` | 描述 |
| `version` | 版本 |
| `store` | bool，是否商店脚本 |

## 4. 后端实现

### 4.1 facade 补充（1 行委托，归属/隔离已在 service 内）

- `modules/app/app.go`（当前为空门面）：
  - `type CustomerApp = internal.CustomerApp`
  - `func List(userID int) ([]CustomerApp, error)` → `AppService.List`
  - `func StoreList() ([]CustomerApp, error)` → `AppService.StoreList`
- `modules/automation/automation.go`：
  - `type AutomationScript = internal.AutomationScript`
  - `func ListUsableScripts(userID int) ([]AutomationScript, error)` → `Service.ListUsableScripts`

### 4.2 openapi handler

- `OpenListApps`：按 `?source` 取 app.List / app.StoreList（或并集），映射成开放 DTO（`appId = CpAppID`）。
- `OpenListScripts`：取 automation.ListUsableScripts，映射成开放 DTO（`scriptId = 本地 id`）。
- 注册到 `registerOpenRoutes`：`GET /apps`、`GET /scripts`。

### 4.3 依赖

`openapi → app` 新增（单向）；`framework/arch_test.go` 自动覆盖边界（app 不反向依赖 openapi）。

## 5. Swagger

- 两个 GET 加 swag 注解（`@Tags 应用` / `@Tags 自动化`，专用响应 DTO 带 example）。
- 重新生成 `docs/openapi`：文档里 install/run-script 旁即可查到 appId/scriptId 来源。

## 6. 前端

仅在 ApiMcpView 的「如何调用」curl 注释里点一句「先 `GET /apps`、`GET /scripts` 拿 id」，不动 UI 结构。

## 7. 测试

- openapi 单测：`/apps` 含我的 + 商店且 `appId` 等于 `CpAppID`、`?source` 过滤正确；`/scripts` 只含启用脚本且 `scriptId` 可直接喂 run-script。
- `pnpm build` 过。

## 8. 不在本期

- 创建云手机的 image/proxy/region 查询端点。
- 应用/脚本读端点的分页（列表通常小，直接全量返回）。
