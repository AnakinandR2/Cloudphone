# 后端数据库批量与并行优化设计

**日期**: 2026-06-23  
**范围**: `backend/` 全后端

---

## 背景

扫描后端发现 10 处可以批量或并行但未优化的操作，分三类：
1. N+1 写入（循环单条 INSERT → 批量）
2. 独立 DB/API 查询串行执行（→ 并行）
3. SQLite 默认 journal mode 不支持并发读（→ WAL 模式）

---

## 设计

### 前置条件：SQLite WAL 模式

**文件**: `framework/db.go`

在 SQLite 连接建立后立即执行：
```sql
PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;
```

同时为 SQLite 设置连接池（MaxOpenConns=10），允许多 goroutine 并发读。WAL 模式下多读一写不互斥，解锁后续所有并行化的前提。

---

### 批量优化

#### B1: proxy BatchCreate 逐条 INSERT → 批量
`modules/proxy/internal/service.go`

收集所有有效 `ProxyDB` 条目，一次 `db.CreateInBatches(&items, 100)` 代替循环 `s.Create()`。

#### B2: staff seedReadonlyUser 权限逐条 INSERT → 批量
`modules/staff/internal/module.go`

先 `db.Delete` 清空已有权限，然后一次 `db.Create(&perms)` 批量写入。

---

### 并行优化

对所有并行改造，统一规则：
- 使用 `sync.WaitGroup` 或 `golang.org/x/sync/errgroup`
- 并发写 slice 元素时，各 goroutine 只写各自独立的 map，最后串行 apply → 避免 race
- 后台 cron 函数（metering/recycle/automation）用 `errgroup`，单项失败不中断整批

#### P1: phone GetList 三个 enrichment 并行
`modules/phone/internal/service.go`

将 `resolveLiveStatuses`/`enrichAdb`/`enrichRoot` 重构为各自返回 `map[string]T`（不直接写 items），用 `sync.WaitGroup` 三个 goroutine 并行获取，`wg.Wait()` 后串行 apply 到 items。

#### P2: phone FileDelete 文件并行删除
`modules/phone/internal/service.go`

用 `errgroup` 并发调 `s.ops.FileDelete()`（每文件一个 goroutine），返回首个错误。

#### P3: phone runRuntimeGuard 用户间 + 用户内并行
`modules/phone/internal/metering.go`

- 用户内：`BootSlotCapacity` 和 `RuntimeMinutesRemaining` 用 2 个 goroutine 并行
- 用户间：整个用户循环改为 `errgroup`

#### P4: phone runSettlement 用户间并行
`modules/phone/internal/metering.go`

`for uid, ivs := range byUser` → `errgroup` 并发调 `billing.SettleRuntime`。`SettleRuntime` 已幂等，用户间互不影响。

#### P5: automation discoverPlanTasks plan 间并行
`modules/automation/internal/worker.go`

`errgroup` 并发按 plan 拉中台任务，各 goroutine 只写本地变量，最后在主 goroutine 串行 upsert。

#### P6: phone runRecycleCleanup 实例并行销毁
`modules/phone/internal/recycle.go`

先用 `errgroup` 并行调中台 `Destroy`，成功的记录收集后串行做本地 `deleteByID` + `ReleaseInstanceOccupancy`（保本地一致性）。

#### P7: phone runReconcilePatrol 用户间并行
`modules/phone/internal/recycle.go`

`for _, uid := range ids` → `errgroup` 并发调 `s.reconcileSeats`，单用户失败 log 但不中断。

#### P8: billing newModelCapacities 三查询并行
`modules/billing/internal/account_caps.go`

`Seat` / `BootSlot` / `RuntimeMinute` 三个独立 DB 查询用 3 个 goroutine 并行，`wg.Wait()` 后组装结果。任一出错则整体返回 error。

---

## 错误处理原则

| 场景 | 策略 |
|------|------|
| GetList enrichment 失败 | best-effort：失败不影响主列表，字段保留默认值 |
| FileDelete 失败 | 返回首个错误（errgroup） |
| 后台 cron（metering/recycle） | best-effort：单用户/实例失败只 log，不中断整批 |
| billing capacities | 任一失败返回 error（调用方已有错误处理） |

---

## 涉及文件

| 文件 | 改动类型 |
|------|---------|
| `framework/db.go` | WAL PRAGMA + SQLite 连接池 |
| `modules/proxy/internal/service.go` | BatchCreate → 批量 INSERT |
| `modules/staff/internal/module.go` | seedReadonlyUser 权限批量 INSERT |
| `modules/phone/internal/service.go` | GetList enrichment 并行 + FileDelete 并行 |
| `modules/phone/internal/metering.go` | runRuntimeGuard + runSettlement 并行 |
| `modules/automation/internal/worker.go` | discoverPlanTasks 并行 |
| `modules/phone/internal/recycle.go` | runRecycleCleanup + runReconcilePatrol 并行 |
| `modules/billing/internal/account_caps.go` | newModelCapacities 并行 |
