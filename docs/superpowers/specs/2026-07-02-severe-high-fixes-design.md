# 设计：修复代码审查报告中的严重 + 高危缺陷

- **日期**：2026-07-02
- **来源**：[docs/code-review-2026-07-02.md](../../code-review-2026-07-02.md)（5 严重 + 9 高）
- **范围**：14 条中排除 **B3**（工作区 CP-0006 已处理其原子性），实际 **13 项修复**，分 7 簇
- **原则**：每项修复配可复现的回归测试（先写失败测试 → 修到绿）；每簇独立提交；全程 `go build ./... && go vet ./... && go test ./...` 通过

## 已确认决策

| 决策 | 选择 |
|---|---|
| 计费范围 | 保留工作区 CP-0006 单事务改动，仅在其上补 **B1/B2** 并发/幂等守卫，不动 B3 |
| RBAC 授权模型 | **最小权限边界**：保留非超管的委派管理，但操作被操作者自身权限收敛 |
| 测试策略 | **每修必配回归测试**（TDD：失败测试先行） |
| SSRF（簇6） | **硬拦私网/环回/链路本地**；内网代理白名单作为后续可配置项（本轮不做） |
| 执行顺序 | 簇1 RBAC → 簇2 billing → 簇3–4 phone/midplat → 簇5–7 framework/SSRF/APK |

## 接线事实（已核实）

- 认证中间件写入 `c.Set("userID", claims.UserID)`（[framework/auth/auth.go:120](../../../backend/framework/auth/auth.go#L120)）。
- `PermissionMiddleware` 对超管 `c.Set("is_superuser", true)`（[modules/staff/internal/middleware.go:71](../../../backend/modules/staff/internal/middleware.go#L71)）。
- staff handler 用 `c.Get("userID")` 取当前操作者（[api.go:72](../../../backend/modules/staff/internal/api.go#L72)）。
- 取角色权限：`roles.permissionsOf(roleID)`；取用户权限并集：`roles.userPermissions(userID)`（均在 [role_service.go](../../../backend/modules/staff/internal/role_service.go)）。
- 计费 `pay/MarkPaid` 已在 CP-0006 工作区里对内建类型走单事务（`withTx`）；`markPaid`（[bizorder_repository.go:66](../../../backend/modules/billing/internal/bizorder_repository.go#L66)）仍是**无守卫** `Update("status", paid)`。

---

## 簇 1 · 后台 RBAC 权限边界（S1/A1/A2/A3/A4/A5）

**目标**：非超管无法通过用户/角色管理接口给自己或他人提权到超出自身权限的等级。

**接线改动**：
- handler（`api.go` 的 `CreateStaff`/`UpdateStaff`/`DeleteStaff`，`role_api.go` 的 `CreateRole`/`UpdateRole`）提取 `actorID := c.GetInt("userID")`、`actorSuper := c.GetBool("is_superuser")`，作为参数传入对应 service 方法。
- service 方法签名新增 `actorID int, actorSuper bool`。非超管分支用 `s.roles.userPermissions(actorID)` 解析操作者权限集（`map[string]bool`）。

**逐项**：

| 项 | 文件:函数 | 修法 |
|---|---|---|
| A1/A2 | service.go `CreateStaffWithForm`(96) / `UpdateStaffWithForm`(134) | 仅 `actorSuper==true` 时才应用请求体的 `is_superuser`；非超管请求里该字段**忽略**（既不置 true 也不由普通用户翻转）。`is_superuser` 发生变化时递增目标用户 `token_version` |
| S1 | role_service.go `Create`(77) / `Update`(99) | 校验合法性后，若 `!actorSuper`：`req.Permissions` 每一项必须 ∈ 操作者权限集，否则 `apperr.Forbidden`（新增或复用 403 映射） |
| A3 | service.go `UpdateStaffWithForm` 处理 `RoleIDs` 处 | 若 `!actorSuper`：对每个 `roleID` 取 `roles.permissionsOf(roleID)`，其集合必须 ⊆ 操作者权限集，否则 403 |
| A5 | service.go `DeleteStaff`(299) | 传入 actor；禁止非超管删除 `IsSuperuser` 目标；禁止 `id==actorID` 自删；删超管前 count 超管数，禁止删末位超管 |
| A4 | service.go `UpdateStaffWithForm` | `is_active` 由 true→false（以及角色变更、超管位变更）时递增 `token_version`（复用 `s.users.bumpTokenVersion(id)`） |

**apperr 依赖**：`apperr.Forbidden`(403) 已存在（[framework/apperr/errors.go:44](../../../backend/framework/apperr/errors.go#L44)，`FailErr` 已映射 403），直接复用。

**回归测试**（`modules/staff/internal/*_test.go`，真实 sqlite）：
- 非超管持 `staff:edit` 传 `is_superuser:true` 更新自己 → 落库仍 `false`。
- 非超管持 `staff:create` 传 `is_superuser:true` 创建 → 新用户 `false`。
- 非超管创建角色含自身没有的权限 → 403；只含自身权限子集 → 成功。
- 非超管给自己赋一个含越权权限的角色 → 403。
- 非超管删超管/自身/末位超管 → 403 或被拒。
- 禁用某员工后其旧令牌 `tv` 与库中不符（`bumpTokenVersion` 生效）。

## 簇 2 · 计费并发/幂等（B1/B2，基于 CP-0006）

**根因**：状态在事务外读 + `markPaid` 无 `WHERE` 守卫 + 无行锁 → 并发 `PayOrder`/`MarkPaid` 同一订单双执行。

**改动**：
- [bizorder_repository.go:66](../../../backend/modules/billing/internal/bizorder_repository.go#L66) `markPaid` 改**状态 CAS**：
  ```
  res := tx.Model(&BizOrder{}).Where("id = ? AND status <> ?", id, BizOrderPaid).Update("status", BizOrderPaid)
  return res.RowsAffected, res.Error
  ```
  接口签名 `markPaid(id int) error` → `markPaid(id int) (int64, error)`。
- `pay()` / `MarkPaid()`（bizorder_service.go，CP-0006 工作区版）：在事务内（内建类型）或顺序流程首步（外部类型）**先执行 CAS 置 paid**：
  - `rows==0` → 订单已被并发赢家处理，直接返回幂等结果（不 charge、不 fulfill）。
  - `rows==1` → 继续 charge + fulfill；内建类型在同事务内，失败整体回滚（状态回 unpaid）。
- 外部注册型路径无事务：CAS 先行仍防双执行；其"已置 paid 但外部 fulfill 失败"的补偿缺口属 CP-0006 既有限制，**不在本轮**（在设计文档"暂不处理"中标注）。

**回归测试**（`modules/billing/internal/*_test.go`）：
- 对同一未支付订单连续两次 `markPaid` → 第一次 `rows==1`，第二次 `rows==0`。
- 两次 `MarkPaid(id)` → 履约只发生一次（席位/时长只发一份），最终 `status==paid`。
- 余额支付：CAS 赢家扣款+履约成功；模拟 fulfill 失败 → 事务回滚，status 回 unpaid、余额未扣。

## 簇 3 · phone status 越权写（I1）

**现状**：`CloudPhoneUpdate` DTO 含 `Status` 字段（[model.go:128](../../../backend/modules/phone/internal/model.go#L128)），`updateFields`（[service.go:451](../../../backend/modules/phone/internal/service.go#L451)）把 `req.Status` 直接写库。phone 是完整状态机（CREATING/RUNNING/STOPPED/RECYCLED…），CLAUDE.md 明确"展示状态以中台实时态为准"——DB 状态应由服务端操作 + 中台同步 + 席位核算共同驱动，**不应由前台通用更新直接改**。

**改动**：`updateFields` **移除 `status` 分支**（不再接受 `req.Status`）。用户电源态变更走专用操作端点（开机/关机/回收，均已同步中台与席位）。`Status` 字段可保留在 DTO 但更新时忽略（或从 DTO 删除，取实现简洁者）。

> **契约变更**：现有测试 [api_test.go:136](../../../backend/modules/phone/internal/api_test.go#L136) 用 `CloudPhoneUpdate{Status: StatusStopped}` 更新并期望 200——该用例编码了缺陷行为。需改为：更新后断言 **status 未被改写**（HTTP 仍 200，但 status 保持原值）。确认前端不依赖"通过通用 update 改 status"（CLAUDE.md 语义支持此改动）。

**回归测试**：前台用户对自己云手机 `Update` 传 `status:"RECYCLED"`（及 `STOPPED`）→ 库中 status 不变；`listNonRecycledByUser` / 席位统计不受影响。

## 簇 4 · 中台失败列表回报（C1）

**现状**：[midplat.go:323](../../../backend/modules/phone/internal/midplat.go#L323) `StartApp` / :329 `StopApp` 适配器用 `_, err := a.c.StartApp(...)` **丢弃了响应**，而响应含失败列表：`StartAppResponse.StartFailedCpIDs`（[cloud_phone_app.go:103](../../../backend/framework/midplat/cloud_phone_app.go#L103)）、`StopAppResponse.StopFailedCpIDs`（[:125](../../../backend/framework/midplat/cloud_phone_app.go#L125)）。

**改动**：`StartApp`/`StopApp` 适配器接收响应，若目标 `cpID` 落在 `StartFailedCpIDs`/`StopFailedCpIDs` 中 → 返回业务错误（`apperr.Validation("启动/停止应用失败: <cpID>")`），不再返回 `nil`。`KillAllApps` 同理——先确认其响应结构是否含失败列表字段（若无结构化失败列表则保持原样，并在实现说明中标注）。

**回归测试**：注入把目标 cp 放进 `StartFailedCpIDs` 的假中台响应（用可 mock 的 `a.c` 接口）→ 适配器返回非 nil error；目标不在失败集 → nil。

## 簇 5 · framework 健壮性/安全（F1/F2）

- **F1**：[framework/scheduler.go:76-89](../../../backend/framework/scheduler.go#L76) `PeriodicRunner` 后台 goroutine 与 [automation worker.go](../../../backend/modules/automation/internal/worker.go) 的 `taskWorker`，每轮 fn 调用包 `defer func(){ if r := recover(); r != nil { log.Printf(...) } }()`，panic 记录后继续下一轮，不再 `os.Exit`/击穿进程。
- **F2**：[framework/config.go:243](../../../backend/framework/config.go#L243) JWT 密钥校验：默认哨兵值 `change-me-in-production` 且 `GIN_MODE=release` → `log.Fatal` fail-fast；非 release 维持 `⚠️` 告警。`UserJWTSecret` 空回退到主密钥的路径同样纳入校验。

**回归测试**：
- F1：注册一个每次都 panic 的周期任务，跑两个 tick → runner goroutine 存活、第二个 tick 仍被调用（用可观察计数器断言未崩）。
- F2：把密钥校验逻辑抽为可测函数 `validateSecrets(cfg, ginMode) error`，release+默认密钥 → 返回 error；dev 或非默认 → nil。

## 簇 6 · proxy 探测 SSRF（S2）

**改动**：[modules/proxy/internal/prober.go:56](../../../backend/modules/proxy/internal/prober.go#L56) 拨号前解析 host → IP，若任一解析 IP 命中**环回 / 链路本地（含 169.254.169.254 元数据）/ RFC1918 私网 / 未指定 / 多播** → 返回校验错误，不拨号。错误信息不回显响应字节（[prober.go:84](../../../backend/modules/proxy/internal/prober.go#L84) 改为通用文案）。用 `net.ParseIP` + `IsLoopback/IsLinkLocalUnicast/IsPrivate/IsUnspecified/IsMulticast` 判定；对域名先 `net.LookupIP` 逐一校验。

> **假设**：代理目标为外网。若未来需内网代理，增加环境变量白名单（本轮不做）。

**回归测试**：`Probe`/`TestProxy` 传 `127.0.0.1`、`169.254.169.254`、`10.0.0.1`、`localhost` → 均返回校验错误、不发起连接；传公网地址 → 通过校验（可用可注入的 dialer stub 避免真实外连）。

## 簇 7 · APK/XAPK 解析上限（O3）

**改动**：
- [modules/app/internal/apkparse/apkparse.go:320](../../../backend/modules/app/internal/apkparse/apkparse.go#L320) `readZipEntry` 与 [:343](../../../backend/modules/app/internal/apkparse/apkparse.go#L343) `scanZipForLauncherIcon`：`io.ReadAll` → `io.ReadAll(io.LimitReader(rc, maxEntryBytes))`，并在读前用 `f.UncompressedSize64` 预判超限拒绝；设总解压量上限。常量：manifest ≤ 1 MiB、icon ≤ 4 MiB、单条目 ≤ 64 MiB、整包解压总量 ≤ 上限（定义为包级常量）。
- [modules/app/internal/service.go:161](../../../backend/modules/app/internal/service.go#L161) `FinalizeUserApp` 拷贝 S3 对象到临时文件时用 `io.LimitReader` 限总大小。

**回归测试**：构造一个声明超大 `UncompressedSize64` 的 zip 条目 → `Parse` 返回错误而非 OOM；正常小包 → 解析成功。

---

## 暂不处理（明确排除）

- **B3**（余额扣款/履约原子性）：CP-0006 已对内建类型单事务化；外部注册型 fulfill 的原子性是既有限制，属 CP-0006 范围。
- 报告中所有 **中/低** 缺陷：本轮只做严重 + 高。
- 簇6 的内网代理白名单：留作后续可配置项。

## 风险与回滚

- RBAC 方法签名变更影响所有调用点（handler + 现有测试）——需同步更新；Modulith 边界不变（仅 `internal/` 内改动）。
- 计费 CAS 与 CP-0006 工作区改动叠加：需先确认 CP-0006 编译通过再叠加。
- 每簇独立提交，便于单簇回滚。

## 验收

- `go build ./... && go vet ./... && go test ./...` 全绿。
- 新增回归测试覆盖上述每一项，且在未修复前能复现失败（TDD 红→绿留痕）。
