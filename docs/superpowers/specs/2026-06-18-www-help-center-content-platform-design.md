# www 帮助中心接入内容中台（文档 + FAQ） — 设计稿

- 日期：2026-06-18
- 范围：在 www 顶部导航把「帮助中心」做成一级下拉，下设两个子模块——**文档**（接入内容中台 `help` 空间，目录树文档站）与 **常见问题**（接入 `faq` 空间，替换现有硬编码 FAQ）。
- 复用：博客接入已落地的服务端代理 + 组合式模式，见 [2026-06-18-www-blog-content-platform-design.md](2026-06-18-www-blog-content-platform-design.md)。
- 不在范围内：api-docs / web-files / 站内全文搜索框（后续可用同一模式扩展）。

## 1. 背景与现状

- 导航 [AppNavbar.vue](../../../www/components/AppNavbar.vue) 的 `help` 项当前指向 `/#help`，落到首页的 [FaqSection.vue](../../../www/components/sections/FaqSection.vue)（`id="help"`）。
- FAQ 现为硬编码：[useGp.ts](../../../www/composables/useGp.ts) 的 `GP_CONTENT.{zh,en}.faq.items`（`[问题, 答案]` 文本对），由 [FaqSection.vue](../../../www/components/sections/FaqSection.vue)（首页板块）与 [pages/faq.vue](../../../www/pages/faq.vue) 渲染为手风琴。
- 内容中台服务端代理工具 [server/utils/content.ts](../../../www/server/utils/content.ts)（`contentFetch` + `langToApi`）已存在，本次直接复用。

## 2. 内容中台 Pub API（已对实例验证）

鉴权、信封、语言映射同博客（`X-API-Key`、`{code,message,data}`、站点 `zh`→中台 `zh-CN`）。

**help 空间（目录编排）：**
- `GET /directory?space=help&lang=` → 目录树，节点为：
  - `group`：`{ kind:'group', title, slug, collapsed, children:[...] }`
  - `article`：`{ kind:'article', title, slug, url:'/help/<slug>', collapsed }`
  - 实测 6 组、约 19 篇、两级深度（组 → 文章）。
- `GET /articles/:slug?space=help&lang=` → 详情，含 `body_html`（正文用 `<h2>/<h3>`，**无 id**）、`available_langs`、`seo_*`、`title`、`summary` 等（结构同博客详情）。

**faq 空间（无目录树）：**
- `GET /articles?space=faq&lang=` → 10 篇，`title` = 问题，`category` 形如 `Account` / `Billing`，无 `body_html`（列表不含正文）。
- `GET /articles/:slug?space=faq&lang=` → 详情，`body_html` = 答案。

## 3. 产品决策（已与用户确认）

1. 导航「帮助中心」做成下拉，下设 **文档**（→ `/help`）与 **常见问题**（→ `/faq`）。
2. 文档页采用**完整三栏**：左目录树 + 正文 + 右 TOC。
3. `/help` 根路径**重定向到目录第一篇**（如 `/help/quickstart`）。
4. 首页**保留** FAQ 板块，改为从 `faq` 空间取数。
5. `/faq` 页按**分类分组**展示。

## 4. 架构

沿用 **Nuxt 服务端代理**：新增 `/api/help/*`、`/api/faq` 路由，密钥仅服务端持有；页面只调用本站接口。`/help` 与 `/faq` 首屏 SSR 与客户端导航均经本站服务端，无 CORS、密钥不外泄。

```
浏览器 ──/api/help/*、/api/faq──▶ Nuxt server 路由 ──X-API-Key──▶ 内容中台 Pub API
```

## 5. 详细设计

### 5.1 导航改造（[AppNavbar.vue](../../../www/components/AppNavbar.vue)）

- 把 `help` 项由「单链接 → `/#help`」改为「父项**帮助中心**」：
  - 桌面：轻量 CSS 悬停下拉，列 **文档**(`/help`) 与 **常见问题**(`/faq`)；父项点击落到 `/help`。
  - 移动端：burger 菜单内将其展开为两个缩进子项。
- i18n 增 `nav.docs`（文档 / Docs）与 `nav.faq`（常见问题 / FAQ）——二者当前 `nav` 对象中均无；复用 `nav.help`（帮助中心 / Help）。
- 仅改 `help` 这一项的结构，其余导航项不动。

### 5.2 服务端代理（新增，复用 `contentFetch`）

- `server/api/help/directory.get.ts` → `contentFetch('/directory', { space:'help', lang })` → 目录树。
- `server/api/help/[slug].get.ts` → `contentFetch('/articles/'+slug, { space:'help', lang })` → 详情；中台 404 透传 404。
- `server/api/faq.get.ts` → 服务端聚合：
  1. `contentFetch('/articles', { space:'faq', lang })` 取列表（含 `category`、`slug`、`title`）；
  2. `Promise.all` 并行取各篇 `contentFetch('/articles/'+slug, { space:'faq', lang })` 拿 `body_html`；
  3. 按 `category` 分组，返回 `[{ category, items:[{ slug, question, answerHtml }] }]`。
  - 规模约 10 篇，服务端并行、可加 `defineCachedEventHandler` 短缓存；FAQ 增大后再改为懒加载答案。

### 5.3 组合式

- `composables/useHelp.ts`：
  - `useHelpDirectory()` → useFetch `/api/help/directory`（默认 `[]`）。
  - `useHelpArticle(slug)` → 返回可 `await` 的 useFetch（详情页用以同步处理 404）。
  - 纯函数 `buildToc(bodyHtml)`：解析正文，给 `h2/h3` 注入 slug 化锚点 id（去重），返回 `{ html, toc:[{ id, text, level }] }`；SSR 期执行，TOC 进首屏。
- `composables/useFaq.ts`：
  - `useFaq()` → useFetch `/api/faq`（默认 `[]`），返回按分类分组的 Q&A。

### 5.4 文档模块（/help）

- 路由：
  - `pages/help/index.vue`：取目录树，`navigateTo` 重定向到第一篇 article 的 `/help/<slug>`（SSR 301/302）；目录空或出错时显示空/错误态。
  - `pages/help/[slug].vue`：`useHelpArticle(slug)`，无效 slug → `createError 404`。
- 布局组件：
  - `components/docs/DocsLayout.vue`：三栏栅格（左 `DocsSidebar` ｜ 中 正文 ｜ 右 `DocsToc`），响应式：≤980px 隐藏右 TOC，≤640px 目录树收进顶部抽屉/下拉。
  - `components/docs/DocsSidebar.vue`：渲染目录树（组标题 + 文章链接，当前篇高亮）；真实 `<a>`（NuxtLink）。
  - `components/docs/DocsToc.vue`：渲染 `buildToc` 的 toc；滚动监听高亮当前小节（`IntersectionObserver`，仅客户端）。
- 正文经 [ArticleBody.vue](../../../www/components/ArticleBody.vue) 渲染注入了锚点 id 的 html；正文样式复用既有 `.article-body` 规则并按文档页布局微调。
- SEO：`useSeoMeta` 取 `seo_title||title`、`seo_description||summary`、`cover_url`。

### 5.5 FAQ 模块

- `components/sections/FaqSection.vue`（首页板块，`id="help"`）：改为 `useFaq()`，平铺取前 6 条问答手风琴；答案经 `ArticleBody` 渲染 `body_html`。出错/空 → 仅渲染区块标题。
- `pages/faq.vue`：用 `useFaq()` 按分类分组的手风琴（分类标题 + 其下问答）；保留页面既有结构（含 `CtaSection`）。
- 移除 `useGp.ts` 的 `faq.items` 硬编码，保留 `faq.eyebrow/title` 标签；FAQ 文案改由中台供给。

### 5.6 错误与边界

- 详情页（文档）真实 404 → fatal 404；列表/目录/FAQ 出错 → 非致命错误态（不崩 SSR），可重试。
- `/help` 重定向：目录为空或上游失败时不重定向，显示错误/空态。
- `lang` 全程按 i18n locale 计算并作为 query 传入；服务端路由无状态。
- 密钥不进浏览器；浏览器只访问本站 `/api/help/*`、`/api/faq`。

## 6. 测试策略

- 纯函数单测（`node --test`）：`buildToc`（锚点注入/去重、toc 结构）、FAQ 分组聚合、`langToApi`。
- 服务端路由：mock `$fetch` 测 `/api/faq` 聚合与错误路径。
- 端到端人工/Playwright（对接活动实例）：导航下拉、`/help` 重定向、文档三栏与 TOC 锚点跳转、目录当前篇高亮、`/faq` 分类分组、首页 FAQ 板块、无效 slug 404、断网错误态、zh/en 切换。

## 7. 验收标准

1. 导航「帮助中心」为下拉，含 **文档**(`/help`) 与 **常见问题**(`/faq`) 两个子项，桌面与移动端均可用。
2. `/help` 重定向到目录第一篇；`/help/<slug>` 渲染三栏（左目录树高亮当前篇、中正文、右 TOC 可点击跳转并随滚动高亮）。
3. 文档与 FAQ 数据均来自内容中台，无硬编码残留（保留纯标签文案）。
4. `/faq` 按分类分组展示问答；首页 FAQ 板块从 `faq` 空间取数。
5. zh/en 正确映射 `zh-CN`/`en` 并重新取数。
6. 密钥不出现在任何客户端产物/请求中。
7. 中台不可用时：文档/FAQ 以友好空/错误态呈现且 SSR 不崩；文档详情无效 slug 返回 404。
