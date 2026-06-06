# Gloryphone 云手机平台 — Monorepo

统一域名下的云手机 SaaS 平台，含营销站、用户后台、运营后台与后端服务。

## 目录结构

| 目录 | 技术栈 | 说明 |
| --- | --- | --- |
| `backend/` | Go + Gin + GORM | 单一后端，接口走 `/api/v1/...`，HttpOnly Cookie 鉴权 |
| `www/` | Nuxt 3 (SSR) | 营销站，根路径 `/`，SSR 读 Cookie 拉登录态 |
| `my/` | Vue 3 + Vite | 用户后台 SPA，挂在 `/my/` 子路径 |
| `admin/` | Vue 3 + Vite | 运营后台 SPA（RBAC） |
| `ops/` | shell + nginx | 构建脚本、nginx 网关配置、启动脚本 |
| `docs/` | Markdown | 中台接口规范、整合笔记等文档 |

> `ref/`、`admin-fantistic/`、`cp-glory-service/` 为本地参考资料，已在 `.gitignore` 中排除，不纳入版本库。

## 本地开发

各前端独立安装与启动（端口见各自 `vite.config.ts` / `nuxt.config.ts`）：

```bash
# 后端
cd backend && cp .env.example .env && go run .

# 用户后台（访问 http://localhost:5666/my/）
cd my && pnpm install && pnpm dev

# 运营后台
cd admin && pnpm install && pnpm dev

# 营销站
cd www && pnpm install && pnpm dev
```

统一域名网关与子路径部署见 `docs/integration-notes.md` 与 `ops/nginx/`。
