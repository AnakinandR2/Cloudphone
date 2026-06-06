# admin

管理后台前端项目。

## 技术栈

- **Vue 3** (`<script setup>` + TypeScript)
- **Vite 7** 构建
- **Pinia** 状态管理
- **Vue Router 4** 路由（含登录守卫）
- **Tailwind CSS v4** 样式
- **shadcn-vue**（基于 reka-ui）UI 组件，组件位于 `src/components/ui`
- **TanStack Table v8**（`@tanstack/vue-table`）数据表格
- **axios** 请求封装，对接后端 `/api/v1`（Go/Gin，端口 9981）
- **pnpm** 包管理

## 开发

```bash
pnpm install
pnpm dev          # http://localhost:5666
```

开发服务器已配置代理：浏览器请求 `/api/*` 会转发到 `http://localhost:9981`（见 `vite.config.ts` 与 `.env.development`）。

## 构建

```bash
pnpm build        # 类型检查 + 打包到 dist
pnpm preview      # 本地预览产物
```

## 目录结构

```
src/
  api/            axios 封装与接口（request.ts / user.ts）
  assets/         全局样式（index.css，含 Tailwind + 主题变量）
  components/
    ui/           shadcn-vue 组件（button/input/card/table/badge/dropdown-menu）
    DataTable.vue 通用 TanStack 表格（排序/筛选/分页/列显隐）
  layouts/        布局（DefaultLayout 侧边栏 + 顶栏）
  lib/            工具（utils.ts cn()、table.ts valueUpdater）
  router/         路由与守卫
  stores/         Pinia（auth 认证、app 全局）
  views/          页面（Dashboard/Users/Login/NotFound）
```

## 新增 shadcn-vue 组件

由于网络环境通过代理，shadcn-vue CLI 在线拉取可能失败。可手动在 `src/components/ui/<name>/` 下按官方源码新增，或在网络允许时使用：

```bash
pnpm dlx shadcn-vue@latest add <component>
```

## 对接后端

- 接口基地址由 `VITE_API_BASE_URL` 控制（默认 `/api/v1`）。
- 响应约定见 `src/api/request.ts` 的 `ApiResult`（`code/message/data`），请按后端实际结构调整解包与错误码判断。
- 后端未启动时，用户列表页会回退到演示数据。
