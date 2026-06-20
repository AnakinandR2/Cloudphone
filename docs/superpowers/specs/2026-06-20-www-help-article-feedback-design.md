# www 帮助文档接入内容中台「读者反馈」（赞踩 + 留言） — 设计稿

- 日期：2026-06-20
- 范围：在 www 帮助文档文章页（`/help/<slug>`）正文末尾接入内容中台的读者反馈能力，形态为 **赞踩（vote）+ 文本留言（content + 联系方式 contact）**。
- 复用：沿用已落地的 `/_content/*` 服务端代理（BFF）+ 组合式 + 纯函数单测模式，见 [2026-06-18-www-help-center-content-platform-design.md](2026-06-18-www-help-center-content-platform-design.md)。
- 不在范围内：博客赞踩、星级评分（rating）、后台分析页（后台在中台侧）。组件预留 `rating` 维度但本次不启用。

## 1. 内容中台反馈 API（已对实例实测）

鉴权/信封同其它端点（`X-API-Key`、`{code,message,data}`、站点 `zh`→中台 `zh-CN`）。本次用的 Key 已具备 `feedback:write` scope（实测写入成功）。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/pub/articles/{slug}/feedback-summary?space=&lang=&visitor_id=` | 聚合 + 我的反馈 |
| POST | `/pub/articles/{slug}/reaction?space=` | 提交赞踩或评分 |
| POST | `/pub/articles/{slug}/feedback?space=` | 提交文本留言 |

**实测得到的字段契约（文档未尽，逐字段探明）：**

- `visitor_id`：string，**必填**。缺失直接 `400 请求参数错误`（此版本未放开「无 visitor_id 时用 IP+UA 兜底」，故前台必须始终带）。
- `lang`：string，可选。
- `meta`：**string**，可选 —— 文档所称「业务自定义标识 meta」。注意是**字符串**（传对象/数字均 `400`），要带结构化身份须 `JSON.stringify`。
- `vote`：int，取值 **`-1`（踩）/ `0`（无/撤销）/ `1`（赞）**（越界 `422 vote 取值 -1 / 0 / 1`）。
- `rating`：int 1–5（reaction，另一维度，本次不用）。reaction 须 `rating`/`vote` 至少其一。
- `content`：string，**必填**（feedback，空/缺 `400`）。
- `contact`：string，可选（feedback）。

**summary 返回**：`{ rating_avg, rating_count, rating_dist[5], up_count, down_count, feedback_count }`；带 `visitor_id` 时附 `my_reaction: { rating, vote }`。

**关键便利**：`reaction`/`feedback` 两个写接口的**响应直接回带最新 summary**（含 `my_reaction`），所以提交后无需再单独拉一次。

## 2. 产品决策（已与用户确认）

1. 仅接入**帮助文档**（`space=help`）；形态为**赞踩 + 留言**，放在**文章正文末尾**。
2. **联系方式输入框不预填**——即使已登录也留空，避免让用户误以为我们未经同意获取其手机号。
3. 登录用户的 `user_id / nickname / phone` 经 **`meta`（JSON 字符串）** 传给中台，仅用于后台明细识别；前台不展示、不回填。
4. 按文档要求，业务端（BFF）把**真实访客 IP / User-Agent** 注入回源请求。

## 3. 软标识与 meta 策略

- **未登录**：`visitor_id` = 客户端生成并持久化于 `localStorage`（键 `gp_visitor_id`）的 UUID；`meta` 不带。
- **已登录**：`visitor_id = "u:<用户id>"`（跨设备稳定、同一人去重——文档明确「业务自传 visitor_id 最稳定」）；`meta = JSON.stringify({ user_id, nickname, phone })`。
- 赞踩按 `(article, visitor)` 唯一「最后一次生效」；留言每次留痕。匿名→登录视为不同访客（可接受）。

## 4. 架构

沿用 **Nuxt 服务端代理**：新增 `/_content/feedback/[slug]/{summary,reaction,feedback}` 路由，密钥仅服务端持有；浏览器只调用本站接口（同源、无 CORS、密钥不外泄）。

**真实 IP/UA 注入**：BFF 读取进入请求的 `x-forwarded-for`（nginx 注入的真实客户端链）与 `user-agent`，作为 `X-Forwarded-For` / `X-Real-IP` / `User-Agent` 头转发给中台，使后台明细记录到真实访客而非 www 服务器。

### 4.1 服务端（BFF）

- [server/utils/content.ts](../../../www/server/utils/content.ts) 新增：
  - `clientForwardHeaders(event)`：从 `event` 提取真实客户端 IP（优先 `x-forwarded-for` 首段，回退 `x-real-ip` / 连接地址）与 `user-agent`，产出转发头。
  - `contentPost<T>(event, path, query, body)`：`POST` 回源，注入 `X-API-Key` + 上述转发头，解封信封；上游 4xx 透传（403→403、422→422、其余→502）。
- `server/routes/_content/feedback/[slug]/summary.get.ts` → `GET …/feedback-summary`（query 透传 `space/lang/visitor_id`，同样转发 IP/UA）。
- `server/routes/_content/feedback/[slug]/reaction.post.ts` → `POST …/reaction`，body 取 `{ vote, rating?, visitor_id, lang, meta }`。
- `server/routes/_content/feedback/[slug]/feedback.post.ts` → `POST …/feedback`，body 取 `{ content, contact, visitor_id, lang, meta }`。
- `space` 由组件经 query 传入（本次固定 `help`），便于将来博客复用。

### 4.2 组合式与纯函数

- [composables/useFeedback.ts](../../../www/composables/useFeedback.ts)：
  - 纯函数（可单测）：
    - `buildVisitorId(user, stored)`：登录→`u:<id>`；否则→已存 UUID 或新生成（返回 `{ id, persist }`，persist 表示是否需写回 localStorage）。
    - `buildMeta(user)`：登录→`JSON.stringify({ user_id, nickname, phone })`；否则 `undefined`。
    - `nextVote(current, clicked)`：赞踩切换 reducer——再次点击同向→`0`（撤销），否则→点击值。
  - 调用封装：`fetchSummary / submitReaction / submitFeedback`（命中本站 `/_content/feedback/*`）。

### 4.3 组件与接入

- [components/ArticleFeedback.client.vue](../../../www/components/ArticleFeedback.client.vue)：客户端岛屿（依赖 `localStorage` 与登录态，纯客户端渲染以规避水合不匹配）。
  - props：`space`、`slug`、布尔 `vote` / `feedback`（预留 `rating`）。
  - 挂载即拉 summary（带 visitor_id）→ 显示赞踩数量、高亮我的选择。
  - 赞踩：👍/👎，乐观更新 + 用写接口返回的 summary 回填；再点同向撤销（`vote=0`）。
  - 留言：正文 `textarea`（必填，前端非空校验）+ 联系方式输入框（**不预填**，可选）+ 提交；成功显示「感谢反馈」，失败提示并保留输入。
- [pages/help/[slug].vue](../../../www/pages/help/[slug].vue)：`<ArticleBody>` 之后加 `<ArticleFeedback space="help" :slug="slug" vote feedback />`。
- [composables/useGp.ts](../../../www/composables/useGp.ts)：新增 `feedback` 中英文案。
- 复用现成 [composables/useAuthUser.ts](../../../www/composables/useAuthUser.ts)（`id/nickname/phone`，SSR 填充已 hydrate）取登录态。

## 5. 错误处理

- BFF：未配置密钥→500；上游 403（scope 不足）/422（校验）透传，其余→502。
- 组件：提交失败→错误提示并回滚乐观态、保留留言输入；留言为空→前端拦截不发请求。
- summary 拉取失败→反馈区静默降级为「可提交但不显示历史计数」，不影响阅读。

## 6. 测试

- 纯函数单测（`tests/feedback-format.test.ts`，node:test）：`buildVisitorId`（登录/匿名/已存）、`buildMeta`、`nextVote` 切换矩阵。
- 构建后启动产物 `curl` 实测三个 BFF 端点（summary / reaction / feedback）走通整条代理链。

## 7. 安全与隐私

- API Key 仅服务端 BFF 持有，浏览器永不接触。
- 联系方式不预填、不展示；手机号仅经 `meta` 上送中台后台，属业务内部识别用途。
- 真实访客 IP/UA 仅用于中台明细，不在前台暴露。
