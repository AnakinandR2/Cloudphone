# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

> 这是 `backend/`（Go + Gin + GORM 的模块化单体）。架构细节见 [ARCHITECTURE.md](ARCHITECTURE.md)，
> 加模块见 [tools/scaffold/README.md](tools/scaffold/README.md)。模块路径前缀为 `manager-backend`。

## 常用命令

```bash
go build ./...                       # 编译
go vet ./...
gofmt -w <files>                     # 提交前格式化（全部 LF）
go test ./...                        # 默认 sqlite，无需外部数据库
go test ./modules/staff/internal/ -run TestLogin -v   # 跑单个测试
go test ./framework/ -run TestModuleBoundaries       # 模块边界自检（Modulith verify）
go run .                             # 启动；默认 sqlite，建库即用

# 加业务模块（自动接线 main.go / apptest / permissions.go）
go run ./tools/scaffold -name <x>                 # 后台 RBAC CRUD
go run ./tools/scaffold -name <x> -kind user      # 前台按属主隔离的 per-owner CRUD

# Swagger（docs/ 是生成产物且已 gitignore）——改了接口注释后重生成：
go install github.com/swaggo/swag/cmd/swag@v1.16.2
swag init -g main.go --parseInternal --parseDependency -o docs
```

可用 `DB_TYPE=mysql`（或 postgres）+ `DB_*` 切换数据库；测试也读这些环境变量，默认 sqlite。

## 架构要点（多文件串起来才能看懂的部分）

**分层**：`framework/`（平台层，无业务，可被所有模块依赖）；`modules/<m>/`（业务模块，垂直切片）；
`apptest/`（应用级 HTTP 集成测试）；`tools/scaffold/`（模块生成器）。
依赖方向只能是 `modules → framework`；模块之间只能经对方的**公开门面**交互。

**Modulith 边界（编译期强制）**：模块实现全部放在 `modules/<m>/internal/`，靠 Go 的 internal 机制——
别的模块无法 import 任何模块的 internal，只能用公开包 `modules/<m>/<m>.go`（如 `staff.PermissionMiddleware`、
`user.AuthMiddleware`）。`framework/arch_test.go` 用 `go list` 断言这些边界，随 `go test ./framework/` 跑。

> **命名约定**：`modules/staff` = 后台管理员侧（scope=`staff`，RBAC）；`modules/user` = 前台用户侧
> （scope=`user`，按属主隔离）。两者是独立限界上下文，独立的表（`staff` / `users`）与 JWT 身份域。

**模块生命周期与注册**：模块在 `internal/module.go` 的 `init()` 里 `framework.GlobalModule.Register(...)`，
靠 `main.go`/`apptest` 对公开包的 **blank import** 触发。`Init(db)` 做依赖注入（构造 repository/service，
**不要用全局 `framework.DB`**）；`RegisterRoutes` 注册路由；`OnStart/OnStop` 管后台任务。
`main.go` 与各 `TestMain` 都遍历 `GlobalModule.GetAll()` 调 `Init`，装配方式一致。

**DDD 约定**：service 依赖 `repository` 接口（GORM 实现在 `repository.go`，事务/SQL 收敛于此）；
错误用 `framework/apperr`（`NotFound/Conflict/Validation/...`）+ handler 的 `framework.FailErr`（映射 404/409/422/...）；
列表排序一律 `framework/query.SafeOrder(order, sort, 白名单, 默认)`，**绝不拼接 order**（防 SQL 注入）。

**双身份域认证（关键）**：JWT 带 `scope` 声明（`staff` / `user`），签发时写死、签名内不可篡改。
`framework/auth.MiddlewareConfig{Secret, Scope, ...}.Middleware()` 验签后**强制比对 scope**，不符即 401。
- 后台（staff）路由用注入的中间件（`framework.AuthMiddlewareFunc`，即 `RegisterRoutes` 收到的 `middlewareFuncs`）+ `staff.PermissionMiddleware("x:y")`。路由前缀 `/api/v1/staff/...`（登录 `/staff/auth/login`）。
- 前台（user）路由用 `user.AuthMiddleware()`（前台模块**忽略** `middlewareFuncs`）。路由前缀 `/api/v1/user/...`（登录 `/user/auth/login`、注册 `/user/auth/register`）；后台管理前台用户在 `/api/v1/admin/users/...`（权限 `user:view` / `user:manage`）。
- 因此前台令牌打后台接口、后台令牌打前台接口都会 401。前台可配独立密钥 `USER_JWT_SECRET` 做纵深防御。
- **令牌版本失效**：用户表存 `token_version` 写入令牌 `tv`，中间件经 `Validate` 钩子比对；改密/登出/禁用即递增 → 旧令牌立即失效。

**建表与初始数据（无迁移系统）**：已移除版本化迁移。各模块 `init()` 里用 `framework.RegisterSetup(func(db) error{...})`
登记自己的建表（`AutoMigrate`）+ 初始 seed；启动时 `main.go` 调 `framework.RunSetup(framework.DB)` 按注册顺序执行。
**实现必须幂等**：`AutoMigrate` 已存在则补列，seed 先查后插（重复启动不重复插）。无 `schema_migrations` 表、无版本号。
sqlite 下索引名全局唯一，跨表别用重名索引。改 schema 直接改 model，下次启动 `AutoMigrate` 补列；开发期如需重置删库文件即可。

**访问日志**：全局中间件按 `scope` 记录；前台请求默认**不入库**（`ACCESS_LOG_USER_ENABLED=false`，前台量大建议走数仓）。
异步批量写入 + 定时清理；`OnStop` 会 drain 写入器，配合 `main` 的优雅关闭不丢日志。

## 测试约定（容易踩坑）

- 单元测试放模块 `internal/`，经 `framework.SetupTestDB(m)` 跑**真实 sqlite**；`TestMain` 调 `Module.Init` 装配。
- 集成测试放 `apptest/`，只能经**公开 HTTP API**（登录拿令牌、REST 准备数据），不能 import 任何模块 internal——
  这正是边界在起作用。需要后台令牌用 `adminToken`（内置 admin/admin123），前台用 `registerUser`。
- **不要 `CleanTable` 截断共享表**（`RunSetup` seed 了 `示例只读` 角色、admin 用户，整库测试共用一个 DB）；
  用唯一命名 + 按 service 清理自己造的数据。

## 加/改模块的正确姿势

优先用 `go run ./tools/scaffold`（自动接线 + 生成单测/隔离测试；`-kind staff` 后台 RBAC，`-kind user` 前台按属主隔离）。
手动加模块时，三处接线点由锚点注释定位：`main.go`/`apptest/main_test.go` 的 `// scaffold:module-imports`、
`permissions.go` 的 `// scaffold:permission-groups`——移动文件时保留锚点。后台路由权限标识需在
`modules/staff/internal/permissions.go` 登记后才能授予非超管（超管默认放行）。
