# Phase 2 计量与结算（开机时长费）设计

- 日期：2026-06-13
- 定位：实现计费 PRD（[2026-06-07-billing-system-design.md](2026-06-07-billing-system-design.md)）的 §5.2 开机时长费 + 解除 §14.1 阻塞项。
- 触发：中台运行日志接口 §2.9 `/cloud-phone-status-logs/page` 上线，即 PRD §14.1「方案 A：开机/关机时间段列表」。运行日志查看功能已落地（见 phone 模块 `RunLogs`）。

## 1. 目标与一句话

把云手机运行会话从中台同步过来，基于真实运行时长计费：
**同步运行会话 → 逐分钟时间线 + 开机席位并发覆盖 → 事后增量扣费（覆盖优先级）→ 准实时护栏关停超额台**。

产品决策（已与用户确认）：
- 结算模型：**事后结算 + 准实时护栏**。只对已流逝的真实分钟计费，绝不预扣未来。
- 席位覆盖：**逐分钟精确时间线**（非按会话近似）。
- 护栏策略：余额/时长包不足时**只关「超出开机席位并发容量且无余额覆盖」的那几台**运行中手机（后开先关）。
- 节奏：同步 + 结算 **1 分钟**一次；准实时护栏 **30 秒**一次。

## 2. 模块分工（遵循既有 phone → billing 单向依赖）

现状依赖方向是 **phone → billing**：phone 的 `runEnforcement` 调 `billing.ListDunningEnforcement()` 等公开门面并执行关机/销毁；billing 从不 import phone。计量沿用同一反转：

- **phone 模块**（能调中台 + 拥有 cp→user 映射 + 有关机通道）：
  - 运行日志**同步**到自有 `run_sessions` 表。
  - **结算编排**：按用户取运行区间，调 `billing.SettleRuntime`。
  - **护栏关机**：算超额台，调 `billing.RuntimeCoverage` 判断，关停超额台。
  - 复用现有 `framework.PeriodicRunner`（DB lease 单飞）+ 关机通道。
- **billing 模块**（Metering 子域，拥有钱 / 权益 / 算法）：
  - `SettleRuntime` 计量算法（逐分钟覆盖 + 覆盖优先级扣费 + 用量切片 + 流水 + 幂等水位）。
  - `RuntimeCoverage` 只读门面（给护栏读：可用开机席位 S、剩余时长包分钟、钱包余额、单价）。
  - 时长费单价等配置；用量查询 API。

## 3. 数据模型（新表）

### 3.1 `run_sessions`（phone 模块）
| 字段 | 说明 |
|---|---|
| `id` | PK |
| `log_no` | 中台日志编号，**唯一索引**（upsert 幂等） |
| `cp_id` | 云手机编号 |
| `user_id` | 属主，同步时由 phone 自己的 `cloud_phones` 解析；解析不到（非我方/已删）则丢弃不入库 |
| `vm_uid` | 服务器编号 |
| `power_on_at` | 开机时间（由 §2.9 `powerOnTime` 解析为 time） |
| `power_off_at` | 关机时间；运行中为 NULL（§2.9 返回「运行中」） |
| `session_status` | RUNNING / SHUTDOWN（存中台码而非展示值） |
| `power_off_reason_code` | SHUTDOWN / CONTAINER_FAULT / ...；运行中 NULL |
| `synced_at` | 最近同步时间 |

索引：`uniq(log_no)`、`idx(user_id, power_on_at)`、`idx(power_off_at)`（找运行中）。

### 3.2 `runtime_usage_slices`（billing 模块）
每次结算每用户一条扣费切片：`user_id, window_start, window_end, billable_unit_minutes（席位覆盖后的台·分钟）, covered_seat_minutes, charged_pack_minutes, charged_balance_cents, unit_price_cents, created_at`。供用量页展示与对账。

### 3.3 `runtime_settlement_watermarks`（billing 模块）
`user_id(唯一), settled_until(time)`。每用户已结算到的时刻，结算幂等核心。

### 3.4 `billing_runtime_config`（billing 模块，单行表）
单行配置（`id=1` 固定，`RegisterSetup` 幂等 seed 默认值）：`unit_price_cents_per_minute`（时长费单价，元/台/分钟，默认待产品定）、`low_balance_alert_cents`（低余额告警阈值）、`rounding_granularity_sec`（取整粒度，默认 60）、`updated_at`。后台可配。

> 说明：现有 catalog 的 `time_pack`（分/小时）是「卖时长包」的价，**不等于超额消耗的扣费单价**，故独立配置。

## 4. 结算算法（逐分钟时间线 · 事后增量）

每分钟 cron（phone 编排 + billing 计算），对每个在窗口内有运行记录的用户：

1. 窗口 `(settled_until, target]`，`target = now`（仅结算**已发生**分钟，含运行中会话「`power_on_at` → now」的已过部分；从不预扣未来）。
2. 对窗口内每分钟 m：
   - `R(m)` = 该用户跨 `run_sessions` 覆盖 m 的并发运行台数（会话覆盖 m ⟺ `power_on_at ≤ m < power_off_at`；运行中会话以 now 作右界）。
   - `S(m)` = 该用户在 m 时刻可用 `boot_seat` 容量（未过期批次容量和）。
   - **应计台·分钟** `= max(0, R(m) − S(m))`，同时累计被席位覆盖的 `min(R(m), S(m))`。
3. Σ 应计台·分钟，按**覆盖优先级**扣减并写流水（事务一致）：
   - 先扣 `runtime_minute` 时长包（FIFO，临到期批次优先），
   - 不足部分按 `unit_price_cents_per_minute` 从**钱包余额**扣，
   - 记 `runtime_usage_slices` + `billing_ledger` 双科目（runtime_minute 负 delta / balance 负 delta）。
4. `settled_until = target`。

幂等：水位保证同一分钟不重复计费；重跑安全。取整：不足 1 分钟按 `rounding_granularity_sec` 向上取整。

### 接口（billing 公开门面）
```
billing.SettleRuntime(userID uint, windowEnd time.Time, runIntervals []Interval) SettleResult
  // Interval = {Start, End(nil=运行中→以 windowEnd 为界)}；billing 内部按 settled_until 裁剪窗口、
  // 逐分钟算 R/S 覆盖、按优先级扣费、写切片+流水、推进水位。返回扣费明细 + 未覆盖（欠费）量。
billing.RuntimeCoverage(userID uint) Coverage
  // {AvailableBootSeats, RemainingPackMinutes, BalanceCents, UnitPriceCents}
```
phone 传「该用户近窗口的运行区间」，billing 负责裁剪 + 计量 + 扣费 + 幂等。水位归 billing 所有。

## 5. 准实时护栏（30s cron，phone）

对每个有运行中会话的用户：
1. 算当前超额运行台数 `over = max(0, R_now − S_now)`。
2. 读 `billing.RuntimeCoverage`：剩余时长包分钟 + 钱包可买分钟（余额/单价）= 还能支撑的台·分钟预算。
3. 若预算不足以覆盖 `over` 台的**下一个结算间隔（1 分钟）** → 关停其中无法覆盖的台：**按开机时间后开先关**（席位与预算优先保障先开的台）+ 发低余额/欠费通知。
4. 复用 phone 关机通道；幂等（跳过已在关机/过渡态的台）。

护栏只**预防**未来分钟，不扣费；扣费仍由 §4 事后结算负责。

## 6. 同步引擎（1 分 cron，phone）

拉 §2.9 `/cloud-phone-status-logs/page`（不传 `cpId` = 全租户范围），分页，按 `log_no` upsert 到 `run_sessions`；`cp_id` 不属于我方用户的丢弃。运行中会话每轮更新（仍 NULL 关机时间）；关机后该 `log_no` 行补齐 `power_off_at`。

⚠️ **风险与对策**：§2.9 的 DTO **无时间窗筛选**（文档明确「如需按时间段查询需后端补充」），且返回排序未经真机确认。
- 增量策略：假设最新在前 → 翻页直到「整页都是已知且已 SHUTDOWN 且已结算」即停；单轮限 `maxPages`；每小时一次全量兜底重扫。
- **并行向中台申请补 `startTime/endTime` 参数**（PRD §14.1 本就要求时间窗）；到位后改为按 `settled_until`/`synced_at` 真增量，去掉翻页兜底。

## 7. 前后台 UI

- 前台「账户与资源」用量页：新增**运行用量**区块 —— 本计费周期累计运行台·分钟、席位覆盖 / 已扣时长包 / 已扣余额、最近用量切片列表。（单台原始会话已有运行日志弹框。）
- 后台计费配置页：新增**时长费单价**（元/台/分钟）+ 低余额告警阈值 + 取整粒度。

## 8. 错误处理与边界

- 幂等：`run_sessions` 按 `log_no` upsert；结算按 `settled_until` 水位；扣费走数据库事务。
- 单飞：复用 `PeriodicRunner` 的 DB lease，避免多实例重复结算/护栏。
- 余额可能因事后扣费短暂为负：以中台用量为事实**仍照扣**（不阻断），由护栏 + 低余额预警兜底，欠费走既有 dunning 心智。
- 时间解析：§2.9 时间为 `yyyy-MM-dd HH:mm:ss` 字符串，按中台时区解析为 time（统一 UTC 存储）。
- cp→user 解析不到：丢弃（销毁/转移的历史会话不向当前不存在的属主计费）。
- 中台同步失败/限流：best-effort，下轮重试；结算只用本地 `run_sessions`，与同步解耦。

## 9. 测试

- **billing**：`SettleRuntime` 纯算法单测 —— 逐分钟 R/S 覆盖、覆盖优先级扣减（时长包 FIFO → 余额）、水位幂等（重跑不重复扣）、跨批次/批次到期、取整。`RuntimeCoverage` 计算。
- **phone**：同步 upsert 幂等（同 `log_no` 不重复）、结算编排区间生成、护栏选台（后开先关、只关超额无覆盖台）。用 fake 运行会话 + fake billing 门面，不依赖真中台。
- 后端 `go build ./... && go vet ./... && go test ./...` 通过；前端 `pnpm build` 通过。

## 10. 实施分期（单 spec，内部排序）

1. 同步引擎 + `run_sessions`（phone）。
2. billing 计量算法 `SettleRuntime` + `RuntimeCoverage` + 配置表 + 用量切片/流水。
3. 结算编排 cron（phone → billing）。
4. 准实时护栏 cron（phone）。
5. 用量 UI（前台）+ 单价配置（后台）。

每步可独立编译 + 测试通过再进下一步。

## 11. 范围 / Non-goals

- 本期：上述五步，真实按时长扣费 + 护栏 + 用量展示 + 单价配置。
- 不做：导出对账 CSV（§2.9.2）、按时间窗的中台真增量（待中台补参数）、发票 / 优惠码 / 原路退款（沿用 PRD Non-goals）。
