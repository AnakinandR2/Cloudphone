# www 价格营销页（数据驱动）· 设计文档

- 日期：2026-06-22
- 范围：`backend/modules/billing`（新增公开接口 + 试用营销标记）+ `www/`（重建 `/pricing` 页 + 首页引流区块）
- 状态：已通过脑暴评审，待写实现计划
- 依赖基线：[2026-06-21-购买与费用重构-design.md](./2026-06-21-购买与费用重构-design.md)、[2026-06-22-席位赠送时长与订单历史明细-design.md](./2026-06-22-席位赠送时长与订单历史明细-design.md)（赠送配置 `GiftMinutesPerSeatMonth` 已落地）

## 0. 背景与目标

价格/试用/赠送等计费能力已成熟，但数据只在登录鉴权后（`/api/v1/billing/*`、`/api/v1/admin/billing/*`）可取。营销站 `www` 现有一个静态 `/pricing` 页，用的是过时的「按台月套餐」模型，与真实计费（钱包 + 实例席位 + 包月开机数 + 临时开机时长 + 数量/时长阶梯折扣 + 赠送 + 试用）不符。

**目标**：在 `www` **就地重建 `/pricing`** 为数据驱动的营销页，数字实时取自后端，营销文案用 www 双语 i18n，突出**赠送福利**与**折扣**；首页价格区块同步改为精简引流版。

**已确认决策**：

| 决策点 | 结论 |
|--------|------|
| 数据来源 | 后端新增**公开免鉴权接口** + www **SSR 代理**调用 |
| 路由 | 就地重建 `/pricing`（不新增 `/price`，不做重定向） |
| 动态范围 | 价格 + 折扣 + 赠送 + 试用 **全动态** |
| 试用展示 | 试用支持多条，新增「营销展示」标记**单选一条**；接口只返回这一条；无则不显示试用相关信息 |
| 首页价格区块 | 同步改为精简引流版（3 张资源卡 + 「查看完整价格」→ /pricing） |
| 文案 vs 数字 | 数字（单价/折扣/赠送/试用数量）来自后端；可见文案来自 `GP_CONTENT`（zh+en），用占位符插入数字 |

## 1. 后端：公开接口 + 试用营销标记（`backend/modules/billing`）

### 1.1 试用「营销展示」单选标记

- `TrialPolicy` 新增列 `MarketingFeatured bool`（[trial_model.go](../../../backend/modules/billing/internal/trial_model.go)，AutoMigrate 补列）。
- 新增服务方法 `SetMarketingFeatured(id int) error`：**事务内**先把所有策略的 `marketing_featured` 置 false，再把目标 `id` 置 true —— 保证全局至多一条。
- 取消标记：`SetMarketingFeatured(0)` 或对已选项再次操作置 false（admin UI 决定，见 §1.4）。

### 1.2 公开定价接口

`GET /api/open/v1/billing/pricing` —— 免鉴权，返回**裁剪过的营销 DTO**（非原始配置，不含内部字段如 `recycle_retention_days`、`billing_note` 等无营销价值项）：

```jsonc
{
  "kinds": {
    "seat":      { "unit_price_cents": 3000, "unit_label": "台", "duration_unit": "month",
                   "qty_tiers": [{ "min_quantity": 10, "discount_bps": 9000 }, ...],
                   "duration_options": [{ "value": 1, "discount_bps": 10000 }, ...] },
    "boot_slot": { "unit_price_cents": 2000, "unit_label": "个", "duration_unit": "day",
                   "qty_tiers": [...], "duration_options": [...] }
  },
  "runtime_pack": { "unit_price_cents_per_minute": 20, "min_minutes": 60,
                    "packs": [{ "minutes": 600, "discount_bps": 10000 }, ...],
                    "daily_cap_minutes": 200, "gift_minutes_per_seat_month": 200 },
  "recharge_presets_cents": [1000, 5000, 10000, 50000]
}
```

数据源 `PricingConfigService.Get()`。

### 1.3 公开试用概览接口

`GET /api/open/v1/billing/trial-overview` —— 免鉴权。返回**被标记 `marketing_featured` 且 `enabled=true`** 的那一条：

```jsonc
// 有营销展示策略：
{ "policy": { "name": "新用户试用",
              "items": [{ "subject": "seat", "quantity": 1, "expire_days": 7 },
                        { "subject": "runtime_minute", "quantity": 600, "expire_days": 0 }] } }
// 无（未标记 / 标记的那条被禁用）：
{ "policy": null }
```

> 不广告已禁用的试用：标记的策略若 `enabled=false`，按「无」处理返回 `null`。

### 1.4 admin：营销展示开关

- 新增 `POST /api/v1/admin/billing/trials/:id/feature`（鉴权 `billing:manage`），body 可带 `{ "featured": true|false }`；调用 `SetMarketingFeatured`。
- `TrialsView.vue`（[admin](../../../admin/src/views/billing/TrialsView.vue)）每条策略加「营销展示」控件，渲染为**单选**（开启某条即关闭其它，呼应后端单条保证）；策略列表返回值带上 `marketing_featured` 以回显。

### 1.5 公开路由注册

新建 `backend/modules/billing/internal/billing_public_api.go`，含 `OpenGetPricing` / `OpenGetTrialOverview` 两个 handler 与 DTO 组装；在 [module.go](../../../backend/modules/billing/internal/module.go) 用 `framework.RegisterRootRoutes` 把 `/api/open/v1/billing/{pricing,trial-overview}` 挂到引擎根（沿用 openapi 模块约定，避免 `/api/v1` 前缀叠加）。两条均**不挂任何鉴权中间件**。

### 1.6 后端测试

- `OpenGetPricing`：无 token 返回 200，含 `kinds.seat`、`runtime_pack.gift_minutes_per_seat_month`。
- `OpenGetTrialOverview`：未标记时 `policy=null`；标记 A 后返回 A；标记 B 后只返回 B（验证单选）；标记的策略禁用后返回 `null`。
- `SetMarketingFeatured`：标记互斥（DB 中至多一条为 true）。

## 2. www 数据流（Nuxt SSR）

### 2.1 运行时配置

`nuxt.config.ts` `runtimeConfig` 新增 `backendPublicUrl`（默认 `http://localhost:9981/api/open/v1`，env 覆盖）。SSR-only，不进 `public`。

### 2.2 SSR server 路由（同源代理）

- `server/routes/_api/pricing.get.ts`：`$fetch(`${backendPublicUrl}/billing/pricing`)`，解包 `{code,message,data}` 返回 `data`。
- `server/routes/_api/trial-overview.get.ts`：同上调 `/billing/trial-overview`。
- 失败时返回 `null`（不抛 500），由页面兜底。镜像现有 `/_content/*` 的服务端取数模式，浏览器只打同源 `/_api/*`。

### 2.3 页面取数与兜底

- `pages/pricing.vue` 与首页（`PricingSection` 取数）用 `useAsyncData('pricing', () => $fetch('/_api/pricing'))` SSR 取数。
- **兜底**：`/_api/pricing` 返回 `null`（后端不可用）时，页面用一份与后端 seed 一致的**默认数字常量**（放 `www/composables/usePricing.ts`）渲染，页面永不崩。
- 试用：`/_api/trial-overview` 的 `policy` 为 `null` → 隐藏全部试用相关 UI（Hero 试用徽章 + 试用区块），不留空位。

### 2.4 取数封装

新增 `www/composables/usePricing.ts`：
- 拉取 + 兜底逻辑（供 `/pricing` 与首页复用）。
- 纯展示计算 helper：`yuan(cents)`、`discountZhe(bps)`（10000→无折扣不显示，8500→「8.5折」）、`giftExample(perSeatMonth)`（赠送示例文案数字）。
- 资源卡「低至」单价（命中最大数量阶梯 × 最长时长折扣后的折合单价）。

## 3. 页面区块（套用 www 设计系统：`.section`/`.container`/`.eyebrow`/`.price-card`/accent/`.reveal`）

`pages/pricing.vue` 组合以下 section（新组件放 `www/components/sections/`）：

1. **PricingHeroSection** —— 价值主张 + 两个数据驱动营销徽章：「新用户免费试用」（仅 `policy` 存在时）与「买席位送时长」；主 CTA → 控制台（`public.myAppPath`）。
2. **ResourcePricingSection** —— 3 张 `.price-card` 对应真实模型：**实例席位**（低至 ¥X/台/月）、**包月开机数**（¥X/个/天）、**临时开机时长**（¥X/分钟，含时长包）。每卡列各自卖点（文案来自 GP_CONTENT）。
3. **DiscountSection** —— 「买多买久，越用越省」：把 `qty_tiers` 与 `duration_options` 渲染成两组折扣徽章/小表（如「满 100 台 8 折」「包年 7 折」），折扣经 `discountZhe(bps)` 展示为「X 折」。
4. **GiftBannerSection** —— 整宽 accent 横幅：「每购买 1 个实例席位，每月赠送 **{gift}** 分钟临时开机时长」+ 实例化示例（「续费 2 台 × 3 个月 = 送 {gift×6} 分钟」），数字来自 `gift_minutes_per_seat_month`；值为 0 时隐藏此区块。
5. **TrialSection** —— 「新用户免费试用」卡片，列出 `trial-overview` 的 items（如「1 台 ×7 天 + 600 分钟」），CTA → 注册；`policy=null` 时整段不渲染。
6. **复用 FaqSection + CtaSection** —— FAQ 用价格相关条目（GP_CONTENT 新增）。

### 3.1 首页引流区块（同步改）

`components/sections/PricingSection.vue` 重写为精简版：复用 `usePricing` 取同一份数据，渲染 3 张资源卡（与 §3.2 同卡片，但精简卖点）+「查看完整价格」按钮 → `/pricing`。移除旧 `GP_CONTENT.pricing.plans`（按台月套餐）模型。首页 `index.vue` 无需改结构。

## 4. i18n 文案（`www/composables/useGp.ts` 的 `GP_CONTENT`）

重写 `pricing` 区块（zh + en，结构对齐），仅含**营销文案**与带占位符的模板，**不含价格数字**：
- `eyebrow/title/sub`、Hero 徽章文案、3 类资源的 `label/desc/卖点数组`、折扣区标题与「满{qty}{unit} {zhe}折」模板、赠送横幅模板（`{gift}`/`{example}`）、试用区文案（`{items}`）、价格 FAQ 条目、CTA。
- 货币符号固定 `￥`。

## 5. 错误处理与边界

- 后端不可用：`/_api/*` 返回 `null` → 页面用默认数字常量渲染（§2.3），不报错。
- `gift_minutes_per_seat_month=0`：隐藏赠送横幅。
- 试用未标记/被禁用：隐藏全部试用 UI。
- 折扣 `discount_bps=10000`：不显示折扣徽章（无折扣不喧宾夺主）。
- 暗色/亮色、8 种 accent：套用现有 token，自动适配。

## 6. 测试

- 后端：§1.6 的 handler/service 单测。
- www：`pnpm build`（`vue-tsc` 通过）；`usePricing` 的纯函数 helper（`yuan`/`discountZhe`/`giftExample`）加轻量单测（若 www 已有测试设施则补，否则仅靠 build + 手动验证两语言/明暗）。

## 7. 非目标（YAGNI）

- 营销页内实时报价/下单/结账（CTA 一律跳控制台/注册）。
- 支付方式展示、充值预设 UI（接口返回 `recharge_presets_cents` 暂不在营销页用，预留）。
- 试用领取/资格/邀请码逻辑（营销页只读展示）。
- 货币换算/多币种（仅 ￥）。

## 8. 影响面清单

| 层 | 文件 | 改动 |
|----|------|------|
| 后端·模型 | `trial_model.go` | `TrialPolicy.MarketingFeatured` 列 |
| 后端·服务 | `trial_service.go` | `SetMarketingFeatured` 单选事务 |
| 后端·公开 API | `billing_public_api.go`（新） | `OpenGetPricing` / `OpenGetTrialOverview` + DTO |
| 后端·admin API | `trial_api.go` | `POST /admin/billing/trials/:id/feature`；列表回显 `marketing_featured` |
| 后端·路由 | `module.go` | `RegisterRootRoutes` 挂 `/api/open/v1/billing/*` |
| 后端·测试 | `*_test.go` | 公开接口 + 单选标记 |
| admin·前端 | `TrialsView.vue` + locales | 「营销展示」单选控件 |
| www·配置 | `nuxt.config.ts` | `backendPublicUrl` |
| www·SSR | `server/routes/_api/{pricing,trial-overview}.get.ts`（新） | 代理后端公开接口 |
| www·取数 | `composables/usePricing.ts`（新） | 拉取 + 兜底 + 展示 helper |
| www·页面 | `pages/pricing.vue` | 重建为数据驱动营销页 |
| www·区块 | `components/sections/`（Hero/Resource/Discount/Gift/Trial 新 + Faq/Cta 复用） | 营销区块 |
| www·首页 | `components/sections/PricingSection.vue` | 改精简引流版 |
| www·文案 | `composables/useGp.ts`（GP_CONTENT zh+en） | 重写 `pricing` 区块 |
