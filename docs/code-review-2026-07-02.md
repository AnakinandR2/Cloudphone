# GloryPhone 代码缺陷审查报告

- **日期**：2026-07-02
- **范围**：`backend/`（Go/Gin/GORM，329 文件）、`my/`（C 端前台）、`admin/`（后台管理）、`www/`（Nuxt 营销站 + BFF）
- **方法**：10 路只读 finder 代理并行审查 → 61 条候选 → 逐条对抗式验证（读真实代码尽力证伪、跨文件核查兜底）→ 补漏扫描 → 校准分级
- **结论**：`git` 未改动，**仅审查未修复**（遵嘱"暂时不修"）。共确认 **52 条真实缺陷**，另澄清 **13 条误报**（见文末）。

> 验证阶段做了大量跨文件核查，纠正了 finder 阶段若干过高定级。例如"面包屑 JSON-LD 存储型 XSS"经查 unhead 框架已中和 `</script`（误报）；"X-API-Key 明文落库"经查开放 API/MCP 挂在引擎根、根本不经 `/api/v1` 的访问日志中间件（误报）；"开放 API 无限流可计费放大"经查建云机被已购席位硬顶 409（误报）。这些已从缺陷列表剔除。

---

## 一、严重度与分类统计

| 严重度 | 数量 | 说明 |
|---|---|---|
| 🔴 严重 | 5 | 可直接提权 / 资金重复扣款 / 弱默认密钥可伪造任意令牌 |
| 🟠 高 | 9 | 授权失效 / 数据不一致 / 静默失败 / SSRF / 内存击穿 |
| 🟡 中 | 18 | 需特定并发/配置/环境触发，或影响可用性与一致性 |
| ⚪ 低 | 20 | 纵深防御缺失、健壮性、体验、观测性 |
| **合计** | **52** | |

| 分类 | 数量 |
|---|---|
| 安全漏洞 | 18 |
| 数据一致性 / 并发竞态 | 13 |
| 健壮性 | 14 |
| 功能缺陷 | 5 |
| 配置 | 2 |

### 主题聚类（建议按主题成批修复）

1. **后台 RBAC 缺"权限边界"强制**（S1/A1/A2/A3/A4/A5）——`staff` 侧多个写接口只校验"是否持有某权限"，不校验"操作者是否有资格授予该等级"，构成完整的垂直提权链。**这是本次最需要优先处理的一类。**
2. **计费缺原子性与幂等**（B1/B2/B3/B4/B5/B6）——支付、履约、结算、试用领取普遍是"读—判—写"跨事务、无唯一约束/行锁/状态守卫；SQLite 串行掩盖，切 MySQL/PG 或接真网关即暴露。
3. **中台交互"失败静默"**（C1/C2/C6）——适配器丢弃中台返回的失败列表/差集，把部分失败当成功。
4. **前端请求层把非 JSON 响应当成功**（D1/M2）——mock 失效或代理回落 SPA HTML 时静默空白。

---

## 二、🔴 严重（5）

### S1 · 后台角色可被低权账号铸造任意高权限并自赋 → 垂直提权
- **位置**：[backend/modules/staff/internal/role_service.go:77](backend/modules/staff/internal/role_service.go#L77)（Create/Update）
- **分类**：安全漏洞
- **触发**：仅持 `role:create`/`role:edit`（+`staff:edit`）的低权运营，`POST /api/v1/staff/role/create` 传 `permissions:["staff:delete","billing:manage","user:manage"]`（全是合法 key，`IsValidPermission` 通过），再 `PUT /staff/update/{自己id}` 把该角色赋给自己。
- **影响**：拿到自己本无权的删员工/调余额/管用户等能力，完成越权。`RoleService.Create/Update` 全程无"操作者是否拥有所授权限"的收敛。
- **建议**：创建/更新角色时，把 `permissions` 交集限制在操作者自身权限集内（超管除外）；或对"高危权限点"要求超管。

### A1 · `staff:edit` 可把任意用户（含自己）`is_superuser` 置 true → 提权
- **位置**：[backend/modules/staff/internal/service.go:157](backend/modules/staff/internal/service.go#L157)
- **分类**：安全漏洞
- **触发**：持 `staff:edit` 的非超管 `PUT /api/v1/staff/update/{自己id}` 传 `{"is_superuser":true}`；`updates["is_superuser"]=*req.IsSuperuser` 原样落库（`repository.go:98` `Updates(fields)` 无白名单）。
- **影响**：下次请求 `IsSuperuser=true`，权限变 `["*"]`，全站超管。**且改超管位不递增 `token_version`**，无需重登即生效。测试 `user_service_test.go:134` 反而断言了该字段可写。
- **建议**：`is_superuser` 移出普通更新 DTO；仅超管可改，且改动即 `bumpTokenVersion`。

### A2 · `staff:create` 可在创建时直接设 `is_superuser=true` → 造超管
- **位置**：[backend/modules/staff/internal/service.go:115](backend/modules/staff/internal/service.go#L115)
- **分类**：安全漏洞
- **触发**：持 `staff:create` 的非超管 `POST /api/v1/staff/create` 传 `{"username":"x","password":"...","is_superuser":true}`，`CreateStaffWithForm` 原样落库。
- **影响**：登录该新账号即超管。与 A1 是同一"缺字段白名单"根因的两个入口。
- **建议**：创建时忽略/拒绝请求体的 `is_superuser`（仅超管可指定）。

### B1 · 订单支付并发下重复扣款 + 重复履约
- **位置**：[backend/modules/billing/internal/bizorder_service.go:213](backend/modules/billing/internal/bizorder_service.go#L213)（`PayOrder`/`pay`）
- **分类**：并发竞态
- **触发**：对同一未支付订单并发两次 `POST /billing/orders/:id/pay`。两请求都读到 `status=unpaid` 并通过 `o.Status==BizOrderPaid` 检查 → 各自 `wallet.Charge` 扣两遍钱 → 各自 `fulfillOrder` 发两份货；`markPaid`（`bizorder_repository.go:66`）是无条件 `Update("status", paid)`，**无 `WHERE status=unpaid` 守卫**。
- **影响**：一次下单扣两次钱、发两份货。余额支付与第三方桩支付皆受影响。
- **建议**：`markPaid` 加乐观状态守卫（`WHERE status='unpaid'`，受影响行数=0 即中止）；扣款+履约+置 paid 收敛到单事务或引入订单级幂等键。

### F2 · `JWT_SECRET` 弱默认值仅告警不阻断 → 可伪造任意令牌
- **位置**：[backend/framework/config.go:176](backend/framework/config.go#L176)（默认 `change-me-in-production`，`config.go:243` 仅 `log.Println`）
- **分类**：安全漏洞（PLAUSIBLE：取决于部署是否漏配）
- **触发**：生产漏配 `JWT_SECRET` 环境变量时，服务用公开常量验签/签发。`UserJWTSecret` 默认空还会回退到它，`crypto` 也用它派生密钥。
- **影响**：攻击者用已知密钥离线签发 `scope=staff`、`user_id=<超管id>`、`tv=0` 的令牌冒充后台超管（若目标 `tv` 恰为 0 连版本校验也过）。
- **建议**：生产（`GIN_MODE=release`）下使用默认密钥应 **fail-fast** 拒绝启动。

---

## 三、🟠 高（9）

### A3 · `staff:edit` 可通过 `role_ids` 给任意用户绑定任意角色
- **位置**：[backend/modules/staff/internal/service.go:174](backend/modules/staff/internal/service.go#L174)
- **触发**：`PUT /staff/update/{任意id}` 传 `role_ids:[高权限角色]`，`updateWithRoles`（`repository.go:98`）无条件清空目标角色再插入任意角色，无归属/自我/权限边界校验。与 S1 组合即完整提权链（前提是已持 `staff:edit` 管理员级权限，故为高而非严重）。

### A4 · 禁用员工（`is_active=false`）不失效令牌
- **位置**：[backend/modules/staff/internal/service.go:154](backend/modules/staff/internal/service.go#L154)
- **触发**：管理员禁用某员工，但仅改密分支才 `bumpTokenVersion`；鉴权链（`AuthMiddleware`/`PermissionMiddleware`）**全程不校验 `IsActive`**。
- **影响**：被禁用员工用手里未过期令牌（默认最长 15 天）可继续访问所有已授权接口。对照前台 `user.SetActive` 已正确递增版本——此处是明显遗漏（`userInfoAdapter.IsActive()` 在鉴权链里是死代码）。
- **建议**：禁用即 `bumpTokenVersion`；或中间件增 `IsActive` 校验。

### B2 · 支付回调 `MarkPaid` 非幂等 → 重复回调重复发放
- **位置**：[backend/modules/billing/internal/bizorder_service.go:235](backend/modules/billing/internal/bizorder_service.go#L235)
- **触发**：网关重试或后台重复点 `mark-paid`，两并发回调都读到非 paid，各 `fulfillOrder` 一次（重复发席位/翻倍充值+赠时长）。当前唯一活跃调用是后台手动按钮、第三方网关尚为桩（代码注释预告接真网关后走此回调重试），故并发窗口现实存在。
- **建议**：同 B1，状态守卫 + 履约幂等键。

### B3 · 余额支付"扣款成功后履约失败"扣钱不发货
- **位置**：[backend/modules/billing/internal/bizorder_service.go:216](backend/modules/billing/internal/bizorder_service.go#L216)
- **触发**：`wallet.Charge` 在自身事务提交扣款后，`fulfillOrder` 报错 `return`，已提交扣款不回滚、`markPaid` 未执行、无补偿/退款。三步分属不同事务，任一步失败留下不一致。
- **建议**：扣款+履约+置 paid 单事务；失败整体回滚或补偿冲正。

### I1 · 前台可 mass-assignment 写 `status` 绕过席位计费
- **位置**：[backend/modules/phone/internal/service.go:457](backend/modules/phone/internal/service.go#L457)（`CloudPhoneUpdate`）
- **触发**：用户 `PUT /phone/update/{自己phoneId}` 传 `{"status":"RECYCLED"}`，被 `listNonRecycledByUser` 排除 → `checkSeatAvailable` 少算一台 → 可创建超过已购席位数的云手机，而中台实例仍在运行、从未真正回收。
- **影响**：计费绕过 + 幽灵实例。
- **建议**：`status` 移出用户可写字段（由服务端状态机驱动）。

### O3 · APK/XAPK 解析压缩炸弹 → 内存击穿 OOM
- **位置**：[backend/modules/app/internal/apkparse/apkparse.go:320](backend/modules/app/internal/apkparse/apkparse.go#L320)
- **触发**：经素材库上传体积小但含超大解压条目的 `.xapk/.apk`；`readZipEntry`/`scanZipForLauncherIcon` 对条目直接 `io.ReadAll`，无 `LimitReader`/大小上限，单包让解析 goroutine 申请数 GB 内存。`ensureFinalized` 还会异步自愈重试，可被反复触发。
- **建议**：对每个 zip 条目与整包加 `io.LimitReader` 与总量上限，超限拒绝。

### C1 · 单台启停/杀应用丢弃中台失败列表 → 失败当成功
- **位置**：[backend/modules/phone/internal/midplat.go:323](backend/modules/phone/internal/midplat.go#L323)（`StartApp`/`StopApp`/`KillAllApps`）
- **触发**：单台调用目标 cp 失败时，中台仍返回 HTTP 200 并把该 cp 放进 `StartFailedCpIDs`/`Containers`（失败列表），无 Go error。适配器 `_, err := ...; return err` 只透传传输错误、忽略失败列表。
- **影响**：handler 返回 200"成功"，但应用实际未启动/停止，用户无从察觉。
- **建议**：适配器检查失败列表，非空即返回业务错误。

### F1 · 调度器后台 goroutine 无 `recover` → 单次 panic 击穿整个进程
- **位置**：[backend/framework/scheduler.go:84](backend/framework/scheduler.go#L84)
- **分类**：健壮性（PLAUSIBLE：需 fn 某次 tick 真 panic）
- **触发**：任一周期任务 fn（结算/清理/巡检等）因空指针/越界 panic。`PeriodicRunner.Start` 的裸 `go func` 与 `TryRunLocked` 内 `return true, fn()` 均无 `recover`；`gin.Recovery` 只覆盖 HTTP 请求 goroutine，不覆盖这些独立后台 goroutine（全仓非测试代码 0 处 `recover`）。`worker.go:24` 的 `taskWorker` 同样裸奔。
- **影响**：一次未捕获 panic 连带 HTTP 服务 + 全部 cron 一起终止，HA 下反复重启。
- **建议**：后台 goroutine 统一包 `defer recover()`（记录并继续下轮）。

### S2 · 代理探测接口 = 内网端口探测 / SSRF 原语
- **位置**：[backend/modules/proxy/internal/prober.go:56](backend/modules/proxy/internal/prober.go#L56)
- **触发**：任一登录前台用户 `POST /api/v1/proxy/probe` 传 `{host:"127.0.0.1",port:6379}`（或内网 DB/元数据地址）。服务端 `xproxy.SOCKS5` 对该 addr 发起真实 TCP 连接握手，据连通/超时/拒绝差异（连同 `err.Error()` 回显的响应字节，`prober.go:84`）判断内网 `host:port` 是否开放。
- **影响**：探测防火墙后方服务，无 host 白名单/私网地址拦截。
- **建议**：拒绝私网/环回/元数据地址；错误信息不回显响应字节。

---

## 四、🟡 中（18）

| ID | 位置 | 分类 | 触发与影响 | 建议 |
|---|---|---|---|---|
| A5 | [staff/service.go:299](backend/modules/staff/internal/service.go#L299) | 安全 | 持 `staff:delete` 可删超管/自身，无末位超管保护 | 加目标超管/末位/自删守卫 |
| I2 | [phone/service.go:463](backend/modules/phone/internal/service.go#L463) | 功能(BOLA) | Create/Update 绑定 `proxy_id` 不校验属主与存在性，可绑他人/悬空代理 | 校验 proxy 归属+存在 |
| B4 | [billing/runtimeengine_service.go:75](backend/modules/billing/internal/runtimeengine_service.go#L75) | 并发 | HA 多实例下 `Settle` 读-算-写非原子，可重复扣临时时长（`RuntimeCharge` 无唯一约束） | 唯一约束 `(session,window)` + 条件写 |
| B5 | [billing/trial_repository.go:157](backend/modules/billing/internal/trial_repository.go#L157) | 安全 | COUNT-then-INSERT，`TrialClaim(policy_id,user_id)` 仅普通索引，MySQL/PG 并发可超领白嫖 | 加 `uniqueIndex` |
| B6 | [billing/bizorder_service.go:115](backend/modules/billing/internal/bizorder_service.go#L115) | 健壮性 | 充值金额只判 `>0` 无上限，桩支付即时置 paid 可自助入账天文余额，累加可溢出 | 加金额上限校验 |
| F3 | [framework/setup.go:51](backend/framework/setup.go#L51) | 并发 | Postgres 建库在咨询锁外，HA 误配多 `STATEFUL` 实例并发建库启动崩溃 | 建库纳入互斥或 fail-fast 更明确 |
| F4 | [framework/probes.go:55](backend/framework/probes.go#L55) | 安全 | 默认无鉴权，外部反复 `GET /debug/panic` → 日志洪泛/CPU 抖动（进程不崩，返 500） | 生产给 `/debug/*` 组加中间件或开关 |
| O5 | [accesslog/middleware.go:96](backend/modules/accesslog/internal/middleware.go#L96) | 功能 | `logChan`(1024) 满即 `select default` 静默丢日志，高峰期审计成片丢失且无计数/告警 | 丢弃计数 + 告警指标 |
| O6 | [openapi/key_api.go:86](backend/modules/openapi/internal/key_api.go#L86) | 安全 | `RevealKey/CreateKey` 明文密钥在响应体；开 `ACCESS_LOG_USER_ENABLED` 后明文落 `response_body`（脱敏白名单缺 `fullKey`） | 脱敏白名单补 `fullKey`/`token` |
| C2 | [automation/worker.go:78](backend/modules/automation/internal/worker.go#L78) | 数据一致性 | 整批错误静默 `return`；中台永久清理的 id 对应本地任务永不标终态成僵尸 | 分批+差集回报+兜底 reaper |
| C5 | [phone/push_from_library.go:60](backend/modules/phone/internal/push_from_library.go#L60) | 健壮性 | 全量读文件入内存仅限个数(≤10)不限字节，10×数百 MB 素材单请求占数 GB → OOM | 限总字节/流式转发 |
| W2 | [www/.../feedback.post.ts:19](www/server/routes/_content/feedback/[slug]/feedback.post.ts#L19) | 健壮性 | 留言写端点无长度/频率限制，可高频灌垃圾（大 body 受 nginx 默认 1MB 约束） | www 层加长度+速率限制 |
| W4 | [www/server/utils/content.ts:79](www/server/utils/content.ts#L79) | 安全 | 取 `x-forwarded-for` 首段转发上游，前置 nginx 若未强制重写则客户端可伪造真实 IP | 只信任可信代理链的 XFF |
| D1 | [admin/src/api/index.ts:70](admin/src/api/index.ts#L70) | 健壮性 | mock 失效/代理回落 SPA HTML(200) 时把 HTML 当 `ApiResult`，视图静默空白 | 校验 `content-type`/`res.code` 类型，非法即报错 |
| M2 | [my/src/api/index.ts:84](my/src/api/index.ts#L84) | 健壮性 | 同 D1（my 前台） | 同上 |
| D4 | [admin/src/directives/auth.ts:16](admin/src/directives/auth.ts#L16) | 健壮性 | 函数式 `v-auth` 每次 `updated` 新建 `watch` 从不停止，37+ 处累积泄漏 watcher | 改对象式指令 + `unmounted` 清理 |
| S3 | [framework/middleware.go:26](backend/framework/middleware.go#L26) | 安全 | `CORS_ALLOWED_ORIGINS` 未配（默认）时回显任意 Origin + `Allow-Credentials:true`，反射式凭证 CORS 反模式 | 默认收紧为白名单；无配置不发凭证 |
| S4 | [proxy/service.go:86](backend/modules/proxy/internal/service.go#L86) | 健壮性 | `proxy/user/staff/role` 列表未钳制 `size`，`?page=0&size=-1` 使 GORM `Limit(-1)` 拉全表（其余模块均有 `size<1||size>200` 钳制） | 统一钳制分页参数 |

---

## 五、⚪ 低（20，纵深防御 / 健壮性 / 体验 / 观测）

| ID | 位置 | 分类 | 摘要 |
|---|---|---|---|
| A6 | [framework/auth/auth.go:55](backend/framework/auth/auth.go#L55) | 安全 | `Parse` 未 `WithValidMethods` 限定算法；现全 HS256 暂不可利用，引入非对称公钥前应加固 |
| A7 | [staff/api.go:30](backend/modules/staff/internal/api.go#L30) | 安全 | 登录用户名不存在时短路跳过 bcrypt，响应时序侧信道可枚举用户名（无 dummy-hash 兜底） |
| F5 | [framework/setup.go:76](backend/framework/setup.go#L76) | 健壮性 | 咨询锁 `RELEASE` 错误被忽略；`GET_LOCK` 返回 NULL 被当 0，与真超时无法区分 |
| F6 | [framework/notification.go:53](backend/framework/notification.go#L53) | 健壮性 | 企业微信 `result["errmsg"].(string)` 无 `ok` 保护，errmsg 缺失/为 null 时 panic（告警路径反被放大） |
| F7 | [framework/config.go:314](backend/framework/config.go#L314) | 配置 | `getEnvAsInt/Bool` 对非法值 `log.Fatalf` 直接终止；与 `getEnvAsDuration` 的回退策略不一致 |
| B7 | [partner/service.go:137](backend/modules/partner/internal/service.go#L137) | 一致性 | `RecordClick` 落明细与自增 `click_count` 分两次独立写无事务，中途失败统计不一致 |
| O2 | [mcp/module.go:34](backend/modules/mcp/internal/module.go#L34) | 安全 | MCP 端点缺传输层 401（每个工具内部 `auth` 已兜底、密钥 192 bit 不可爆破），纵深防御差异 |
| O7 | [app/service.go:161](backend/modules/app/internal/service.go#L161) | 健壮性 | `os.CreateTemp` 模板拼入用户可控扩展名，异常字符致临时文件创建异常 |
| C3 | [automation/task_service.go:140](backend/modules/automation/internal/task_service.go#L140) | 健壮性 | `TaskDetail` 回写本地状态 `_ = updateTaskStatus(...)` 吞错误，本地与中台可静默偏离 |
| C6 | [phone/install_by_url.go:81](backend/modules/phone/internal/install_by_url.go#L81) | 健壮性 | 批量按 URL 安装不做"请求 cpIDs vs 返回 InstanceID"差集，部分手机未安装无回报 |
| C7 | [phone/op_api.go:694](backend/modules/phone/internal/op_api.go#L694) | 健壮性 | `FileUpload` 的 `folderPath` 不校验绝对/相对合法性，越界路径透传中台 |
| C8 | [framework/midplat/client.go:66](backend/framework/midplat/client.go#L66) | 安全 | AKSK 签名不覆盖 body 且无防重放窗口，链路可篡改时 body（如 downloadUrl）可被改而签名仍有效 |
| W5 | [www/composables/useBlog.ts:42](www/composables/useBlog.ts#L42) | 健壮性 | 客户端 `useFetch` 直接拼 `slug` 未 `encodeURIComponent`，含 `?/#/空格` 时 URL 污染（`useHelp.ts:18` 同） |
| W6 | [www/.../summary.get.ts:9](www/server/routes/_content/feedback/[slug]/summary.get.ts#L9) | 配置 | 含 `visitor_id` 的个性化响应未设 `Cache-Control: private/no-store`，前置共享缓存可串号 |
| D2 | [admin/src/router/guards.ts:27](admin/src/router/guards.ts#L27) | 性能 | 零权限合法账号 `permissions.length===0` 恒真，每次导航都重复拉 `/auth/me`（缺 `loaded` 标志） |
| D7 | [admin/src/stores/user.ts:52](admin/src/stores/user.ts#L52) | 一致性 | `getInfo` 只回写部分字段，不更新 `account`、不持久化 `permissions`、清空头像不生效 |
| D8 | [admin/src/router/guards.ts:53](admin/src/router/guards.ts#L53) | 功能 | 大量叶子路由未设 `meta.auth`，任意登录用户手输 URL 可达（均为无 API 调用的静态展示页） |
| M3 | [my/src/views/auth/RegisterView.vue:25](my/src/views/auth/RegisterView.vue#L25) | 功能 | 登录/注册无手机号格式与密码非空前端校验（后端已强制校验，仅缺体验/纵深） |
| M4 | [my/src/api/index.ts:92](my/src/api/index.ts#L92) | 并发 | 错误拦截器先判 `config.retry` 再进 401，带 `retry` 请求会带失效 token 重试后才清会话（当前无 `retry:true` 调用，潜伏） |
| M5 | [my/src/router/guards.ts:25](my/src/router/guards.ts#L25) | 并发 | `loaded` 判断与 `getInfo` 间无并发保护，并行导航各发一次 `getInfo`，任一失败可误清会话 |

---

## 六、已排除 / 误报澄清（13）

以下条目在 finder 阶段被提出，但验证阶段经跨文件核查**证伪或确认已被别处兜底**，不计入缺陷：

| 原判 | 位置 | 证伪结论 |
|---|---|---|
| 严重·X-API-Key 明文落库 | accesslog/sanitize.go:22 | 访问日志中间件只挂 `/api/v1`；开放 API/MCP（唯二 X-API-Key 消费方）在引擎根，**不经该中间件**，不落库 |
| 高·面包屑 JSON-LD 存储型 XSS | www/Breadcrumb.vue:29 | `unhead` SSR 序列化 script innerHTML 时对 `</script` 做 `<\/script` 中和，注入不执行 |
| 高·开放 API 无限流可计费放大 | openapi/module.go:35 | 建云机被"已购席位数"服务端硬顶（超额 409），无法无节制批量创建 |
| 严重·my 认证路径 user vs customer 不符 | my/api/modules/user.ts:14 | 后端前缀实为 `/user`（`backend/CLAUDE.md` 亦然），是 `my/CLAUDE.md` 文档陈旧；契约正确 |
| 低·my 停用账号仍可进 | my/stores/user.ts:47 | 后端禁用即递增 `token_version`，旧令牌打任何前台接口立即 401，守卫强制登出 |
| 中·admin 登录开放重定向 | admin/LoginView.vue:37 | `router.push(字符串)` 恒同源拼 `origin+base+path`，恶意绝对 URL 落本站 not-found，无跨域跳转 |
| 中·admin `v-auth` CSS 隐藏可绕过 | admin/directives/auth.ts:13 | 所有 staff 写接口有服务端 `PermissionMiddleware` 兜底（403），绕过前端隐藏无法越权 |
| 中·admin 401 retry 带失效 token | admin/api/index.ts:59 | 全仓无 `retry:true`，401 一律立即登出；后端鉴权失败返真 401 不包成 200 |
| 中·reaction 无范围校验 | www/reaction.post.ts:19 | 外部内容中台契约强制 `rating 1–5`/`vote ±1` 且返 400，BFF 透传 4xx |
| 低·开放 API scope 未 set 意外持久化 | openapi/middleware.go:27 | 同 X-API-Key：开放 API 不经 `/api/v1` 访问日志中间件 |
| 中·number→int 截断小数 | automation/params.go:437 | 中台 `"int"` 仅为"裸注入不加引号"形态标签，非取整；`"3.5"` 裸替为合法 Lua float，无截断 |
| 低·DSN 明文密码落日志 | framework/db.go:43 | 锁定的 pgx/mysql 驱动连接错误已内建口令脱敏或不含 DSN，无落盘路径 |
| 低·手续费少收一分 | billing/pricing.go:77 | 标准四舍五入到分，tie 向上进位偏商家，无系统性少收 |

---

## 七、验证确认为"安全/正确"的关键面（复盘参考）

- **属主隔离（IDOR）**：`note/library/proxy/phone` 数据层强制 `user_id=?`；`app` 属主校验收敛在 `library` 门面。未发现横向越权。
- **SQL 注入**：排序统一走 `query.SafeOrder(白名单,默认)`（白名单为小写列名，输入须精确等值，无大小写/空格绕过）；生产代码无字符串拼接 SQL。
- **前台 user 侧令牌失效**：登出/禁用/改密均正确递增 `token_version`。
- **cloudphone / example 模块**：只读透传、正确挂 RBAC 权限，无误暴露到匿名/前台。
- **library/app S3**：私有桶 presigned + 合理 TTL，无公有读回退。
- **www 路径穿越/SSRF**：`[slug]` 均作为 query 或已 `encodeURIComponent` 的路径段发往受信内容中台，非文件系统、非用户可控目标主机。

---

## 八、修复优先级建议

1. **P0（本周）**：S1 / A1 / A2 —— 关闭后台 RBAC 提权链（字段白名单 + 权限交集收敛）。
2. **P0**：B1 / B2 / B3 —— 支付与履约加状态守卫、单事务、幂等键（接真网关前必须）。
3. **P1**：A4（禁用即失效令牌）、I1（`status` 移出可写）、F2（弱密钥 fail-fast）、C1（失败列表回报）。
4. **P1**：O3（zip 上限）、S2（SSRF 私网拦截）、F1（后台 goroutine `recover`）。
5. **P2**：中/低批量按主题清理（分页钳制、CORS 收紧、前端拦截器 content-type 校验、观测性）。

> 报告基于当前 `main`（`b8e0d3e`）静态审查；并发/HA 类问题建议切到 MySQL/PG 并发压测复现验证后再定最终优先级。
