# www API 文档展示（Scalar）+ 导航「资源」重构 — 设计稿

- 日期：2026-06-19
- 范围：在 www 新增 `/api-docs` 页，用 Scalar 渲染内容中台托管的 OpenAPI 文档（显示列表第一篇）；并把 www 导航的「帮助中心」下拉重构为「资源」，下设 博客 / 帮助文档 / 常见问题 / API 文档。
- 复用：内容中台服务端代理（`/_content/*`）与 `contentFetch`/`proxyWebFile` 模式，见 [2026-06-18-www-help-center-content-platform-design.md](2026-06-18-www-help-center-content-platform-design.md)。
- 不在范围内：多文档切换器（本期只显示第一篇）；`my` 的 ApiMcpView（不改动）；后端 Go 改动。

## 1. 背景

- `my` 是静态 SPA、无自有服务端，且 content key 只在 www；故 API 文档展示落在 **www**（已有 `/_content/*` 网关）。
- 内容中台 Pub 端点（不带 `space`，按 Key 工作空间）：
  - `GET /api-docs` → 列出当前 Key 可见的已发布文档，字段：`slug,name,description,visibility,spec_title,spec_version`（**信封** `{code,message,data}`）。
  - `GET /api-docs/:slug/spec` → 该文档的**原始 OpenAPI**（直接是文档本身、非信封，带 `Content-Type`/`ETag`/304）。
  - 已验证：现有两篇 `sample-petstore`（Swagger Petstore）、`g`（Gloryphone 开放 API）。
- www 导航现状（[AppNavbar.vue](../../../www/components/AppNavbar.vue)）：顶层有独立「博客」链接 + 「帮助中心」下拉（文档/常见问题）。

## 2. 产品决策（已与用户确认）

1. API 文档展示做在 **www**；`my` 不动。
2. 导航「帮助中心」→ **资源**，下设 博客(`/blog`)、帮助文档(`/help`)、常见问题(`/faq`)、API 文档(`/api-docs`)；移除顶层独立「博客」。
3. 用 Scalar 的 **Vue 组件** `@scalar/api-reference` 渲染。
4. **只显示列表第一篇**，不做多文档切换器。
5. 父项「资源」点击 → `/blog`；`/api-docs` 用站点默认布局（保留 nav/footer）。

## 3. 详细设计

### 3.1 导航重构（[AppNavbar.vue](../../../www/components/AppNavbar.vue)）
- `links` 中移除独立 `blog` 项；把 `help` 项改为 `resources`：
  - 父项 label = `nav.resources`，href = `/blog`。
  - children：博客(`nav.blog`→/blog)、帮助文档(`nav.docs`→/help)、常见问题(`nav.faq`→/faq)、API 文档(`nav.apiDocs`→/api-docs)。
- 桌面悬停下拉、移动端缩进子项（沿用现有结构与样式）。
- i18n（`GP_CONTENT.{zh,en}.nav`）：新增 `resources`(资源/Resources)、`apiDocs`(API 文档/API Docs)；`docs` 文案由「文档」改为「帮助文档」（en 保持 Docs）。

### 3.2 服务端代理（`www/server/routes/_content/`）
- `api-docs.get.ts` → `contentFetch<ApiDocSummary[]>('/api-docs', {})` → 列表。
- `api-docs/[slug]/spec.get.ts` → 原始 spec 回源 `/api-docs/<slug>/spec`，透传 `Content-Type`/`ETag`。
- 重构 [content.ts](../../../www/server/utils/content.ts)：抽出通用 `proxyRaw(event, path, query?)`（回源任意 Pub 路径、透传 Content-Type/ETag、404/502 处理）；`proxyWebFile` 改为调用 `proxyRaw`，spec 路由也用它。

### 3.3 /api-docs 页面（`www/pages/api-docs/index.vue`）
- `composables/useApiDocs.ts`：`useApiDocs()` → useFetch `/_content/api-docs`（默认 `[]`）。
- 页面：取 `list[0]`；若无 → 友好空态；出错 → 错误态 + 重试。
- 渲染：
  ```vue
  <ClientOnly>
    <ApiReference :configuration="{ url: `/_content/api-docs/${slug}/spec` }" />
  </ClientOnly>
  ```
  `url` 指向本站 spec 代理（Key 不进浏览器）。
- SEO：`useSeoMeta` 标题用首篇 `spec_title || name`。
- 布局：站点默认布局（nav/footer）；内容区给 Scalar 足够宽度（必要时整行/全宽容器）。

### 3.4 Scalar 依赖
- www 加 `@scalar/api-reference`；用其 Vue 组件，**client-only** 渲染（重客户端组件，避免 SSR 问题）。
- 若构建/SSR 出现兼容问题，回退用 Scalar standalone `data-url`（仍指向我们的 spec 代理）。

### 3.5 类型
- `types/content.ts` 增 `ApiDocSummary { slug, name, description, visibility, spec_title, spec_version }`。

## 4. 错误与边界
- 列表出错/空 → 非致命态（不崩 SSR），可重试；详情 spec 代理 404/502 透传。
- Key 只在 www 服务端，浏览器只访问 `/_content/api-docs` 与 `/_content/api-docs/<slug>/spec`。

## 5. 测试
- 纯/服务端：列表代理与 `proxyRaw` 路径（mock `$fetch`）；既有 buildToc/seoMetasToHead 等不受影响。
- 端到端人工/Playwright（对接活动实例）：导航「资源」四项可达；`/api-docs` 渲染首篇（Scalar UI 出现、try 可用）；无文档时空态；Key/内网域名不入客户端。

## 6. 验收标准
1. 导航顶层为「资源」下拉，含 博客/帮助文档/常见问题/API 文档 四项，桌面+移动可用；顶层不再有独立「博客」。
2. `/api-docs` 用 Scalar 渲染内容中台列表第一篇的 OpenAPI，交互式可用。
3. spec 由本站 `/_content/api-docs/<slug>/spec` 注入 Key 回源；Key/内网域名不出现在客户端。
4. 无文档/中台不可用时以友好空/错误态呈现，SSR 不崩。
5. `my` 与后端不受改动。
