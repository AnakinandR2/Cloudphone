# 试用/福利支持多类资源发放 — 设计

日期：2026-06-18
状态：已通过设计评审，待写实现计划

## 背景与问题

当前一个试用策略 `TrialPolicy` 只能发放**单一资源科目**（`grant_subject` + `grant_quantity` + `grant_expire_days`，见 [trial_model.go](../../../backend/modules/billing/internal/trial_model.go)）。运营希望**一个试用/福利配置能同时包含三类资源**——实例数量(`instance_seat`)、开机时长(`runtime_minute`)、包月数量(`boot_seat`)，用户一次领取全部发放。

现状链路：`ClaimTrial` → repo `claim()` 在单事务里建 1 个 `EntitlementBatch` + 1 条 `TrialGrant` + 1 条 ledger（type=trial）。限领次数 `per_user_limit` 按 `TrialGrant` 行数计。

## 需求决策（已确认）

- 一个策略可配置三类资源各自的**数量**，一次领取原子发放全部。
- **每项独立到期**：每个发放项各设有效天数（0=永久）。
- **不做数据回填、不做迁移**：直接清库重建（开发环境）。

## 方案（两张子表，规范 DDD）

采用配置子表 + 领取头表，与现有 `EntitlementBatch`/ledger 风格一致，限领计数正确、每项独立到期、未来加资源类型不改表。

### 1. 数据模型

- **`TrialPolicy`**：移除 `GrantSubject / GrantQuantity / GrantExpireDays`。保留 `code / name / enabled / per_user_limit / allow_new_user / invite_code / 时间戳`。
- **新增 `TrialPolicyItem`**（表 `billing_trial_policy_items`）：
  ```
  ID uint pk
  PolicyID uint  index, uniqueIndex(policy_id, subject)
  Subject string         // instance_seat / runtime_minute / boot_seat
  Quantity int64
  ExpireDays int          // 0 = 永久
  ```
- **新增 `TrialClaim`**（表 `billing_trial_claims`）：`{ ID, PolicyID, UserID, CreatedAt }`，一次领取一行；**`per_user_limit` 按本表计数**。`index(policy_id, user_id)`。
- **`TrialGrant`**（沿用，表 `billing_trial_grants`）：加 `ClaimID uint`（关联领取头），保留 `Subject / Quantity`，作为发放明细/审计。

无迁移系统：`AutoMigrate` 自动建两张新表 + 给 `TrialGrant` 加 `claim_id` 列；旧 `TrialPolicy` 单科目列直接随清库消失（不读、不回填）。`module.go` 的 AutoMigrate 列表加入 `TrialPolicyItem{}`、`TrialClaim{}`。

### 2. 服务 / 领取

- **资格判定**（`eligible` / `ClaimTrial` 前段）：逻辑不变（新用户=已付订单 0 / 手动授予 / 邀请码精确匹配）；唯一变化是 `per_user_limit` 改为 `countClaims = COUNT(TrialClaim WHERE policy_id,user_id)`。
- **`claim()` 单事务**：
  1. 守卫式再查 `countClaims < per_user_limit`（防并发超领，sqlite 串行安全；迁 MySQL/PG 需唯一约束/行锁，沿用现有注释标注）。
  2. 建 `TrialClaim`，拿到 `claimID`。
  3. 遍历该策略 `TrialPolicyItem`：按 `item.ExpireDays` 各算 `expireAt`（>0 才设）→ `EntitlementBatch{subject, quantity, source=trial, source_ref="trial:"+code, expire_at}` + `TrialGrant{claim_id, policy_id, user_id, subject, quantity}` + ledger（type=trial，delta=quantity，balance_after=该科目最新容量）。
  4. 任一步失败整体回滚。
- **`CreatePolicy`**：入参 `items[]`；校验：≥1 项、每项 `subject ∈ {instance_seat, runtime_minute, boot_seat}`、科目不重复、`quantity > 0`、`expire_days ≥ 0`；`code` 唯一。事务建 policy + items。
- **`UpdatePolicy`**：策略元信息字段照旧；若传 `items` 则**整体替换**（删旧 items 建新，同样校验）。
- **响应**：策略对象内嵌 `items []TrialPolicyItem`（前台 `ListClaimable` 仍抹除 `invite_code`）。

### 3. 接口 / DTO

- `TrialPolicyCreate`：`{ code, name, enabled?, per_user_limit?, allow_new_user?, invite_code?, items: [{subject, quantity, expire_days}] }`。
- `TrialPolicyUpdate`：`{ name?, per_user_limit?, allow_new_user?, invite_code?, enabled?, items?: [...] }`。
- `TrialPolicy` 响应 + `ClaimableItem.Policy` 内嵌 `items`。
- 路由不变（`/admin/billing/trials*`、`/billing/trials*`）。

### 4. 后台 admin `TrialsView`

- 表单：单科目区改为**三行可选发放项**（实例数量 / 开机时长 / 包月数量），每行 数量(0=不发该项) + 有效天数(0=永久)。提交时过滤掉数量为 0 的项；全 0 则前端拦下提示「至少配置一项」。
- 列表摘要单元格展示全部项，例：`实例×2 30天 ・ 开机时长×600分 永久 ・ 包月×1 30天`。
- 编辑回填：把 `policy.items` 映射回三行。

### 5. 前台 my `BillingTrialsView`

- 领取卡片的单项摘要改为**多项列表**：遍历 `item.policy.items`，每项展示 数量+单位+到期（单位/到期文案复用现有 `subjectUnit` / `trialExpireDays`）。

### 6. 校验 / 错误

- 创建/更新：至少 1 项、科目合法且不重复、数量>0、到期≥0，否则 `apperr.Validation`。
- 领取：策略停用 / 超限 / 不符合资格 → 现有错误码不变。

### 7. 测试

- **后端**（billing internal）：
  - 一次领取多科目（如 instance_seat×2 + runtime_minute×600 + boot_seat×1）→ 三类容量均到账、写 1 个 TrialClaim + 3 条 TrialGrant + 3 条 ledger。
  - 限领：`per_user_limit=1` 时领第 2 次被拒（按 TrialClaim 计，不被"多行 grant"误判）。
  - 每项独立到期：设不同 expire_days，验证各 batch 的 expire_at。
  - 创建校验：无项 / 重复科目 / 数量≤0 → 报错。
  - Update 替换 items 生效。
- **前端**：`pnpm build`（双端 vue-tsc）+ 现有 vitest 通过；mock 更新为多项结构。

## 不做（YAGNI）

- 不做旧数据回填 / 迁移（直接清库）。
- 不引入第 4 类资源或余额型试用（仍仅三类资源科目）。
- 不改资格判定维度（新用户/手动授予/邀请码不变）。
