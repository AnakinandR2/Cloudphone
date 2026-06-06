# 后端 - Go

基于 Go + Gin + GORM 的**模块化单体（Modulith）**服务框架，按 DDD 分层、模块边界编译期强制。

## 技术栈

- **Web 框架**: Gin
- **ORM**: GORM（默认 SQLite，开箱即用；支持 MySQL / PostgreSQL）
- **认证**: JWT (HS256)，按身份域隔离后台（staff）/ 前台（customer）；后台支持 SSO + RBAC
- **Go 版本**: 1.24+

## 快速开始

```bash
cp .env.example .env      # 默认 sqlite，无需额外配置
go run .                  # 启动；Swagger: http://localhost:9981/swagger/index.html
go test ./...             # 默认 sqlite，无需外部数据库
```

Swagger 文档（`docs/` 为生成产物，已 gitignore）按需重新生成（handler 在 internal 包，需带 `--parseInternal`）：

```bash
go install github.com/swaggo/swag/cmd/swag@v1.16.2
swag init -g main.go --parseInternal --parseDependency -o docs
```

## 文档

- 架构与编码约定：[ARCHITECTURE.md](ARCHITECTURE.md)
- 新增模块脚手架：[tools/scaffold/README.md](tools/scaffold/README.md)

## 目录速览

```
framework/        平台层（共享内核，无业务）：module/router/db/config/migrator/apperr/query/auth
modules/          业务模块（垂直切片，internal 隔离）：user(后台) / customer(前台) / example / accesslog
apptest/          应用级集成测试（仅经公开 API）
tools/scaffold/   新模块生成器
```
