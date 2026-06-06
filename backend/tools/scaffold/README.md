# 模块脚手架（scaffold）

一条命令生成符合本项目 DDD / Modulith 规范的新业务模块（含单元测试），避免手写样板、保证结构一致。

## 用法

```bash
# 后台模块（默认）：受保护接口走后台鉴权 + RBAC 权限
go run ./tools/scaffold -name product

# 前台模块：受保护接口要求 user 登录，无 RBAC
go run ./tools/scaffold -name wishlist -kind user

# 自定义实体名 / 表名
go run ./tools/scaffold -name order -title Order -table orders
```

参数：

| 参数 | 说明 | 默认 |
|---|---|---|
| `-name` | 模块名（小写单词），同时作为包名 / 路由前缀 / 权限前缀 | 必填 |
| `-kind` | 模块风格：`staff`（后台 RBAC）/ `user`（前台登录） | `staff` |
| `-title` | 实体名（PascalCase），也是服务变量前缀 | 由 `-name` 推导 |
| `-table` | 数据表名 | `name + "s"` |
| `-root` | 项目根目录（含 go.mod） | `.` |
| `-skip-wire` | 仅生成文件，不自动接线 | `false` |

两种风格的差异：

- `staff`：后台 RBAC CRUD。受保护接口用后台中间件 + `user.PermissionMiddleware("<name>:xxx")`，自动登记权限组；数据全租户共享。
- `user`：前台**按属主隔离**的 per-user CRUD。所有接口都要求 `user` 登录；实体带 `user_id`，仓储所有读写以当前用户约束（`WHERE user_id = ?`），**用户只能增删改查自己的数据、访问他人记录一律 404**，从数据层杜绝 IDOR；不涉及 RBAC、不登记权限。生成的 `service_test` 含「用户间隔离」用例。

迁移版本号会自动扫描现有 `Version:` 取最大值 +1。

## 生成结构

```
modules/<name>/
  <name>.go                公开门面（blank-import internal 触发注册）
  internal/
    model.go               实体 + Create/Update DTO
    repository.go          repository 接口 + GORM 实现（隔离 SQL）
    service.go             业务服务（注入 repository、领域错误 apperr、SafeOrder 白名单排序）
    api.go                 gin handler（统一响应 + FailErr）
    module.go              Module 注册 + 迁移 + 路由（公开 list/get，受保护 create/update/delete）
    main_test.go           TestMain（默认 sqlite，复用 Module.Init 注入测试库）
    service_test.go        CRUD / 过滤 / NotFound 单元测试
```

生成的是「后台（staff）受保护的 CRUD」骨架，与 `modules/example` 同构。

## 自动接线

默认会自动改写三处并 `gofmt`（幂等，重复运行不会重复插入）：

1. `main.go`：插入 `_ "manager-backend/modules/<name>"`
2. `apptest/main_test.go`：插入同样的 blank import（集成测试注册该模块）
3. `modules/user/internal/permissions.go`：登记 `<name>:view/create/edit/delete`

接线点由这三个文件中的锚点注释定位：`// scaffold:module-imports` 与 `// scaffold:permission-groups`，
迁移或重排文件时保留这些锚点即可。加 `-skip-wire` 可关闭自动接线、仅生成模块文件。

生成后直接验证：

```bash
go build ./... && go test ./modules/<name>/... ./framework/
# framework 下的 arch_test 会自动校验新模块的边界
```

## 约定回顾（手写模块时同样适用）

- 实现一律放 `internal/`，靠 Go internal 机制编译期隔离；跨模块只经公开门面交互。
- service 不碰全局 DB，经 `repository` 接口注入；持久化/事务收敛在 repository。
- 错误用 `framework/apperr`（NotFound/Conflict/Validation/...），handler 用 `framework.FailErr` 统一映射状态码。
- 列表排序用 `framework/query.SafeOrder` + 字段白名单，杜绝 SQL 注入。
- 鉴权用 `framework/auth`：后台 `scope=staff`，前台 `scope=user`，令牌互不通用。
