# TC-13 www 内容站（营销 / 博客 / 帮助 / API 文档）

> 覆盖：营销页 SSR 渲染与 SEO、博客列表/分页/详情/反馈、帮助中心文档、Scalar API 文档、SSR 首屏登录态、Cookie 同意（GDPR）、i18n 中英切换与主题、www BFF `_api`/`_content` 前缀约定。
> 关联：`www/pages/*`、`www/components/*`（BlogList/BlogPager/ArticleBody/ArticleFeedback.client/ScalarDoc.client/CookieConsent/FaqAccordion/AppNavbar/PromoBanner/TweakPanel）、`www/server/routes/{_api,_content}/*`、`www/composables/*`、`www/plugins/auth.server.ts`；文档《产品功能清单》§5、`www-bff-prefix-convention`、`integration-notes.md`。
> **背景**：本域此前在 TC-09/TC-08 只做「原型冒烟」，后端内容中台（Pub API）与营销接口已建成，本文档将其升级为**完整验收用例**。类型以 UI / E2E / 兼容 / 集成为主，**多为人工或 Playwright，后续 Plan C 自动化**。

## 一、营销页渲染（首页 / 定价 / FAQ）

| 用例ID | 标题 | 类型 | 优先级 | 前置条件 | 步骤 | 预期结果 |
| --- | --- | --- | --- | --- | --- | --- |
| TC-13-001 | 首页各 Section 渲染 | UI | P1 | www dev :3000 | 访问 `/` | 依次渲染 Hero/Features/UseCases/Compare/Pricing/Testimonials/Downloads/Blog/FAQ/Cta（见 `pages/index.vue`）；无控制台报错 |
| TC-13-002 | 首页 SSR 首屏非空 | 集成 | P1 | 直连 www | `curl -s http://localhost:3000/` | 返回 HTML 已含 Hero 文案（`t.hero.title1`）等正文，非空壳等待 hydrate |
| TC-13-003 | 推广横幅 | UI | P2 | — | 访问首页 | 顶部 `PromoBanner` 展示 `promo.tag/title/body/countdown/cta`；点击整条跳 `/#pricing`（当前实现为链接，**无关闭按钮**） |
| TC-13-004 | 定价页 Hero 与徽章 | UI | P1 | — | 访问 `/pricing` | 展示 `pricing.eyebrow/title/sub`；`trial` 存在时显示试用 pill、`gift>0` 显示赠送 pill；「免费开始」CTA 指向 `myAppPath`（默认 `/my/`） |
| TC-13-005 | 定价数字来自后端 | 集成 | P1 | 后端 `/api/open/v1/billing/pricing` 可用 | 访问 `/pricing` 观察资源卡「最低」单价 | 数字来自 `/_api/pricing` 代理的 `PublicPricing`（席位/包月数/临时时长叠加数量×时长最深折扣算得，见 `usePricing.lowest`） |
| TC-13-006 | 定价后端不可用兜底 | 集成 | P1 | 关闭/断开后端 | 访问 `/pricing` | `/_api/pricing` 返回 `null`，页面用 `DEFAULT_PRICING`（镜像后端 seed）渲染，不白屏不报错 |
| TC-13-007 | 定价页计费 FAQ 过滤 | UI | P2 | FAQ 数据存在 | 访问 `/pricing` 底部 FAQ | 仅展示计费相关问答（`faqKeywords` 命中：价格/费用/试用/折扣/席位/pric/bill/trial 等） |
| TC-13-008 | FAQ 页分组展开 | UI | P2 | 中台 faq 空间有数据 | 访问 `/faq` | 按 `group.name` 分组，`FaqAccordion` 每项可展开/收起；面包屑 首页 > 常见问题 |
| TC-13-009 | FAQ 空/错态 | UI | P2 | 中台 faq 无数据或不可用 | 访问 `/faq` | 空显示 `t.faq.empty`；错显示 `t.faq.error` + 「重试」按钮，点击 `refresh()` 重取 |
| TC-13-010 | 响应式布局 | 兼容 | P2 | — | 375 / 768 / 1440 三档宽度访问首页与定价 | 导航折叠为汉堡菜单（`.nav-burger` → Teleport 移动菜单）、栅格自适应、无横向滚动/元素溢出 |
| TC-13-011 | meta / SEO 中台注入 | 集成 | P1 | 中台配置了该 path 的 SEO | 访问任意页并查看 `<head>` | `useSeoConfig` 经 `/_content/seo?path=` 解析 TDK，`tagPriority:1` 压过页面默认 title/meta；未命中则保留页面自身值 |
| TC-13-012 | 文章级 og/SEO | 接口 | P2 | 存在文章 | 访问 `/blog/:slug` 查看 head | `useSeoMeta` 注入 `seo_title/seo_description/seo_keywords/og:*`（回退 `title/summary/cover_url`），`og:type=article` |

## 二、博客（列表 / 分页 / 详情 / 反馈）

| 用例ID | 标题 | 类型 | 优先级 | 前置条件 | 步骤 | 预期结果 |
| --- | --- | --- | --- | --- | --- | --- |
| TC-13-020 | 博客列表首页 | UI | P1 | 中台 blog 空间有文章 | 访问 `/blog` | 第 1 页首条作 featured 大图，其余 12/页卡片（封面/日期/摘要/标签）；分类 chips 与标签 chips 均为真实 `<a>` |
| TC-13-021 | 列表空/错态 | UI | P2 | 无文章或中台不可用 | 访问 `/blog` | 空显示 `t.blog.empty`；错显示 `t.blog.error` + 「重试」 |
| TC-13-022 | 分页伪静态路由 | UI | P1 | 文章数 > 12 | 访问 `/blog/page/2` | `BlogPager` 渲染数字页码（首尾常驻+中间省略），全部为真实 `<a>`（`hrefFor`→`localePath(blogListPath)`）；边界「上一页/下一页」为禁用态 span |
| TC-13-023 | 非法页码 404 | 集成 | P2 | — | 访问 `/blog/page/abc` 或 `/blog/page/0` | `Number.parseInt` 非有限或 <1 → `createError 404`（fatal） |
| TC-13-024 | 按分类过滤 | UI | P2 | 存在分类 | 点分类 chip 或访问 `/blog-categories/<slug>` | 仅列该分类（含子分组）文章；分类由 `/_content/blog/taxonomy` 的 directory group 递归摊平提供 |
| TC-13-025 | 按标签过滤 | UI | P2 | 存在标签 | 点标签 chip 或访问 `/blog-tags/<slug>` | 按 `tag_id` 过滤（slug 经 taxonomy 解析为 id）；标题 `#标签名` |
| TC-13-026 | 无效分类/标签 slug | 集成 | P2 | — | 访问不存在的 `/blog-categories/xxx` | `BlogList` await taxonomy 后未命中 → `createError 404`（fatal） |
| TC-13-027 | 文章详情渲染 | UI | P1 | 存在文章 | 访问 `/blog/:slug` | 渲染分类/发布日期/阅读时长（`readingMinutes(body_html)`）/封面/正文（`ArticleBody` v-html）/可点标签/同分类推荐（≤3，排除本篇） |
| TC-13-028 | 更新时间条件展示 | UI | P2 | 文章有 updated_at | 访问详情 | 仅当 `fmt(updated_at) !== fmt(published_at)` 才另显「更新于」，避免与发布日期重复 |
| TC-13-029 | 无效文章 slug 走 404 | 集成 | P1 | — | 访问 `/blog/不存在` | 上游 404 经 `_content/blog/[slug]` 透传，SSR 阶段 `useBlogPost` 拿到 error → `throw 404`（fatal）；中台错误则 502 |
| TC-13-030 | 文章正文 XSS 入口收敛 | 安全 | P1 | — | 审查 `ArticleBody.vue` | 全站 v-html **唯一入口**在此，内容来自受信内部 CMS；确认无用户可控 HTML 直插其它组件 |
| TC-13-031 | 文章反馈-赞踩 | UI | P2 | 帮助文档页（`vote`）| 打开 `/help/:slug` 点「有帮助/没帮助」 | `ArticleFeedback` 命中 `/_content/feedback/:slug/reaction`（POST），乐观更新计数；再点同向撤销（`nextVote`→0）；失败回滚 |
| TC-13-032 | 文章反馈-留言 | UI | P2 | 帮助文档页（`feedback`）| 展开「留言反馈」，空内容提交 | 空内容前端拦截提示 `feedback.emptyError`；填内容+可选联系方式提交，命中 `/_content/feedback/:slug/feedback`，成功显示 `thanksFeedback` |
| TC-13-033 | 访客身份 visitor_id | 集成 | P2 | — | 未登录/登录分别提交反馈 | 未登录用 localStorage `gp_visitor_id`（首访生成持久化）；登录用 `u:<id>` 不落库；登录附 `meta`（user_id/nickname/phone）JSON，仅后台明细识别、前台不展示（`useFeedback`） |
| TC-13-034 | 密钥不进浏览器 | 安全 | P0 | — | DevTools Network 观察反馈/内容请求 | 浏览器仅请求本站 `/_content/*`；`X-API-Key`（contentApiKey）只在 www server 侧注入回源，前端载荷/响应头均无密钥 |

## 三、帮助中心（文档浏览 / 反馈）

| 用例ID | 标题 | 类型 | 优先级 | 前置条件 | 步骤 | 预期结果 |
| --- | --- | --- | --- | --- | --- | --- |
| TC-13-040 | /help 重定向首篇 | 集成 | P1 | 中台 help 目录非空 | 访问 `/help` | SSR `useHelpDirectory` 取目录树，`firstArticleUrl` → `navigateTo(302)` 到首篇文档 |
| TC-13-041 | /help 目录为空兜底 | UI | P2 | help 目录空/出错 | 访问 `/help` | 不强跳，显示 `t.blog.error` + 返回首页链接 |
| TC-13-042 | 文档三栏布局 | UI | P2 | 存在文档 | 访问 `/help/:slug` | `DocsLayout`：左目录树（`DocsSidebar`/`DocsTreeNode` 递归，可折叠、当前篇高亮）+ 正文（注入锚点）+ 右 TOC（`buildToc`） |
| TC-13-043 | 文档面包屑分组路径 | UI | P2 | 嵌套分组文档 | 访问 `/help/:slug` | 面包屑 首页 > 帮助文档 > 分组路径…（`docTrail`，分组无落地页渲染纯文本）> 当前文章 |
| TC-13-044 | 无效文档 slug 404 | 集成 | P2 | — | 访问 `/help/不存在` | `useHelpArticle` 上游 404 透传 → `throw 404`（fatal）；其它错误 502 |
| TC-13-045 | 文档更新时间 | UI | P2 | 文档有 updated_at | 访问文档 | 底部显示 `t.docs.updated: <日期>`，随后为 `ArticleFeedback`（vote+feedback，space=help） |
| TC-13-046 | 帮助中心无独立搜索 | UI | P2 | — | 浏览 `/help/*` | **确认现无站内全文搜索框**（仅目录树导航；API 文档另由 Scalar 内置搜索承担），验收以目录导航为准，勿臆造搜索用例 |

## 四、API 文档（Scalar）

| 用例ID | 标题 | 类型 | 优先级 | 前置条件 | 步骤 | 预期结果 |
| --- | --- | --- | --- | --- | --- | --- |
| TC-13-050 | Scalar 渲染开放 API | UI | P1 | 中台有已发布 API 文档 | 访问 `/api-docs` | 取 `/_content/api-docs` 列表首篇，`ScalarDoc` 自托管 standalone（`/vendor/scalar-standalone.js`）挂载，加载期显示骨架屏、就绪后淡出 |
| TC-13-051 | spec 经 BFF 代理注入 Key | 安全 | P0 | — | 观察 `specUrl` 与 Network | spec 指向 `/_content/api-docs/:slug/spec`（`proxyRaw` 回源注入 `X-API-Key`），浏览器不接触 pubBaseUrl/密钥；透传上游 Content-Type/ETag |
| TC-13-052 | 列表空/错态 | UI | P2 | 无文档或中台不可用 | 访问 `/api-docs` | 空显示 `t.blog.empty`；错显示 `t.blog.error` + 「重试」 |
| TC-13-053 | Scalar 主题跟随站点 | UI | P2 | — | 在 `/api-docs` 切暗色/主题色 | `customCss` 把 Scalar 变量映射到站点 token，明暗与 accent 联动；明暗「真正翻转」时重建实例（`darkMode` 跟随 `colorMode`） |
| TC-13-054 | Scalar 品牌隐藏 | UI | P2 | — | 查看 Scalar 侧栏/页脚 | Powered by Scalar / Open API Client / `a[href*=scalar.com]` 均 `display:none` |
| TC-13-055 | 刷新不白屏（水合时序） | 集成 | P1 | — | 在 `/api-docs` 直接刷新 | `scheduleMount`（nextTick+rAF）把挂载推到水合结束后，避免嵌套 createApp 与水合冲突导致空白 |

## 五、SSR 首屏登录态

| 用例ID | 标题 | 类型 | 优先级 | 前置条件 | 步骤 | 预期结果 |
| --- | --- | --- | --- | --- | --- | --- |
| TC-13-060 | 已登录刷新首屏即登录态 | 集成 | P0 | nginx 统一域名；已登录（有 HttpOnly `user_token`）| 刷新任意 www 页面 | SSR 阶段 `auth.server.ts`→`fetchAuthUserOnServer` 携 Cookie 调后端 `/user/me`，填充 `useAuthUser`；navbar **首屏即**显示「控制台/退出」，**无闪烁**（非先渲染登录/注册再切换） |
| TC-13-061 | curl 验 SSR 登录态 | 集成 | P1 | 有效登录 Cookie | `curl -H 'Cookie: user_token=<jwt>' <统一域名>/` | 返回 HTML 首屏已含「控制台」文案（`nav.console`），证明服务端已解析登录态 |
| TC-13-062 | 登出态首屏一致 | 集成 | P1 | 无 `user_token` | 直连或 curl `/` | 首屏 navbar 显示「登录/免费注册」；`fetchAuthUserOnServer` 在无 Cookie 时早退不打后端 |
| TC-13-063 | 401/后端故障静默降级 | 集成 | P1 | Cookie 无效或后端不可用 | 携失效 Cookie 访问 `/` | `/user/me` 401/网络错被静默忽略，回退未登录态，页面正常渲染不报错 |
| TC-13-064 | 客户端登出联动 | UI | P1 | 已登录 | 点 navbar「退出登录」 | `logoutAuthUser` 调 `/api/v1/user/auth/logout`（POST 由后端 Set-Cookie 清 HttpOnly），清本地 `useAuthUser` 并 `refreshNuxtData`，navbar 切回登录/注册 |
| TC-13-065 | 控制台/登录跳转前缀 | UI | P1 | — | 观察 navbar 按钮 href | 登录`{myBase}/login`、注册`{myBase}/register`、控制台`{myBase}/`（`myBase`=去尾斜杠的 `public.myAppPath`，默认 `/my`），为真实 `<a>` 跨 SPA 跳转非 www 内部路由 |

## 六、Cookie 同意（GDPR）

| 用例ID | 标题 | 类型 | 优先级 | 前置条件 | 步骤 | 预期结果 |
| --- | --- | --- | --- | --- | --- | --- |
| TC-13-070 | 首访弹同意 banner | UI | P1 | 无 `gp-consent` cookie | 访问任意页 | 底部弹出 `CookieConsent`（`hasDecided=false`）；含标题/说明/「了解更多」链 `/cookies` |
| TC-13-071 | 全部接受 | UI | P1 | banner 可见 | 点「全部接受」 | 写 `gp-consent`（v=1, ts, necessary=true, functional/analytics/marketing=true），banner 消失 |
| TC-13-072 | 仅必要 | UI | P1 | banner 可见 | 点「仅必要」 | 写 `gp-consent`，仅 `necessary=true`，其余 false |
| TC-13-073 | 自定义偏好 | UI | P2 | banner 展开「自定义」 | 勾选部分类别→「保存偏好」 | 按选择落库；`necessary` 复选框始终 checked+disabled，显示「始终启用」不可改 |
| TC-13-074 | 决策后不再弹 | UI | P1 | 已决策 | 刷新/再访问 | banner 不再出现（`useCookie` SSR+客户端共享同值，`hasDecided=true`） |
| TC-13-075 | schema 版本升级重弹 | 集成 | P2 | 旧 `gp-consent` 的 `v !== CONSENT_VERSION` | 访问任意页 | `consent` computed 视为 null → 重新弹 banner |
| TC-13-076 | 半年过期重征求 | 集成 | P2 | cookie 超 `COOKIE_MAX_AGE`（180 天）| 访问 | cookie 失效 → 重新弹 banner |
| TC-13-077 | 政策页改偏好 | UI | P2 | — | 访问 `/cookies` 改开关→「保存偏好」 | 偏好更新；显示「上次更新：<本地时间>」（未决策显示 `notDecided`）；提供全部拒绝/全部接受/保存 |
| TC-13-078 | 同意状态被消费 | 集成 | P2 | analytics=false | 加载页面 | 未加载分析类脚本；置 true 后加载（`allowed('analytics')` 语义；`necessary` 恒 true） |

## 七、i18n / 主题 / 持久化

| 用例ID | 标题 | 类型 | 优先级 | 前置条件 | 步骤 | 预期结果 |
| --- | --- | --- | --- | --- | --- | --- |
| TC-13-080 | 中英切换 | UI | P1 | — | navbar 语言弹层选 简体中文/English | 全站文案在 zh/en 切换（`GP_CONTENT`/i18n），无缺失 key；no_prefix 策略 URL 不带语言前缀 |
| TC-13-081 | 语言持久化 | 集成 | P1 | — | 切到 zh 后刷新 | 语言由 `gp-lang` cookie 记住（`detectBrowserLanguage.useCookie`），刷新维持所选语言 |
| TC-13-082 | 内容随语言回源 | 集成 | P2 | 中台有中英内容 | zh/en 分别访问博客/FAQ | BFF `langToApi` 把 `zh→zh-CN`、其余→`en` 传中台，返回对应语言内容 |
| TC-13-083 | 默认主题薄荷色 | UI | P1 | 首次访问（无 `gp-accent`）| 加载首页 | 默认 `teal`；`useThemeColor` 经 `useHead` 在 `<html data-accent>` SSR 首屏即生效，无颜色闪烁 |
| TC-13-084 | TweakPanel 切主题色 | UI | P2 | — | 打开 Tweak 面板选配色 | 8 色（purple/blue/teal/green/orange/pink/indigo/rose）即时生效，写 `gp-accent`（一年）持久化 |
| TC-13-085 | 明暗模式切换 | UI | P2 | — | navbar 或 TweakPanel 切 明/暗/跟随系统 | `@nuxtjs/color-mode` 切换 `.dark`，`colorMode.preference` 写 `gp-color-mode`；刷新维持 |

## 八、www BFF 前缀约定与集成

| 用例ID | 标题 | 类型 | 优先级 | 前置条件 | 步骤 | 预期结果 |
| --- | --- | --- | --- | --- | --- | --- |
| TC-13-090 | `_api` 前缀避后端冲突 | 集成 | P1 | — | 审查 `server/routes/_api/*` | www BFF 用 `_api/`（pricing、trial-overview）代理**后端营销接口**（`/api/open/v1/billing/*`），刻意区别于后端 `/api/*`；**勿改名** |
| TC-13-091 | `_content` 前缀内容中台 | 集成 | P1 | — | 审查 `server/routes/_content/*` | www BFF 用 `_content/`（blog/help/faq/api-docs/feedback/seo）代理**内容中台 Pub API**；`X-API-Key` 只服务端注入 |
| TC-13-092 | 上游状态码语义透传 | 接口 | P2 | — | 触发中台 404 / 4xx / 5xx | `contentFetch`/`contentPost`：上游 404→404、写接口 4xx 透传（403 scope/422 校验）、其余→502，携中台 message |
| TC-13-093 | 中台未配置报错 | 集成 | P2 | 缺 `NUXT_PUB_BASE_URL`/`NUXT_CONTENT_API_KEY` | 访问 `/blog` | BFF 返回 500「内容中台未配置」提示，而非静默空白 |
| TC-13-094 | 真实访客 IP/UA 转发 | 集成 | P2 | 经 nginx | 提交反馈/取 summary | `clientForwardHeaders` 取 `x-forwarded-for` 首段（回退 x-real-ip/连接 IP）+ UA 转发中台，后台明细记真实访客而非 www 服务器 |
| TC-13-095 | SEO 辅助文件回源 | 接口 | P2 | 中台配置 web-files | `curl /sitemap.xml /robots.txt /llm.txt /faq.md` | 经 `proxyWebFile` 回源全局/语言文件，透传 Content-Type；`?lang=` 指定语言，缺省走中台回退链 |
| TC-13-096 | 同源 Cookie 前提 | 集成 | P0 | nginx 统一域名 | 经统一域名访问 www 与 my | www 与后端同源，HttpOnly `user_token` 在 SSR 请求中随行，SSR 登录态（§五）方成立；跨域部署需另行验证 CORS/Cookie |

## 关联 / 备注

- **环境**：www dev `:3000`（`NUXT_DEV_PORT` 可覆盖）；SSR 登录态与同源 Cookie 相关用例（TC-13-060~064、096）必须在 **nginx 统一域名**下验证，独立起 dev 端口无法复现 HttpOnly Cookie 透传。
- **数据依赖**：博客/帮助/FAQ/API 文档内容来自**内容中台 Pub API**（需配置 `NUXT_PUB_BASE_URL` + `NUXT_CONTENT_API_KEY`，space 见 `contentSpace`=blog / faq / help）；定价与试用来自**后端公开营销接口**（`NUXT_BACKEND_PUBLIC_URL`）。缺内容中台配置时内容页 500 提示；缺后端时定价走 `DEFAULT_PRICING` 兜底。
- **与旧文档关系**：本文档取代 TC-08/TC-09 中 www 相关的原型冒烟条目，升级为完整验收；跨 SPA 链接（www→my）与导航登录态联动的历史用例参见 TC-08-030/040~042、TC-01 §五。
- **SSR/首屏一致性测法**：优先用 `curl -s` 直取 HTML 断言首屏正文/登录态/主题属性（`data-accent`），再辅以浏览器观察 hydrate 无闪烁。
- **自动化覆盖**：本域**无 Plan A（Go/后端）自动化交叉**。现有 www 单测仅覆盖纯函数格式（`www/tests/{blog-format,help-format,feedback-format,seo-format}.test.ts`，对应 `formatBlogDate`/`readingMinutes`/`buildToc`/`docTrail`/`buildVisitorId`/`buildMeta`/`nextVote`/`seoMetasToHead` 等），**不覆盖页面渲染/SSR/交互**。页面级用例归 **Plan C（Playwright）**，本文档用例暂以人工执行为主，后续接入 Plan C 自动化。
