# 架构说明（DDD + 模块化单体 / Modulith）

本服务是 Go + Gin + GORM 的**模块化单体**：单进程部署，但内部按业务模块垂直切分，
模块边界由 Go 编译器（`internal/`）强制、由架构测试守护。本文沉淀整体结构与编码约定。

## 分层与目录

```
backend/
  main.go                 组装根：加载配置 → 连库 → 迁移 → 各模块 Init → 路由 → OnStart
  framework/              平台层（共享内核）：不含任何业务，可被所有模块依赖
    module.go             Module 接口 + 注册表
    router.go             路由装配
    db.go config.go migrator.go response.go notification.go testutil.go
    apperr/               领域错误类型 + HTTP 状态映射
    query/                SafeOrder 排序白名单
    auth/                 通用 JWT（带 Scope 身份域）+ 鉴权中间件
    arch_test.go          模块边界自检（go list 依赖分析）
  modules/                业务模块（垂直切片），每个模块自包含
    <module>/
      <module>.go         公开门面：blank-import internal 触发注册；按需再导出契约
      internal/           实现，受 Go internal 保护（仅本模块可导入）
        model.go repository.go service.go api.go module.go
        *_test.go         模块内单元测试
  apptest/                应用级集成测试（HTTP，仅经公开 API）
  tools/scaffold/         新模块脚手架生成器
```

依赖方向：`modules/* → framework`，模块之间只能经对方**公开包**交互，**禁止**反向或跨内部依赖。

## Modulith：模块边界如何强制

- **编译期**：实现放在 `modules/<m>/internal/`，Go 规定 `.../<m>/internal` 只能被 `.../<m>/**` 导入，
  其他模块无法 import 它的内部实现 —— 这是"只能通过发布的 API 交互"的硬保证。
- **公开契约**：`modules/<m>/<m>.go` 是模块对外的唯一入口。多数模块只在此 blank-import internal 触发注册；
  需要被别的模块调用的能力在此再导出（例：`modules/user` 导出 `PermissionMiddleware`）。
- **架构测试**：`framework/arch_test.go` 用 `go list` 解析依赖图，断言：
  1) 平台层不依赖业务模块；2) 不跨模块访问 internal；3) 业务模块不依赖 main。随 `go test ./framework/` 运行。
- **CI linter**：`.golangci.yml` 用 depguard 表达同样的分层规则（可选）。

## 模块生命周期

模块实现 `framework.Module` 接口并在 `init()` 中 `framework.GlobalModule.Register(...)`：

| 方法 | 时机 | 用途 |
|---|---|---|
| `Name()` | — | 模块名 |
| `Init(db)` | 启动/测试装配 | **注入 DB、构造 repository 与 service**（依赖注入，不用全局 DB） |
| `RegisterRoutes(g, mw...)` | 路由装配 | 注册本模块路由 |
| `OnStart()` / `OnStop()` | 启停 | 后台任务（如访问日志异步写入） |

`main.go` 与各测试的 `TestMain` 都通过遍历 `GlobalModule.GetAll()` 调用 `Init`，保证装配方式一致。

## 数据库与迁移

- 默认 **sqlite**（开箱即用，`DB_NAME` 即文件路径）；支持 mysql / postgres，改 `DB_TYPE` 等环境变量即可。
- 迁移在各模块 `init()` 里用显式递增 `Version` 注册到 `framework.GlobalMigrations`，启动时按版本号顺序执行；
  顺序与模块 init 顺序无关。新增模块的版本号由脚手架自动取「现有最大值 +1」。

## DDD 战术约定

- **Repository**：service 依赖 `repository` 接口（GORM 实现在 `repository.go`），不直接接触全局 `framework.DB`；
  事务/SQL 收敛在仓储层，便于隔离单测。
- **领域错误**：service 返回 `framework/apperr` 的类型化错误（`NotFound/Conflict/Validation/Unauthorized/Forbidden/BadRequest`），
  handler 统一用 `framework.FailErr(c, err)` 映射 HTTP 状态码（404/409/422/401/403/400），不再硬编码。
- **防 SQL 注入**：列表排序一律用 `framework/query.SafeOrder(order, sort, 白名单, 默认)`，排序字段只取白名单内值。
- **统一响应**：`framework.OK/OKWithData/OKWithPage/Fail/FailErr`，响应体 `{code, message, data}`。

## 认证与身份域（前后台隔离）

后台员工与前台用户是**两个独立限界上下文**，各自独立的表、登录流程与令牌：

- `modules/user` —— 后台员工（账号密码 + SSO + RBAC 角色/权限），令牌 `scope=staff`。
- `modules/customer` —— 前台用户（手机号注册/登录，无 RBAC），令牌 `scope=customer`。
- `framework/auth` 提供通用 JWT（`Claims` 带 `Scope`）与可配置鉴权中间件（密钥 / 期望 Scope / 可选 Cookie）。
  中间件强制校验 Scope，**后台令牌打前台接口、前台令牌打后台接口都会 401**（见 `apptest`）。
- 密钥：后台用 `JWT_SECRET`；前台可用独立的 `CUSTOMER_JWT_SECRET`（留空则回退到 `JWT_SECRET`，仅靠 Scope 隔离）。
  生产建议为前台配置不同密钥做纵深防御。
- Cookie：后台用 `JWT_COOKIE_NAME`（默认 `jwt_token`）、前台用 `CUSTOMER_JWT_COOKIE_NAME`（默认 `customer_token`，独立避免覆盖）。
  登录/注册会 Set-Cookie（HttpOnly），鉴权时令牌优先取 Authorization 头、其次取对应 Cookie；置空则该侧仅走头。
- 访问日志：中间件全局记录请求，按身份域 `scope`（staff/customer/anonymous）落库，仅 staff 侧凭 `access_log:view` 可查。
  **前台（customer）请求默认不入库**（`ACCESS_LOG_CUSTOMER_ENABLED=false`）——前台流量大，建议单独走数仓/OLAP；需要时可开启开关。
- 令牌失效：JWT 无状态，故用**令牌版本号**做服务端失效——用户表存 `token_version`、写入令牌（claim `tv`），
  鉴权时经 `MiddlewareConfig.Validate` 钩子与库中当前值比对，不一致即 401。前后台均已接入：
  - 前台（customer）：登出（`POST /customer/auth/logout`）递增版本 → 登出全部设备。
  - 后台（staff）：改密（`POST /auth/change-password`）与管理员重置密码（更新用户时带 password）递增版本 → 旧令牌立即失效、强制重新登录。
  默认版本为 0；存量令牌（tv=0）与默认值匹配，部署不会误登出。
- RBAC：后台权限在 `modules/user/internal/permissions.go` 的 `PermissionGroups` 登记；
  路由用 `user.PermissionMiddleware("<perm>")` 守护，超管默认放行。

## 安全与运维基线

- **CORS 白名单**：`framework.CORSMiddleware` 按 `CORS_ALLOWED_ORIGINS` 校验来源，命中则回显具体 Origin + `Allow-Credentials`（绝不用 `*`+凭证），未命中不下发 CORS 头。留空=放行任意来源（仅开发，启动会告警）。
- **安全响应头**：`framework.SecurityHeaders`（`X-Content-Type-Options`/`X-Frame-Options`/`Referrer-Policy`）。
- **优雅关闭**：`main` 用 `signal.NotifyContext` 监听 SIGINT/SIGTERM → `http.Server.Shutdown` 排空在途请求 → 逆序 `Module.OnStop()`。access_log 的 `OnStop` 会关闭通道并**等待异步写入器 flush**（最多 5s），保证关闭前不丢日志。
- **密钥告警**：启动时若 `JWT_SECRET` 仍为默认值会打印警告，提醒生产改强随机值。

## 测试策略

- **单元测试**：放各模块 `internal/`，经 repository 跑真实 sqlite（`TestMain` 用 `framework.SetupTestDB`）。
- **集成测试**：放 `apptest/`，只经公开 HTTP API 准备数据与断言（含前后台令牌互不通用的验证）。
- **架构测试**：`framework/arch_test.go` 守护模块边界。
- 默认 sqlite，直接 `go test ./...` 即可，无需外部数据库。

## 新增一个模块

用脚手架（详见 `tools/scaffold/README.md`）：

```bash
go run ./tools/scaffold -name product
go build ./... && go test ./modules/product/... ./framework/
```

会生成规范的 CRUD 模块（model/repository/service/api/module + 单元测试），并**自动接线**
`main.go`、`apptest/main_test.go` 的 blank import 与 `permissions.go` 的权限组（靠锚点注释定位，幂等）。

## 约定速查清单

- [ ] 实现放 `internal/`，跨模块只经公开门面
- [ ] service 经 `repository` 接口访问数据，不碰全局 DB
- [ ] 错误用 `apperr` + `framework.FailErr`
- [ ] 列表排序用 `query.SafeOrder` + 白名单
- [ ] 迁移用递增 `Version` 注册
- [ ] 需鉴权的接口选对身份域（staff / customer）
- [ ] 补单元测试（internal）/ 必要时补集成测试（apptest）
- [ ] `go test ./framework/` 边界测试通过
