# 代理IP合作商管理 + my 端代理IP推荐 — 设计稿

日期：2026-06-20
状态：已确认，待写实现计划

## 背景与目标

运营需要在 admin 后台管理一批「代理IP合作商」（名称、logo、介绍文案、配图、推广链接、排序、启用开关），并在 my 端「代理管理」里新增「代理IP推荐」tab 展示这些合作商。用户点击推广链接跳转到合作商站点，我们通过推广链接获得收益，并记录点击明细用于后续分析。

收益归因方式：**外链跳转 + 点击计数**（点击明细表 + 合作商总计缓存）。不做精确分成/转化回传（非目标）。

## 已确认的关键决策

- **架构**：独立后端模块 `partner`（不塞进现有 `proxy` 模块），沿用现有模块范式。
- **图片**：admin 上传到 S3（复用 `framework.S3`），存返回的公开 URL。
- **内容模型**：单配图 + 纯文本介绍。
- **点击数据**：点击明细表（含用户/IP/软标识）+ 合作商表 `click_count` 总计缓存。
- **匿名点击**：list / click 接口**公开**，支持未登录调用。营销站 www 的游客（不登录跳转推广）也走同一套接口，采集软标识。
- **本次范围**：后端接口设计成公开 + 匿名 + 软标识；前端**只先上 my 的推荐 tab**。www 营销站展示页以后接同一套接口再做（非本次范围）。
- **admin 菜单**：单独新建顶层分组「合作商/推广」，其下放「合作商管理」。

## 非目标（YAGNI）

- 不做点击去重 / 防刷（原始点击全量入明细表；以后要趋势/去重从明细表聚合即可）。
- 不做精确转化回传 / 佣金分成。
- 不做富文本介绍、多配图画廊。
- 本次不做 www 营销站的展示页（仅保证接口已就绪）。

## 数据模型

### `partners`（合作商）

| 字段 | 类型 | 说明 |
|---|---|---|
| id | uint PK | |
| name | string | 名称，必填 |
| logo_url | string | logo 图片 URL（S3） |
| image_url | string | 配图 URL（S3，单张） |
| intro | text | 纯文本介绍 |
| promo_url | string | 推广链接，必填 |
| sort | int | 排序权重，小在前，默认 0 |
| enabled | bool | 是否启用，默认 true |
| click_count | int | 点击总数缓存，默认 0 |
| created_at | time | |
| updated_at | time | |

### `partner_clicks`（点击明细）

| 字段 | 类型 | 说明 |
|---|---|---|
| id | uint PK | |
| partner_id | uint | 关联合作商，索引 |
| user_id | uint (nullable) | 登录用户 id；匿名为空 |
| ip | string | 服务端从请求取（含反代头处理） |
| user_agent | string | 服务端从请求头取 |
| referer | string | 服务端从请求头取 |
| anonymous_id | string | 前端持久化的访客 uuid（localStorage） |
| session_id | string | 前端会话标识（可空） |
| utm_source | string | |
| utm_medium | string | |
| utm_campaign | string | |
| utm_term | string | |
| utm_content | string | |
| channel | string | 来源渠道：`my` / `www` / 具体落地页标识 |
| extra | JSON | 兜底扩展字段，未来软标识无需改表 |
| created_at | time | 索引（按时间聚合用） |

软标识缺失（UA/Referer/匿名 id 等）一律存空串，不报错。

## 后端：`backend/modules/partner`

目录结构沿用现有模块范式：`internal/{module,api,service,repository,model}.go`，模块在 `init()` 自注册到 `framework.ModuleRegistry`，并在 `backend/main.go` 加空导入 `_ "manager-backend/modules/partner"`。

### 路由

**admin 侧（`/admin/partners`，staff 鉴权）**
- `GET /admin/partners` — 列表（全部，含禁用），按 sort、id 排序。权限 `partner:view`。
- `POST /admin/partners` — 创建。权限 `partner:manage`。
- `PUT /admin/partners/:id` — 更新。权限 `partner:manage`。
- `DELETE /admin/partners/:id` — 删除。权限 `partner:manage`。
- `POST /admin/partners/upload` — 上传图片（logo/配图）到 S3，返回 `{ url }`。权限 `partner:manage`。multipart，不手动设 Content-Type。
- `GET /admin/partners/:id/clicks` — 点击明细分页（给以后做趋势/明细查看）。权限 `partner:view`。

**公开侧（`/partner`，无强制鉴权）**
- `GET /partner/list` — 仅返回 `enabled=true`，按 sort、id 排序。返回展示所需字段（name/logo_url/image_url/intro/promo_url/id）。
- `POST /partner/:id/click` — 记录点击：写一行 `partner_clicks` + 原子自增 `partners.click_count`。
  - 服务端采集：ip（处理反代头）、user_agent、referer。
  - 可选鉴权：若带有效用户 token，软取 user_id；无则留空（不拦截）。
  - body 收：`anonymous_id`、`session_id`、`utm_*`、`channel`、`extra`。
  - partner 不存在或已禁用：忽略写入但返回成功语义，不阻断前端跳转（前端拿到响应即开链）。

> 鉴权实现：公开路由组不挂 `user.AuthMiddleware()`（强制登录）；click 若要软取 user_id，用「可选鉴权」——能解析出登录态就带上，否则匿名。具体中间件在实现计划里确定（沿用 user 模块现有能力）。

### S3 上传

复用 `framework.S3`（未配置时为 nil）。upload handler：
- `framework.S3 == nil` → 返回「未配置 S3」错误，admin 表单提示。
- 校验文件类型为图片、大小 ≤ 5MB，否则拒绝。
- key 规则：如 `partners/{uuid}.{ext}`；返回 `S3.PublicURL(key)`。

### 并发

`click_count` 自增用 `UPDATE partners SET click_count = click_count + 1 WHERE id = ?`（原子），不用「读-改-写」。

## admin 前端

- `src/views/partner/PartnerView.vue`：DataTable（列：logo 缩略图、名称、推广链接、排序、启用徽章、点击数、操作）。删除用就近 Popconfirm（destructive）。沿用 DataTable 约定（pageSize ∈ [20,50,100,200]、骨架屏、搜索同步 URL）。
- `src/views/partner/PartnerFormDialog.vue`：创建/编辑表单。字段：名称、推广链接、介绍（textarea）、排序、启用开关（Switch v-model）、logo 上传、配图上传。上传交互参考现有 app 上传。
- `src/api/modules/partner.ts`、`src/types/partner.ts`。
- locales：`zh-CN.ts` + `en.ts` 同步新增 `menu.partnerGroup`（合作商/推广）、`menu.partners`（合作商管理）、表单/列文案。
- 路由：`src/router/routes.ts` 顶层新增分组 `{ meta:{ title:'menu.partnerGroup', icon:'Handshake' } }`，子项 `/partners`（name `partners`，`icon:'Handshake'`，`meta.auth:'partner:view'`）。
- 图标：`Icon.vue` 同步加 import + registry。
- 权限键：`partner:view`、`partner:manage`（按 admin RBAC 约定，需在角色/权限数据里登记；mock 侧补齐）。

## my 前端

- 重构 `src/views/proxy/ProxyView.vue` 为 Tabs：
  - Tab 1「我的代理」：现有表格/逻辑原样迁入（行为不变）。
  - Tab 2「代理IP推荐」：新增 `src/views/proxy/PartnerRecommendPanel.vue`。
- `PartnerRecommendPanel.vue`：调 `GET /partner/list` 渲染卡片列表（logo + 名称 + 配图 + 介绍 + CTA 按钮）。点 CTA：先 `POST /partner/:id/click`（带 `anonymous_id`、`channel:'my'`，可带 user 登录态），再 `window.open(promo_url, '_blank')`。click 失败也不挡跳转（best-effort）。
- `src/api/modules/partner.ts`、`src/types/partner.ts`。
- 工具：`anonymous_id` 持久化在 localStorage（无则生成 uuid，存储前缀沿用 my 约定）。
- locales：`zh-CN.ts` + `en.ts` 同步新增 tab 标题、卡片/CTA 文案。
- Tabs 组件用 shadcn-vue 现有 `Tabs`（参考 RemoteAppPanel 等已有用法）。

## 错误处理 / 边界

- click 接口对脏数据宽容：partner 不存在/已禁用不报错给用户，不阻断跳转。
- 软标识缺失存空串；`extra` 兜未知字段。
- list 仅 enabled，按 sort、id 兜底排序，保证顺序稳定。
- 上传：非图片 / 超限 / S3 未配置 → 明确错误文案。
- my 端 click 为 best-effort，网络失败仍执行 `window.open`。

## 测试

- **后端 service 层单测**：创建/更新/删除；list 仅返回 enabled 且按 sort 排序；click 落明细表 + click_count 原子自增；匿名 click（user_id 为空）与登录 click（带 user_id）两种路径；脏 partner_id 不报错。
- **后端 repository**：过滤 + 排序。
- **上传**：S3 未配置返回错误；类型/大小校验（沿用 app upload 测试范式）。
- **前端**：mock 文件补 partner 接口；my tab 卡片渲染与「click→open」顺序手测；admin CRUD + 上传手测。

## 文件清单（预估）

**新增**
- `backend/modules/partner/internal/{module,api,service,repository,model}.go`（+ service 单测）
- `admin/src/views/partner/{PartnerView,PartnerFormDialog}.vue`
- `admin/src/api/modules/partner.ts`、`admin/src/types/partner.ts`
- `my/src/views/proxy/PartnerRecommendPanel.vue`
- `my/src/api/modules/partner.ts`、`my/src/types/partner.ts`
- my 端 `anonymous_id` 工具（如 `my/src/utils/visitor.ts`）

**修改**
- `backend/main.go`（空导入）
- `admin/src/router/routes.ts`、`admin/src/components/Icon.vue`、`admin/src/locales/{zh-CN,en}.ts`、RBAC 权限/角色数据 + mock
- `my/src/views/proxy/ProxyView.vue`（改 Tabs）、`my/src/locales/{zh-CN,en}.ts`
