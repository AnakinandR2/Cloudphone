# 后端批量与并行优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 消除后端 9 处串行/逐条 DB 操作，改为批量 INSERT 或并行 goroutine，并开启 SQLite WAL 模式支持并发读。

**Architecture:** 每个任务独立修改一个或两个文件，不引入新抽象，只改实现。并行化用 `sync.WaitGroup` 或 `golang.org/x/sync/errgroup`，批量化用 GORM 的 `db.Create(&slice)`。SQLite WAL 模式在 `InitDB` 结尾执行 PRAGMA，对 MySQL/Postgres 无影响。

**Tech Stack:** Go 1.21+, GORM, `golang.org/x/sync v0.19.0`（已在 go.mod），SQLite（WAL journal mode）

## Global Constraints

- 模块路径前缀 `manager-backend`（已有，不改）
- 测试用真实 sqlite，命令 `go test ./...` 或 `go test ./modules/xyz/internal/`
- 提交前 `gofmt -w <file>`
- 并行函数必须 race-free：各 goroutine 只写独占变量，`wg.Wait()` 后统一 apply
- Best-effort cron（metering/recycle）：单项失败只 log，不返回 error，不中断整批
- 不改 service/repository 接口签名（proxy 除外，需加 `createBatch`）

---

## 涉及文件总览

| 文件 | 任务 | 改动类型 |
|------|------|---------|
| `framework/db.go` | T1 | WAL PRAGMA + SQLite 连接池 |
| `modules/proxy/internal/repository.go` | T2 | 加 `createBatch` 接口+实现 |
| `modules/proxy/internal/service.go` | T2 | BatchCreate 批量 INSERT |
| `modules/staff/internal/module.go` | T3 | seedReadonlyUser 权限批量 INSERT |
| `modules/phone/internal/service.go` | T4 | enrichParallel + GetList 改调 + FileDelete errgroup |
| `modules/phone/internal/metering.go` | T5 | runSettlement + runRuntimeGuard 并行 |
| `modules/automation/internal/worker.go` | T6 | discoverPlanTasks 并行 |
| `modules/phone/internal/recycle.go` | T7 | runRecycleCleanup + runReconcilePatrol 并行 |
| `modules/billing/internal/account_caps.go` | T8 | newModelCapacities 并行 |

---

## Task 1: SQLite WAL 模式 + 连接池

**Files:**
- Modify: `framework/db.go`

**Interfaces:**
- Produces: SQLite 连接支持多并发读、写等待 5s 不返回 BUSY

- [ ] **Step 1: 修改 `InitDB` 中 SQLite 分支**

将 `framework/db.go` 的 `if dbType != "sqlite"` 块替换为下面版本（原 79-87 行），在其后增加 SQLite 的连接池 + WAL PRAGMA：

```go
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("获取数据库实例失败: %v", err)
	}
	if dbType == "sqlite" {
		// WAL 模式：允许并发读（多读一写），写等待最多 5 秒再报 BUSY。
		DB.Exec("PRAGMA journal_mode=WAL")
		DB.Exec("PRAGMA busy_timeout=5000")
		sqlDB.SetMaxOpenConns(10)
		sqlDB.SetMaxIdleConns(5)
		sqlDB.SetConnMaxLifetime(time.Hour)
	} else {
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetConnMaxLifetime(time.Hour)
	}
```

完整替换后的 `InitDB` 函数（79-90 行变为以下内容）：

```go
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("获取数据库实例失败: %v", err)
	}
	if dbType == "sqlite" {
		DB.Exec("PRAGMA journal_mode=WAL")
		DB.Exec("PRAGMA busy_timeout=5000")
		sqlDB.SetMaxOpenConns(10)
		sqlDB.SetMaxIdleConns(5)
		sqlDB.SetConnMaxLifetime(time.Hour)
	} else {
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetConnMaxLifetime(time.Hour)
	}
```

- [ ] **Step 2: 格式化并编译**

```bash
cd /home/root/workspace005/gloryphone-code/backend
gofmt -w framework/db.go
go build ./...
```

Expected: 无错误

- [ ] **Step 3: 跑 framework 测试**

```bash
go test ./framework/... -v -count=1 2>&1 | tail -20
```

Expected: PASS（所有 framework 测试通过）

- [ ] **Step 4: Commit**

```bash
cd /home/root/workspace005/gloryphone-code/backend
git add framework/db.go
git commit -m "perf(db): enable SQLite WAL mode and connection pool for concurrent reads"
```

---

## Task 2: proxy BatchCreate 批量 INSERT

**Files:**
- Modify: `modules/proxy/internal/repository.go`（加接口方法 + 实现）
- Modify: `modules/proxy/internal/service.go`（BatchCreate 改用批量）
- Test: `modules/proxy/internal/service_test.go`（已有 `TestProxyBatchCreate`，应继续通过）

**Interfaces:**
- 新增 repository 方法: `createBatch(items []Proxy) error`
- BatchCreate 语义变化：从「单条失败返回已创建数」改为「全部成功或全部失败」（更合理的 batch 语义）

- [ ] **Step 1: 在 repository interface 加 `createBatch`**

在 `modules/proxy/internal/repository.go` 的 `repository` interface 中，`create(item *Proxy) error` 后加一行：

```go
	createBatch(items []Proxy) error
```

完整 interface（第 7-21 行）变为：

```go
type repository interface {
	// 前台（属主隔离）
	count(userID int, kw string) (int64, error)
	list(userID, offset, limit int, kw, orderClause string) ([]Proxy, error)
	listAllOwned(userID int) ([]Proxy, error)
	findByID(userID, id int) (*Proxy, error)
	create(item *Proxy) error
	createBatch(items []Proxy) error
	update(userID, id int, fields map[string]interface{}) error
	delete(userID, id int) error
	// 管理侧（不限属主）
	adminCount(kw, status string) (int64, error)
	adminList(offset, limit int, kw, status, orderClause string) ([]Proxy, error)
	adminFindByID(id int) (*Proxy, error)
	adminDelete(id int) error
}
```

- [ ] **Step 2: 实现 `createBatch` 在 gormRepository**

在 `modules/proxy/internal/repository.go` 第 68 行（`create` 方法）后面加：

```go
func (r *gormRepository) createBatch(items []Proxy) error { return r.db.Create(&items).Error }
```

- [ ] **Step 3: 改 service.go `BatchCreate`**

将 `modules/proxy/internal/service.go` 中 `BatchCreate`（第 163-177 行）整体替换为：

```go
// BatchCreate 批量为当前用户新增代理：跳过缺 host/port 的无效条目，一次性 INSERT，返回成功条数。
func (s *serviceImpl) BatchCreate(userID int, items []ProxyCreate) (int, error) {
	toCreate := make([]Proxy, 0, len(items))
	for i := range items {
		it := items[i]
		if it.Host == "" || it.Port == 0 {
			continue
		}
		toCreate = append(toCreate, Proxy{
			UserID:   uint(userID),
			Name:     it.Name,
			Protocol: normalizeProtocol(it.Protocol),
			Host:     it.Host,
			Port:     it.Port,
			Username: it.Username,
			Password: it.Password,
			Region:   it.Region,
			Status:   StatusUnknown,
			Remark:   it.Remark,
		})
	}
	if len(toCreate) == 0 {
		return 0, nil
	}
	if err := s.repo.createBatch(toCreate); err != nil {
		return 0, err
	}
	return len(toCreate), nil
}
```

- [ ] **Step 4: 格式化并编译**

```bash
gofmt -w modules/proxy/internal/repository.go modules/proxy/internal/service.go
go build ./...
```

Expected: 无错误

- [ ] **Step 5: 跑 proxy 测试**

```bash
go test ./modules/proxy/internal/... -v -count=1 -run TestProxyBatchCreate 2>&1
```

Expected: `PASS TestProxyBatchCreate`

- [ ] **Step 6: 跑 proxy 全部测试**

```bash
go test ./modules/proxy/internal/... -count=1 2>&1 | tail -10
```

Expected: ok

- [ ] **Step 7: Commit**

```bash
git add modules/proxy/internal/repository.go modules/proxy/internal/service.go
git commit -m "perf(proxy): batch INSERT in BatchCreate instead of N serial creates"
```

---

## Task 3: staff seedReadonlyUser 权限批量 INSERT

**Files:**
- Modify: `modules/staff/internal/module.go`

- [ ] **Step 1: 替换权限循环插入为批量**

在 `modules/staff/internal/module.go` 找到 `seedReadonlyUser` 函数中以下 3 行（约 174-178 行）：

```go
	// 确保权限正确：清空后重建
	db.Where("role_id = ?", roleDB.ID).Delete(&RolePermissionDB{})
	for _, p := range wantPerms {
		db.Create(&RolePermissionDB{RoleID: roleDB.ID, Permission: p})
	}
```

替换为：

```go
	// 确保权限正确：清空后批量重建
	db.Where("role_id = ?", roleDB.ID).Delete(&RolePermissionDB{})
	perms := make([]RolePermissionDB, 0, len(wantPerms))
	for _, p := range wantPerms {
		perms = append(perms, RolePermissionDB{RoleID: roleDB.ID, Permission: p})
	}
	if len(perms) > 0 {
		db.Create(&perms)
	}
```

- [ ] **Step 2: 格式化并编译**

```bash
gofmt -w modules/staff/internal/module.go
go build ./...
```

Expected: 无错误

- [ ] **Step 3: 跑 staff 测试**

```bash
go test ./modules/staff/internal/... -count=1 2>&1 | tail -10
```

Expected: ok

- [ ] **Step 4: Commit**

```bash
git add modules/staff/internal/module.go
git commit -m "perf(staff): batch INSERT permissions in seedReadonlyUser"
```

---

## Task 4: phone GetList 并行 enrichment + FileDelete errgroup

**Files:**
- Modify: `modules/phone/internal/service.go`

**重构策略：**
- 保留 `resolveLiveStatuses`（仍被 `GetByIDDisplay` 和 admin list 调用）
- 新增 `enrichParallel`：并行调三个中台接口，串行 apply 结果，替换 GetList 中的三行串行调用
- FileDelete 改为 errgroup 并行，返回首个错误

- [ ] **Step 1: 在 service.go import 加 `sync`**

在 `modules/phone/internal/service.go` 第 3 行 import 块加 `"sync"`：

```go
import (
	"context"
	"sync"
	"time"

	"manager-backend/framework/apperr"
	"manager-backend/framework/midplat"
	"manager-backend/framework/query"
	"manager-backend/modules/billing"
)
```

- [ ] **Step 2: 替换 GetList 中的三行串行调用**

将 `GetList` 函数体中（第 48-50 行）：

```go
	s.resolveLiveStatuses(items)
	s.enrichAdb(items)
	s.enrichRoot(items)
```

替换为：

```go
	s.enrichParallel(items)
```

- [ ] **Step 3: 在 `enrichRoot` 函数之后（约第 214 行）插入 `enrichParallel`**

在 `enrichRoot` 函数的右花括号 `}` 之后，`GetByIDDisplay` 之前插入：

```go
// enrichParallel 并行拉取三个中台 enrichment（状态/ADB/Root），串行 apply 到 items。
// 各 goroutine 仅写各自局部变量，wg.Wait() 后统一写 items，无数据竞争。
func (s *serviceImpl) enrichParallel(items []CloudPhone) {
	if s.ops == nil {
		return
	}
	cpIDs := make([]string, 0, len(items))
	for i := range items {
		if items[i].CpID != "" {
			cpIDs = append(cpIDs, items[i].CpID)
		}
	}
	if len(cpIDs) == 0 {
		return
	}

	var (
		statuses     map[string]string
		statusFailed bool
		adbMap       map[string]bool
		rootMap      map[string]bool
	)

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		ctx, cancel := opCtx()
		defer cancel()
		m, err := s.ops.Statuses(ctx, cpIDs)
		if err != nil {
			statusFailed = true
			return
		}
		statuses = m
	}()
	go func() {
		defer wg.Done()
		ctx, cancel := opCtx()
		defer cancel()
		m, err := s.ops.AdbEnabledMap(ctx, cpIDs)
		if err != nil {
			return
		}
		adbMap = m
	}()
	go func() {
		defer wg.Done()
		ctx, cancel := opCtx()
		defer cancel()
		m, err := s.ops.RootEnabledMap(ctx, cpIDs)
		if err != nil {
			return
		}
		rootMap = m
	}()
	wg.Wait()

	for i := range items {
		cp := &items[i]
		if cp.CpID == "" {
			continue // 未开通：保留本地态
		}
		if statusFailed {
			cp.Status = StatusUnknown
		} else if raw, ok := statuses[cp.CpID]; !ok || raw == "" {
			cp.Status = StatusUnknown
		} else {
			cp.Status = mapMidplatStatus(raw)
		}
		if adbMap != nil && adbMap[cp.CpID] {
			cp.AdbEnabled = true
		}
		if rootMap != nil && rootMap[cp.CpID] {
			cp.Rooted = true
		}
	}
}
```

- [ ] **Step 4: 改 FileDelete 为 errgroup 并行**

在 service.go 的 import 块中同时加入 `"golang.org/x/sync/errgroup"`：

```go
import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"manager-backend/framework/apperr"
	"manager-backend/framework/midplat"
	"manager-backend/framework/query"
	"manager-backend/modules/billing"
)
```

将 `FileDelete` 函数（第 602-622 行）替换为：

```go
// FileDelete 删除云手机上的一个或多个文件（并行透传中台 file-delete，返回首个错误）。
func (s *serviceImpl) FileDelete(userID, id int, paths []string) error {
	if len(paths) == 0 {
		return apperr.BadRequest("未选择文件")
	}
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	eg, ctx := errgroup.WithContext(ctx)
	for _, path := range paths {
		if path == "" {
			continue
		}
		path := path
		eg.Go(func() error {
			return s.ops.FileDelete(ctx, p.VmID, p.CpID, path)
		})
	}
	return eg.Wait()
}
```

- [ ] **Step 5: 格式化并编译**

```bash
gofmt -w modules/phone/internal/service.go
go build ./...
```

Expected: 无错误

- [ ] **Step 6: 跑 phone 测试**

```bash
go test ./modules/phone/internal/... -count=1 2>&1 | tail -15
```

Expected: ok（race detector 可选加 `-race`）

- [ ] **Step 7: Commit**

```bash
git add modules/phone/internal/service.go
git commit -m "perf(phone): parallel enrichment in GetList; parallel FileDelete"
```

---

## Task 5: phone metering 并行（runSettlement + runRuntimeGuard）

**Files:**
- Modify: `modules/phone/internal/metering.go`

- [ ] **Step 1: 加 import**

将 `modules/phone/internal/metering.go` 的 import 块替换为：

```go
import (
	"context"
	"log"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"manager-backend/modules/billing"
)
```

- [ ] **Step 2: 替换 `runSettlement` 中的用户循环**

将 `runSettlement` 函数中（第 30-34 行）：

```go
	for uid, ivs := range byUser {
		if _, err := billing.SettleRuntime(int(uid), now, ivs); err != nil {
			log.Printf("[metering] 结算失败 user=%d: %v", uid, err)
		}
	}
	_ = ctx
```

替换为：

```go
	eg := &errgroup.Group{}
	for uid, ivs := range byUser {
		uid, ivs := uid, ivs
		eg.Go(func() error {
			if _, err := billing.SettleRuntime(int(uid), now, ivs); err != nil {
				log.Printf("[metering] 结算失败 user=%d: %v", uid, err)
			}
			return nil // best-effort：单用户失败不中断整批
		})
	}
	_ = eg.Wait()
	_ = ctx
```

- [ ] **Step 3: 替换 `runRuntimeGuard` 中的用户循环**

将 `runRuntimeGuard` 函数中（第 57-75 行）：

```go
	for _, uid := range order {
		sess := byUser[uid] // power_on_at 升序：越靠后越晚开机
		// 新模型：包月开机名额(boot_slot)免费，超出名额的台靠临时时长支撑下一分钟。
		bootCap, _ := billing.BootSlotCapacity(int(uid))
		remain, _ := billing.RuntimeMinutesRemaining(int(uid))
		over := int64(len(sess)) - int64(bootCap) // 超出包月名额的运行中台数
		if over <= 0 {
			continue // 全在名额内，免费
		}
		// 下一分钟可支撑的超额台数 = 剩余临时时长分钟（每台·分钟）。新模型不再用余额折算分钟。
		budget := remain
		unfundable := over - budget
		if unfundable <= 0 {
			continue // 下一分钟付得起，暂不关
		}
		// 关最后开机的 unfundable 台（后开先关）。
		toClose := sess[int64(len(sess))-unfundable:]
		s.shutdownSessions(ctx, int(uid), toClose)
	}
```

替换为：

```go
	eg := &errgroup.Group{}
	for _, uid := range order {
		uid := uid
		sess := byUser[uid]
		eg.Go(func() error {
			// 用户内并行：包月名额与临时时长余量两个查询独立，可同时发出。
			var bootCap int
			var remain int64
			var innerWg sync.WaitGroup
			innerWg.Add(2)
			go func() {
				defer innerWg.Done()
				bootCap, _ = billing.BootSlotCapacity(int(uid))
			}()
			go func() {
				defer innerWg.Done()
				remain, _ = billing.RuntimeMinutesRemaining(int(uid))
			}()
			innerWg.Wait()

			over := int64(len(sess)) - int64(bootCap)
			if over <= 0 {
				return nil
			}
			unfundable := over - remain
			if unfundable <= 0 {
				return nil
			}
			toClose := sess[int64(len(sess))-unfundable:]
			s.shutdownSessions(ctx, int(uid), toClose)
			return nil
		})
	}
	_ = eg.Wait()
```

- [ ] **Step 4: 格式化并编译**

```bash
gofmt -w modules/phone/internal/metering.go
go build ./...
```

Expected: 无错误

- [ ] **Step 5: 跑 phone 测试（含 metering）**

```bash
go test ./modules/phone/internal/... -count=1 2>&1 | tail -15
```

Expected: ok

- [ ] **Step 6: Commit**

```bash
git add modules/phone/internal/metering.go
git commit -m "perf(phone): parallel settlement across users; parallel billing queries per user in guard"
```

---

## Task 6: automation discoverPlanTasks 并行

**Files:**
- Modify: `modules/automation/internal/worker.go`

**策略：** goroutine 并行拉中台任务（网络 I/O），用 mutex 收集结果，主 goroutine 串行 upsert（DB 写）。

- [ ] **Step 1: 加 import**

将 `modules/automation/internal/worker.go` 的 import 块替换为：

```go
import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)
```

（`midplat.ScriptTaskVO` 类型由接口返回，Go 可从赋值推断，不需显式命名该类型，无需引入 midplat 包。）

- [ ] **Step 2: 替换 `discoverPlanTasks`**

将整个 `discoverPlanTasks` 函数替换为：

```go
// discoverPlanTasks 并行按 planUid 拉中台派生任务，串行 upsert 进本地索引。
// goroutine 内构建好 AutomationTask 切片，用 mutex 汇总，主 goroutine 串行写库。
func (s *serviceImpl) discoverPlanTasks(ctx context.Context) {
	plans, err := s.repo.activePlans()
	if err != nil {
		return
	}

	var (
		mu       sync.Mutex
		allTasks []*AutomationTask
	)

	eg, ctx := errgroup.WithContext(ctx)
	for i := range plans {
		p := plans[i]
		if p.PlanUID == "" {
			continue
		}
		eg.Go(func() error {
			vos, err := s.ops.TasksByPlan(ctx, p.PlanUID)
			if err != nil {
				return nil // best-effort：单 plan 失败跳过
			}
			tasks := make([]*AutomationTask, 0, len(vos))
			for _, vo := range vos {
				tasks = append(tasks, &AutomationTask{
					UserID:        p.UserID,
					MidTaskID:     vo.ID,
					TaskNo:        vo.TaskID,
					ScriptLocalID: p.ScriptLocalID,
					ScriptName:    p.ScriptName,
					PlanLocalID:   p.ID,
					CpID:          vo.CpID,
					TaskName:      vo.TaskName,
					Trigger:       TriggerPlan,
					LastStatus:    vo.TaskStatus,
					RunStart:      vo.RunStartTime,
					RunEnd:        vo.RunEndTime,
				})
			}
			mu.Lock()
			allTasks = append(allTasks, tasks...)
			mu.Unlock()
			return nil
		})
	}
	_ = eg.Wait()

	for _, t := range allTasks {
		_ = s.repo.upsertTaskByMidID(t)
	}
}
```

- [ ] **Step 3: 格式化并编译**

```bash
gofmt -w modules/automation/internal/worker.go
go build ./...
```

Expected: 无错误

- [ ] **Step 4: 跑 automation 测试**

```bash
go test ./modules/automation/internal/... -count=1 2>&1 | tail -10
```

Expected: ok

- [ ] **Step 5: Commit**

```bash
git add modules/automation/internal/worker.go
git commit -m "perf(automation): parallel TasksByPlan calls in discoverPlanTasks"
```

---

## Task 7: phone recycle 并行（runRecycleCleanup + runReconcilePatrol）

**Files:**
- Modify: `modules/phone/internal/recycle.go`

- [ ] **Step 1: 加 import**

将 `modules/phone/internal/recycle.go` 的 import 块替换为：

```go
import (
	"context"
	"log"
	"math"
	"time"

	"golang.org/x/sync/errgroup"
	"manager-backend/framework/apperr"
	"manager-backend/modules/billing"
)
```

- [ ] **Step 2: 替换 `runRecycleCleanup`**

将整个 `runRecycleCleanup` 函数替换为：

```go
// runRecycleCleanup 回收站清理（每日 cron §3）：回收超保留天数的实例 → 调中台销毁 + 硬删本地记录。
// 中台销毁并行执行；本地清理串行（保一致性）。
func (s *serviceImpl) runRecycleCleanup(ctx context.Context) {
	retention, err := billing.RecycleRetentionDays()
	if err != nil || retention <= 0 {
		retention = 30
	}
	before := time.Now().AddDate(0, 0, -retention)
	phones, err := s.repo.listExpiredRecycled(before)
	if err != nil {
		return
	}
	if len(phones) == 0 {
		return
	}

	// 并行销毁中台实例；results[i].err != nil 表示销毁失败，保留本地记录待下轮重试。
	type destroyResult struct {
		phone CloudPhone
		err   error
	}
	results := make([]destroyResult, len(phones))
	eg, ctx2 := errgroup.WithContext(ctx)
	for i := range phones {
		i, p := i, phones[i]
		eg.Go(func() error {
			if p.CpID == "" || s.ops == nil {
				results[i] = destroyResult{phone: p}
				return nil
			}
			log.Printf("[recycle-cleanup] 销毁过期回收实例 user=%d cp=%s phoneID=%d", p.UserID, p.CpID, p.ID)
			results[i] = destroyResult{phone: p, err: s.ops.Destroy(ctx2, p.CpID)}
			return nil // best-effort：销毁失败不取消其他
		})
	}
	_ = eg.Wait()

	// 串行本地清理：只处理中台销毁成功（或无需销毁）的实例。
	for _, r := range results {
		if r.err != nil {
			continue // 中台销毁失败：保留本地，下轮重试
		}
		if err := s.repo.deleteByID(r.phone.ID); err == nil && r.phone.CpID != "" {
			_ = billing.ReleaseInstanceOccupancy(int(r.phone.UserID), []string{r.phone.CpID})
		}
	}
}
```

- [ ] **Step 3: 替换 `runReconcilePatrol`**

将 `runReconcilePatrol` 函数（约 82-97 行）替换为：

```go
// runReconcilePatrol 周期席位巡检：对所有有「非回收」实例的用户跑 reconcile，
// 兜底处理席位过期/热迁移/溢出回收（关键事件触发之外的兜底）。best-effort，单用户失败不影响其它。
func (s *serviceImpl) runReconcilePatrol(ctx context.Context) {
	if s.ops == nil {
		return // 本地降级无中台：无 cpId 实例无法物化席位，跳过
	}
	ids, err := s.repo.activeOwnerIDs()
	if err != nil {
		return
	}
	eg := &errgroup.Group{}
	for _, uid := range ids {
		uid := uid
		eg.Go(func() error {
			if err := s.reconcileSeats(ctx, uid); err != nil {
				log.Printf("[reconcile-patrol] user=%d 失败: %v", uid, err)
			}
			return nil // best-effort
		})
	}
	_ = eg.Wait()
}
```

- [ ] **Step 4: 格式化并编译**

```bash
gofmt -w modules/phone/internal/recycle.go
go build ./...
```

Expected: 无错误

- [ ] **Step 5: 跑 phone/recycle 测试**

```bash
go test ./modules/phone/internal/... -count=1 -run TestRecycle 2>&1
```

Expected: PASS all TestRecycle* tests

- [ ] **Step 6: 跑 phone 全部测试**

```bash
go test ./modules/phone/internal/... -count=1 2>&1 | tail -15
```

Expected: ok

- [ ] **Step 7: Commit**

```bash
git add modules/phone/internal/recycle.go
git commit -m "perf(phone): parallel Destroy in runRecycleCleanup; parallel reconcile patrol"
```

---

## Task 8: billing newModelCapacities 三查询并行

**Files:**
- Modify: `modules/billing/internal/account_caps.go`

- [ ] **Step 1: 加 import**

在 `modules/billing/internal/account_caps.go` 文件头部 `package billing` 之后加：

```go
import (
	"fmt"
	"sync"
)
```

- [ ] **Step 2: 替换 `newModelCapacities`**

将整个 `newModelCapacities` 函数替换为：

```go
// newModelCapacities 并行查询三个独立来源，组合新模型账户容量。
func newModelCapacities(userID int) (AccountCapacitiesV2, error) {
	var (
		seat int
		boot int
		mins int64
		errs [3]error
	)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		seat, errs[0] = LicenseService.Capacity(userID, KindSeat)
	}()
	go func() {
		defer wg.Done()
		boot, errs[1] = LicenseService.Capacity(userID, KindBootSlot)
	}()
	go func() {
		defer wg.Done()
		mins, errs[2] = RuntimeWalletService.Remaining(userID)
	}()
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return AccountCapacitiesV2{}, fmt.Errorf("查询账户容量失败: %w", err)
		}
	}
	return AccountCapacitiesV2{Seat: seat, BootSlot: boot, RuntimeMinute: mins}, nil
}
```

- [ ] **Step 3: 格式化并编译**

```bash
gofmt -w modules/billing/internal/account_caps.go
go build ./...
```

Expected: 无错误

- [ ] **Step 4: 跑 billing 测试**

```bash
go test ./modules/billing/internal/... -count=1 2>&1 | tail -15
```

Expected: ok

- [ ] **Step 5: Commit**

```bash
git add modules/billing/internal/account_caps.go
git commit -m "perf(billing): parallel capacity queries in newModelCapacities"
```

---

## 最终验证

- [ ] **全量测试**

```bash
cd /home/root/workspace005/gloryphone-code/backend
go test ./... -count=1 2>&1 | tail -30
```

Expected: 所有包 ok，无 FAIL

- [ ] **Race detector（可选，推荐）**

```bash
go test ./modules/phone/internal/... -count=1 -race 2>&1 | tail -20
go test ./modules/billing/internal/... -count=1 -race 2>&1 | tail -10
```

Expected: 无 DATA RACE

- [ ] **编译最终检查**

```bash
go build ./... && go vet ./...
```

Expected: 无错误，无警告
