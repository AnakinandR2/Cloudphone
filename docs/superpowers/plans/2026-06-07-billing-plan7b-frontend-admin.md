# 计费系统 Phase 1 · 计划 7b：admin（运营）计费前端接真 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 admin 运营后台的计费原型页（定价/折扣/订单）改为对接真实 `/admin/billing/*`，并补齐账户/资源调整、试用配置，配 RBAC（`billing:view`/`billing:manage`）。

**Architecture:** 复用 admin 既有约定（Vue3 + shadcn-vue + `@/api` `api.get/post<unknown,R<T>>`、`@/types/*`、`@/mock/*` `defineFakeRoute`、`DataTable`(含 expandable/#expanded、`#cell-*`、`#filters`)、`v-auth` 指令 + 路由 `meta.auth` RBAC、vue-i18n 双语、`pnpm build`=vue-tsc 必过）。后端 JSON snake_case、金额整数分。新增 `billing` API+类型+mock+money 工具；视图保留原型骨架换真数据；写操作 `v-auth='billing:manage'`，列表路由 `meta.auth:'billing:view'`。无后端改动。

**Tech Stack:** Vue3 / TS / Vite / shadcn-vue / vue-i18n / pnpm。分支 `feature/billing-frontend-admin`。

> 后端契约（已就绪，admin 组 staff 鉴权 + 权限）：`GET /admin/billing/skus`、`POST /admin/billing/skus`、`PUT /admin/billing/skus/:id`、`DELETE /admin/billing/skus/:id`、`GET /admin/billing/skus/:id/tiers`、`POST /admin/billing/skus/:id/tiers`、`PUT /admin/billing/tiers/:tierId`、`DELETE /admin/billing/tiers/:tierId`；`GET /admin/billing/orders?page&size&userId&status`、`POST /admin/billing/orders/:id/mark-paid`；`GET /admin/billing/accounts/:userId`(→{account,ledger,ledger_total,capacities})、`POST /admin/billing/accounts/:userId/adjust{delta_cents,reason}`、`POST /admin/billing/accounts/:userId/adjust-resource{subject,delta,reason}`；`GET /admin/billing/trials`、`POST /admin/billing/trials`、`PUT /admin/billing/trials/:id`、`DELETE /admin/billing/trials/:id`、`POST /admin/billing/trials/:id/eligibility{user_id}`、`GET /admin/billing/trials/:id/grants`。`{code,message,data}`，列表 `{list,total}`。

---

## 计划 7b 文件结构

| 文件 | 动作 |
|---|---|
| `admin/src/types/billing.ts` | 创建（DTO，snake_case，可复用 my 同款定义） |
| `admin/src/api/modules/billing.ts` | 创建 |
| `admin/src/mock/billing.ts` | 创建 |
| `admin/src/utils/money.ts` + `money.test.ts` | 创建（fmtCents/fmtDiscountBps，同 my） |
| `admin/src/views/billing/PricingView.vue` | 重写（SKU CRUD + 行展开管理阶梯） |
| `admin/src/views/billing/DiscountsView.vue` | 重写（折扣阶梯总览/管理，优惠码下线） |
| `admin/src/views/billing/OrdersView.vue` | 重写（真订单 + 标记已付 + 详情展开） |
| `admin/src/views/billing/AccountsView.vue` | 创建（查用户账户/流水/容量 + 余额&资源调整） |
| `admin/src/views/billing/TrialsView.vue` | 创建（试用策略 CRUD + 授予资格 + 发放记录） |
| `admin/src/router/routes.ts` | 增 accounts/trials 路由 + 全部 billing 路由加 `meta.auth:'billing:view'` |
| `admin/src/components/Icon.vue` | 注册新图标 |
| `admin/src/locales/{zh-CN,en}.ts` | 扩充 billing.* |

**约定速查**：`import api from '../index'`；`v-auth="'billing:manage'"` 控写操作按钮；路由 `meta.auth:'billing:view'`；`DataTable`(`:columns` computed、`#cell-<id>`、`#filters`、`expandable`+`#expanded`、`:loading`)；`Popconfirm` 删除二次确认；金额 `fmtCents`；i18n 双语；**提交前 `pnpm build` 必过**（独立 pnpm，见 admin/CLAUDE.md）。

---

## Task 1：admin billing API 客户端 + 类型 + mock + money 工具

**Files:** Create `types/billing.ts`, `api/modules/billing.ts`, `mock/billing.ts`, `utils/money.ts`, `utils/money.test.ts`。

- [ ] **Step 1：`utils/money.ts` + `money.test.ts`** — 与 my 同款 `fmtCents`/`fmtDiscountBps`（可直接复制 my 版本 + 单测）。Run vitest 确认通过。

- [ ] **Step 2：`types/billing.ts`** — 复用 my 的 DTO（snake_case）并补 admin 专用：
```ts
export interface Sku { id: number, code: string, category: string, name: string, description: string, unit_price_cents: number, unit: string, listed: boolean, sort: number }
export interface DiscountTier { id: number, sku_id: number, cycle_months: number, min_quantity: number, discount_bps: number }
export interface SkuCreate { code: string, category: string, name: string, description?: string, unit_price_cents: number, unit?: string, listed?: boolean, sort?: number }
export interface SkuUpdate { name?: string, description?: string, unit_price_cents?: number, unit?: string, listed?: boolean, sort?: number }
export interface TierCreate { cycle_months: number, min_quantity: number, discount_bps: number }
export interface TierUpdate { cycle_months?: number, min_quantity?: number, discount_bps?: number }

export interface Order { id: number, order_no: string, user_id: number, status: string, pay_method: string, total_cents: number, paid_at: string | null, created_at: string, updated_at: string }
export interface OrderItem { id: number, order_id: number, sku_code: string, sku_name: string, category: string, cycle_months: number, quantity: number, unit_price_cents: number, discount_bps: number, original_cents: number, payable_cents: number }
export interface OrderDetail { order: Order, items: OrderItem[] }

export interface LedgerEntry { id: number, user_id: number, subject: string, type: string, delta: number, balance_after: number, reason: string, order_id: number, operator: string, created_at: string }
export interface Account { id: number, user_id: number, balance_cents: number, created_at: string, updated_at: string }
export interface CapacitySnapshot { instance_seat: number, boot_seat: number, runtime_minute: number }
export interface AccountView { account: Account, ledger: LedgerEntry[], ledger_total: number, capacities: CapacitySnapshot }

export interface TrialPolicy { id: number, code: string, name: string, enabled: boolean, grant_subject: string, grant_quantity: number, grant_expire_days: number, per_user_limit: number, allow_new_user: boolean, invite_code: string, created_at: string, updated_at: string }
export interface TrialPolicyCreate { code: string, name: string, grant_subject: string, grant_quantity: number, grant_expire_days?: number, per_user_limit?: number, allow_new_user?: boolean, invite_code?: string, enabled?: boolean }
export interface TrialPolicyUpdate { name?: string, grant_quantity?: number, grant_expire_days?: number, per_user_limit?: number, allow_new_user?: boolean, invite_code?: string, enabled?: boolean }
export interface TrialGrant { id: number, policy_id: number, user_id: number, subject: string, quantity: number, created_at: string }
```

- [ ] **Step 3：`api/modules/billing.ts`**
```ts
import type {
  Account, AccountView, DiscountTier, Order, OrderDetail, Sku, SkuCreate, SkuUpdate,
  TierCreate, TierUpdate, TrialGrant, TrialPolicy, TrialPolicyCreate, TrialPolicyUpdate,
} from '@/types/billing'
import api from '../index'

interface R<T> { code: number, message: string, data: T }
interface Page<T> { list: T[], total: number }

export default {
  // 定价
  listSkus: () => api.get<unknown, R<Sku[]>>('admin/billing/skus'),
  createSku: (d: SkuCreate) => api.post<unknown, R<Sku>>('admin/billing/skus', d),
  updateSku: (id: number, d: SkuUpdate) => api.put<unknown, R<Sku>>(`admin/billing/skus/${id}`, d),
  deleteSku: (id: number) => api.delete<unknown, R<null>>(`admin/billing/skus/${id}`),
  listTiers: (skuId: number) => api.get<unknown, R<DiscountTier[]>>(`admin/billing/skus/${skuId}/tiers`),
  createTier: (skuId: number, d: TierCreate) => api.post<unknown, R<DiscountTier>>(`admin/billing/skus/${skuId}/tiers`, d),
  updateTier: (tierId: number, d: TierUpdate) => api.put<unknown, R<DiscountTier>>(`admin/billing/tiers/${tierId}`, d),
  deleteTier: (tierId: number) => api.delete<unknown, R<null>>(`admin/billing/tiers/${tierId}`),

  // 订单
  orders: (params: { page?: number, size?: number, userId?: number, status?: string }) =>
    api.get<unknown, R<Page<Order>>>('admin/billing/orders', { params }),
  markPaid: (id: number) => api.post<unknown, R<OrderDetail>>(`admin/billing/orders/${id}/mark-paid`),

  // 账户/资源调整
  account: (userId: number, params?: { page?: number, size?: number }) =>
    api.get<unknown, R<AccountView>>(`admin/billing/accounts/${userId}`, { params }),
  adjustBalance: (userId: number, delta_cents: number, reason: string) =>
    api.post<unknown, R<Account>>(`admin/billing/accounts/${userId}/adjust`, { delta_cents, reason }),
  adjustResource: (userId: number, subject: string, delta: number, reason: string) =>
    api.post<unknown, R<unknown>>(`admin/billing/accounts/${userId}/adjust-resource`, { subject, delta, reason }),

  // 试用
  listTrials: () => api.get<unknown, R<TrialPolicy[]>>('admin/billing/trials'),
  createTrial: (d: TrialPolicyCreate) => api.post<unknown, R<TrialPolicy>>('admin/billing/trials', d),
  updateTrial: (id: number, d: TrialPolicyUpdate) => api.put<unknown, R<TrialPolicy>>(`admin/billing/trials/${id}`, d),
  deleteTrial: (id: number) => api.delete<unknown, R<null>>(`admin/billing/trials/${id}`),
  grantEligibility: (id: number, user_id: number) => api.post<unknown, R<null>>(`admin/billing/trials/${id}/eligibility`, { user_id }),
  trialGrants: (id: number) => api.get<unknown, R<TrialGrant[]>>(`admin/billing/trials/${id}/grants`),
}
```

- [ ] **Step 4：`mock/billing.ts`** — `defineFakeRoute` 覆盖上述全部 `/v1/admin/billing/...`（参考 `admin/src/mock/cloudphone.ts` 的 `ok` 助手 + url 写 `/v1/admin/billing/...`）。内存：3 SKU（含 seed 默认价/单位/上架）、各 SKU 的阶梯、订单数组（含不同状态/支付方式）、某用户账户(余额+流水+容量)、两条试用策略 + 发放记录。CRUD 操作真实改内存数组并回写；`markPaid` 置 paid；`adjust*` 改账户；列表 `{list,total}`。仅 UI 联调。
  Run: `cd admin && pnpm build` 通过（类型自洽）。

- [ ] **Step 5：质量门 + 提交**
```bash
cd /home/root/workspace005/gloryphone-code/admin && ./node_modules/.bin/vitest run src/utils/money.test.ts && pnpm build
cd /home/root/workspace005/gloryphone-code
git add admin/src/types/billing.ts admin/src/api/modules/billing.ts admin/src/mock/billing.ts admin/src/utils/money.ts admin/src/utils/money.test.ts
git commit -m "feat(admin): billing API 客户端+类型+mock+money 工具

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 2：PricingView（SKU CRUD）+ DiscountsView（折扣阶梯 CRUD）

**Files:** Modify `views/billing/PricingView.vue`, `views/billing/DiscountsView.vue`; `locales/*`。

- [ ] **Step 1：PricingView 接真** — 读现有原型（本地 SKU CRUD）。改为 `billingApi.listSkus()` 驱动 DataTable；列：code/category(badge)/name/unit_price(fmtCents+元)/unit/listed(badge)/sort/操作。新增/编辑用 Dialog 调 `createSku/updateSku`（category 在新建时选 instance_fee/boot_pack/time_pack，编辑时只读；单价以元输入→分；listed 开关）；删除 `Popconfirm`+`deleteSku`。写操作按钮 `v-auth="'billing:manage'"`。金额 `fmtCents`。加载 `:loading`、`toast`。
- [ ] **Step 2：DiscountsView 接真（折扣阶梯）** — **去掉优惠码部分**（Non-goal）。改为「按 SKU 管理折扣阶梯」：先选/列出 SKU，展开或选中后 `billingApi.listTiers(skuId)` 列出该 SKU 阶梯（cycle_months/min_quantity/discount_bps(显示 fmtDiscountBps)），增/改/删调 `createTier/updateTier/deleteTier`；bps 用「折」输入（如 8.5 折→8500）或直接 bps 输入（标注）。写操作 `v-auth`。或：DataTable 列全部 SKU×阶梯，行内编辑。实现者择一清晰方案，保证可增删改阶梯。
- [ ] **Step 3：i18n + `pnpm build` + 提交** `feat(admin): 定价SKU CRUD + 折扣阶梯管理(优惠码下线)`。

---

## Task 3：OrdersView 接真 + 账户/资源调整页

**Files:** Modify `views/billing/OrdersView.vue`; Create `views/billing/AccountsView.vue`; `router/routes.ts`; `locales/*`; `Icon.vue`。

- [ ] **Step 1：OrdersView 接真** — `billingApi.orders({page,size,userId,status})` 驱动 DataTable；列 order_no/user_id/pay_method/total(fmtCents)/status(badge)/created_at；筛选 userId(输入)+status；行展开 `#expanded` 显示订单项（如需详情可由 list 项已含或加详情接口——本期 list 仅 Order，行展开展示基本信息或略）。**标记已付**：对 pending 单行内按钮 `v-auth="'billing:manage'"` → `Popconfirm` → `billingApi.markPaid(id)` → 刷新（网关支付桩：运营手动确认到账）。去掉静态数据。
- [ ] **Step 2：AccountsView（账户/资源管理）新页** — 顶部输入 userId → `billingApi.account(userId)` 载入：账户余额(fmtCents)、三类资源容量、流水表(ledger，单位感知同 my LedgerView)。两个调整操作（`v-auth="'billing:manage'"`）：
  - 余额调整：Dialog 输入金额(元，正赠负扣)+理由 → `adjustBalance(userId, cents, reason)`（退款入口）。
  - 资源调整：Dialog 选科目(instance_seat/boot_seat/runtime_minute)+数量(正负)+理由 → `adjustResource(userId, subject, delta, reason)`。
  调整后刷新账户视图 + toast。
- [ ] **Step 3：router** 增 `/billing/accounts`（name `billingAccounts`，`menu.billingAccounts`，icon 如 `Wallet`/`UserCog`，`meta.auth:'billing:view'`）。Icon 注册。
- [ ] **Step 4：i18n + `pnpm build` + 提交** `feat(admin): 订单接真+标记已付 + 账户/资源调整页`。

---

## Task 4：试用配置页 + 路由/导航/RBAC/i18n 收尾

**Files:** Create `views/billing/TrialsView.vue`; `router/routes.ts`（trials 路由 + 全部 billing 路由补 `meta.auth`）; `Icon.vue`; `locales/*`。

- [ ] **Step 1：TrialsView（试用配置）新页** — `billingApi.listTrials()` DataTable：code/name/enabled/grant(subject+quantity+expire_days)/per_user_limit/allow_new_user/invite_code/操作。
  - 新建/编辑 Dialog（`createTrial/updateTrial`）：code、name、grant_subject(选 instance_seat/boot_seat/runtime_minute)、grant_quantity、grant_expire_days、per_user_limit、allow_new_user(开关)、invite_code、enabled(开关)。写操作 `v-auth="'billing:manage'"`。
  - 删除 `Popconfirm`+`deleteTrial`。
  - 行展开/详情：授予资格（输入 userId → `grantEligibility(id, userId)`）+ 发放记录（`trialGrants(id)` 列表）。
- [ ] **Step 2：router + 导航 + RBAC** — 增 `/billing/trials`（`billingTrials`，`menu.billingTrials`，icon `Gift`，`meta.auth:'billing:view'`）；**给全部 billing 子路由补 `meta.auth:'billing:view'`**（pricing/discounts/orders/accounts/trials）；分组注释去掉「原型/暂无对接」。Icon 注册 Gift（若无）。
- [ ] **Step 3：i18n（双语全部新键）+ `pnpm build`** + **全量校验**：`cd admin && pnpm build && ./node_modules/.bin/vitest run`。
- [ ] **Step 4：提交** `feat(admin): 试用配置页 + 计费路由RBAC/导航/i18n`。

---

## 计划 7b 验收对照（PRD §12 admin 端）

| 页面 | 覆盖 |
|---|---|
| 定价管理(真 SKU/规格/单价/上下架) | Task 2 PricingView |
| 折扣管理(阶梯;优惠码下线) | Task 2 DiscountsView |
| 订单管理(真,全用户,标记已付) | Task 3 OrdersView |
| 账户/资源管理(查账户+手动赠送/扣减,带理由=退款入口) | Task 3 AccountsView |
| 试用配置(策略CRUD+授予资格+发放记录) | Task 4 TrialsView |
| RBAC(billing:view 列表 / billing:manage 写) | 各页 v-auth + 路由 meta.auth |

> 欠费/账户状态运营视图为可选(PRD §12 未强制)，本期不做；如需，后续可加 dunning 只读视图。

## Self-Review 记录
- **Spec 覆盖**：PRD §12 admin 全部页面 + RBAC；优惠码明确下线。
- **占位扫描**：mock 为联调假数据(预期)；视图接真，去静态。
- **类型一致**：types/billing.ts snake_case 对齐后端；API 方法与 `/admin/billing/*` 端点一致；money fmtCents/fmtDiscountBps 复用；bps(10000=全价)统一。
- **约定**：`pnpm build` 每任务必过；`v-auth`/`meta.auth` RBAC；DataTable/Popconfirm/Dialog/Icon/i18n 双语按 admin 约定；金额分→元。
