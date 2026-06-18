# www 博客接入内容中台 — 设计稿

- 日期：2026-06-18
- 范围：把 `www/`（Nuxt 3 营销站）的博客从硬编码内容切换为从「内容中台」Pub API 获取，覆盖博客列表页、文章详情页与首页博客模块；补齐分类（category）与标签（tag）能力。
- 不在范围内：FAQ、帮助中心、API 文档、web-files。它们可后续复用同一套代理模式，本期不实现。

## 1. 背景与现状

当前 www 博客内容**全部硬编码**在 [`www/composables/useGp.ts`](../../../www/composables/useGp.ts)：

- `GP_CONTENT.{zh,en}.blog.posts[]`，每条字段：`id`(slug) / `cover`(数字，picsum 种子) / `cat`(分类名，自由字符串) / `date` / `read`(分钟) / `title` / `excerpt`。
- 文章正文在 `GP_ARTICLE_BODIES`（`ArticleBlock[]`，结构化块），经 `useGp().bodies` 暴露。
- 消费方：
  - [`www/pages/blog/index.vue`](../../../www/pages/blog/index.vue)：按 `cat` 过滤，featured + grid。
  - [`www/pages/blog/[slug].vue`](../../../www/pages/blog/[slug].vue)：按 `id` 取文章，渲染结构化 `blocks`，relate 推荐。
  - [`www/components/sections/BlogSection.vue`](../../../www/components/sections/BlogSection.vue)：首页取 `posts.slice(0,3)`，卡片链接到 `/blog#id`。

现状有「分类」（`cat` 自由字符串）但**没有标签**；内容无法独立于发版更新。`www/` 下当前**没有** `server/` 目录，也**没有** `.env`。

## 2. 内容中台 Pub API（已对实例验证）

- 根路径（环境变量 `NUXT_PUB_BASE_URL`）：`http://192.168.10.110:9981/api/v1/pub`
- 鉴权：请求头 `X-API-Key: <NUXT_CONTENT_API_KEY>`，**密钥，仅服务端持有**。
- 响应壳：`{ code, message, data }`，`code=0` 为成功。
- 语言：`?lang=zh-CN | en`，缺省 `zh-CN`。**站点 i18n 用 `zh`，中台用 `zh-CN`**，需映射。
- 内容空间：博客相关端点需 `?space=blog`。

| 方法 | 路径 | 用途 | 关键参数 |
| --- | --- | --- | --- |
| GET | `/articles` | 文章列表 | `space`(必填) `lang` `q` `category_id` `tag_id` `sort`(`directory`\|`published_desc`) `page`(从1) `size`(默认20，最大50) |
| GET | `/articles/:slug` | 单篇（含 `body_html`） | `space` `lang` |
| GET | `/article-categories` | 分类列表 | `space` `lang` |
| GET | `/article-tags` | 标签列表 | `space` `lang` |
| GET | `/seo/resolve?path=` | 解析某 URL 的 SEO TDK | （不带 space，按 Key 工作空间） |

**列表项**字段：`id, slug, lang, title, path, full_url, summary, cover_url, cover_alt, seo_title, seo_description, seo_keywords, category{id,slug,name}, tags[]{id,slug,name}, published_at, created_at, updated_at`。
**详情**在列表项基础上增加：`body_html`、`available_langs`（如 `['zh-CN','en']`）。
分类/标签返回：`[{id, slug, name}]`。

已验证：`size` 受 `size` 参数控制（非 `pageSize`）；过滤用数值 `category_id`/`tag_id`（非 slug）；分页用 `page`/`size`，`data.total` 为过滤后总数。

## 3. 产品决策（已与用户确认）

1. 博客列表：**分类 + 标签筛选 + 分页**（不含关键词搜索框）。
2. 标签：**可点击**，点击后按该标签过滤列表。
3. 中台不可用时：**优雅的空/错误态**（SSR 不崩溃，提供重试），不保留静态兜底文章。
4. `body_html` 直接渲染；硬编码示例文章**删除**（不作为兜底）。

## 4. 架构：Nuxt 服务端代理

密钥不得进入浏览器。采用 **Nuxt server-route 代理**：新增 `server/routes/_content/blog/*` 路由，密钥存于服务端 `runtimeConfig`，由这些路由调用中台；页面只调用本站 `/_content/blog/*`。首屏 SSR 与客户端路由切换（点击卡片进详情）均经由本站服务端，无 CORS、浏览器无需直达内网主机，密钥不外泄。

```
浏览器 ──/_content/blog/*──▶ Nuxt server 路由 ──X-API-Key──▶ 内容中台 Pub API
                         (持有密钥)
```

## 5. 详细设计

### 5.1 配置与密钥

[`www/nuxt.config.ts`](../../../www/nuxt.config.ts) 的 `runtimeConfig` 顶层（服务端私有，**不**放 `public`）：

```ts
runtimeConfig: {
  backendBaseUrl: 'http://localhost:9981/api/v1', // 既有，不动
  pubBaseUrl: '',        // ← NUXT_PUB_BASE_URL
  contentApiKey: '',     // ← NUXT_CONTENT_API_KEY
  contentSpace: 'blog',  // 博客空间 slug
  public: { /* 既有 */ },
}
```

Nuxt 的环境覆盖规则使 `pubBaseUrl`↔`NUXT_PUB_BASE_URL`、`contentApiKey`↔`NUXT_CONTENT_API_KEY` 自动对应。新增 `www/.env`（已被 `www/.gitignore` 覆盖）写入两个变量真实值；新增 `www/.env.example` 记录变量名与示例（不含真实密钥）。

### 5.2 服务端代理（`www/server/`）

**`server/utils/content.ts`**
- `langToApi(locale)`：`'zh' → 'zh-CN'`，其余 `→ 'en'`（纯函数，单测覆盖）。
- `contentFetch(path, query)`：拼 `pubBaseUrl + path`，带 `X-API-Key` 头，`$fetch` 调中台；解封 `{code,message,data}`：`code!==0` 抛 `createError`（携带中台 message 与对应 HTTP 码），返回 `data`。

**路由**（均读 query 中的 `lang`，由页面传入；默认 `published_desc` 排序）：
- `server/routes/_content/blog/posts.get.ts` → `contentFetch('/articles', {space:'blog', lang, page, size, category_id, tag_id, sort})` → 返回 `{ list, total }`。
- `server/routes/_content/blog/[slug].get.ts` → `contentFetch('/articles/'+slug, {space:'blog', lang})` → 返回详情对象；中台 404 透传为 404。
- `server/routes/_content/blog/taxonomy.get.ts` → 并行 `contentFetch('/article-categories',…)` 与 `contentFetch('/article-tags',…)` → 返回 `{ categories, tags }`。

上游失败时路由返回 502（携带原始 message），由页面渲染错误态。

### 5.3 客户端组合式（`www/composables/useBlog.ts`）

封装对 `/_content/blog/*` 的 `useFetch`，**以 i18n locale 作为 key**，切语言自动重取；query 变化时 `watch` 重取：

- `useBlogPosts(opts)`：`opts` = `{ page, size, categoryId, tagId, sort }`（响应式），返回 `{ list, total, pending, error, refresh }`。
- `useBlogPost(slug)`：返回 `{ post, pending, error }`；可在 setup 同步判断 404。
- `useBlogTaxonomy()`：返回 `{ categories, tags, pending, error }`。
- 纯辅助函数（单测）：`langToApi`、`formatDate(iso, locale)`、`readingTime(html)`（按约 200 wpm，从 `body_html` 文本估算）。

### 5.4 页面与组件

**`pages/blog/index.vue`**
- 数据：`useBlogTaxonomy()` 渲染分类 chips 与标签 chips；`useBlogPosts()` 取列表。
- 筛选状态写入 URL：`/blog?category=<slug>&tag=<slug>&page=<n>`。URL 用 slug，进 API 前用 taxonomy 映射成 `category_id`/`tag_id`。点击标签 chip → 设置 `tag` query。
- 布局：保留 featured（当前页第 1 条）+ grid + 分页控件（页码上一页/下一页 + 页号，按 `total`/`size` 计算页数；不采用「加载更多」）。卡片字段：`cover_url`/`cover_alt`、`category.name`、`published_at`（`formatDate`）、标签 chips（可点击）。
- 状态：`pending` 显示骨架/占位；`error` 显示错误态 + 重试（`refresh()`）；`list` 为空显示空态。文案取 i18n。

**`pages/blog/[slug].vue`**
- `useBlogPost(slug)`：缺失则 `createError({statusCode:404,fatal:true})`。
- 正文：`body_html` 经统一渲染组件以 `v-html` 输出到 `.prose` 容器（集中保留未来净化的入口）。
- 元信息：`category.name`、`published_at`、`readingTime(body_html)`（配合既有 `t.blog.min`）、标签 chips（可点击 → `/blog?tag=<slug>`）。
- 推荐：再取同 `category_id` 的若干篇，排除当前。
- SEO：`useHead` 用 `seo_title||title`、`seo_description||summary`、`seo_keywords`、`cover_url`（og:image）。

**`components/sections/BlogSection.vue`（首页）**
- 改为 `useBlogPosts({ size: 3, sort: 'published_desc' })`。
- 卡片链接由 `/blog#id` 改为 `/blog/{slug}`（进详情页）。
- `error`/空：仅渲染区块标题，不崩溃。

### 5.5 删除硬编码内容

- [`useContent.ts`](../../../www/composables/useContent.ts)：移除 `BlogPostMeta`、`BLOG_POSTS`、`useBlog`（`PRICING_PLANS` 等无关导出保留）。
- [`useGp.ts`](../../../www/composables/useGp.ts)：移除 `GP_CONTENT.{zh,en}.blog.posts[]`、`ArticleBlock`、`GP_ARTICLE_BODIES`、`useGp().bodies`。**保留** `blog` 标签键（`eyebrow/title/sub/all/readMore/min`）。
- 确认无其它引用残留（`bodies`、`BLOG_POSTS`、`t.blog.posts`）。

### 5.6 i18n 新增文案

在 `GP_CONTENT.{zh,en}.blog` 增补：`tags`(标签/Tags)、`category`(分类/Category)、`empty`(暂无文章/No articles yet)、`error`(内容加载失败/Couldn't load articles)、`retry`(重试/Retry)、分页 `prev`(上一页/Prev)、`next`(下一页/Next)。保留既有键。

## 6. 错误与边界处理

- 服务端 `contentFetch` 统一解封与抛错；路由对上游失败返回 502。
- 详情页真实 404 → fatal 404；列表/首页错误 → 非致命错误态（不 `throw`），保证 SSR 出页。
- 语言回退由中台处理；`available_langs` 可用于后续做语言切换提示（本期仅用于不报错）。
- `lang` 全程由页面按 i18n locale 计算并作为 query 传入，服务端路由保持无状态。

## 7. 测试策略

- 纯函数单测：`langToApi`、`formatDate`、`readingTime`（Vitest）。
- 服务端路由：对 `contentFetch` 解封/抛错路径做单测（mock `$fetch`）。
- 端到端人工/Playwright 验证（对接活动实例）：列表筛选+分页、标签点击过滤、详情渲染与 404、首页 3 条、断网时的错误态。

## 8. 验收标准

1. `/blog` 列表、`/blog/:slug` 详情、首页博客模块的数据均来自内容中台，无任何硬编码文章残留。
2. 列表支持分类筛选、标签筛选（可点击）与分页；URL 可分享并还原筛选态。
3. 详情页正确渲染 `body_html`、分类、日期、阅读时长、可点击标签与推荐。
4. 切换 zh/en 时正确映射 `zh-CN`/`en` 并重新取数。
5. 密钥不出现在任何客户端产物/网络请求中（浏览器只访问本站 `/_content/blog/*`）。
6. 中台不可用时页面以友好空/错误态呈现且可重试，SSR 不崩溃。
