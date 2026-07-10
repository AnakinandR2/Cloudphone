# TC-16 中危补测与 apptest 补齐（Plan D · 可自动化）

> 覆盖：Plan A 未纳入的中/低危回归（C6 按 URL 安装差集回报、B4 运行时结算并发重复扣、B6 充值无上限、O6 明文密钥落访问日志）+ 素材库/自动化的端到端 apptest 补齐（TC-11/TC-14 flagged 缺口）+ 低危纵深防御（A7/C7/S3）。· 关联：`docs/code-review-2026-07-02.md`（缺陷编号/文件行来源）、`docs/superpowers/specs/2026-07-02-上线验收测试计划-design.md`（分层判据/安全回归矩阵）、TC-10（计费）、TC-11（素材库）、TC-12（按 URL 安装）、TC-14（自动化）；后端 `backend/modules/*/internal/*` 与 `backend/apptest/*`。

## 定位与判据

本文档是 Plan D（可自动化补齐层）的用例库，专治两类缺口：

1. **中/低危回归**——code-review 确认的真实缺陷里，Plan A（apptest 幂等/隔离主线）未固化的项。每条注明是**回归防护（应通过）**还是**发现待修（可能先失败→修生产码再转正向断言）**。
2. **素材/自动化 apptest**——TC-11/TC-14 明确 flagged「Plan A 未新增专门 apptest」的两块，给出经公开 HTTP + `framework.DB` 的端到端骨架。

> **分层判据**（承总纲）：能只用 HTTP+DB 复现 → 接口层；必须看渲染/浏览器 → Playwright（Plan C）；必须真机/人眼 → 人工 runbook（Plan D）。本文档全部落在**可自动化**一侧（模块 internal 单测 或 apptest HTTP 端到端），故归 Plan D「可自动化清单」。

> **注入方式约定**：中台失败注入有两种接缝——(a) **fakePort**（实现 `midplatPort` 接口的假 ops，直接注入 `Service.ops`，见 `phone/internal/ops_test.go`）；(b) **fake 中台 HTTP 服务**（`httptest.NewServer` + `midplat.New{BaseURL:srv.URL}`，走真实 `sdkAdapter` 解析，见 `phone/internal/midplat_failedlist_test.go`）。C6 用 (a) 更简；O6/apptest 用真实路由 + DB 断言。

---

## 一、C6 · 按 URL 安装差集回报（phone/internal · install_by_url.go）

> **现状**：`InstallByURL`（`backend/modules/phone/internal/install_by_url.go:35`）逐台属主校验收集 `cpIDs`，一次 `ops.InstallAppByURL({cpIds, apps})` 下发，然后**只**把返回的 `res.TaskInfoList` 逐条透出为 `AppInstallTask`（`install_by_url.go:86-91`）。它**从不比对**「请求的 `cpIDs`」与「返回 taskInfoList 覆盖到的 InstanceID 集合」——若中台只受理了部分手机（其余 cp 在 `taskInfoList` 里缺席），未受理手机**无任何回报**，handler 仍返回受理成功的那几台，用户误以为全部下发。对应 review **C6**（`phone/install_by_url.go:81`，健壮性）。
>
> **失败注入**：fakePort 的 `installByURLResp` 只填部分手机的 `taskInfoList`（如请求 cp-x/cp-y，只回 cp-x）。断言 service 应能识别 cp-y 未受理并回报（当前实现**做不到** → 用例先 FAIL → 记为待修 C6）。

| 用例ID | 标题 | 类型 | 优先级 | 前置条件 | 步骤 | 预期结果 |
| --- | --- | --- | --- | --- | --- | --- |
| TC-16-001 | 【发现待修·C6】部分受理应回报未受理手机 | 集成 | P1 | phone/internal 单测；`withFakeOps` 注入 `fakePort`、`withStubInstallSpecs` 桩安装载荷；`provisionedPhone(userA,"cp-x")`、`provisionedPhone(userA,"cp-y")` | 注入 `installByURLResp = {TaskInfoList:[{TaskID:"tk-1",InstanceID:"cp-x"}]}`（**缺 cp-y**）；调 `PhoneService.InstallByURL(userA,[idX,idY],refs)` | **应**：返回结果能区分「cp-x 已受理 / cp-y 未受理」（未受理集合非空或整体带 partial 标志）。**当前实现**：只返回 `tk-1`（cp-x），cp-y 静默丢失、无差集 → 用例 **FAIL**。修复方向：`InstallByURL` 对 `cpIDs \ {t.InstanceID}` 求差集，非空即回报未受理列表（参照 C1 单台失败列表回报的处理范式） |
| TC-16-002 | 【发现待修·C6】全空 taskInfoList 应整体判失败 | 集成 | P1 | 同上 | 注入 `installByURLResp = {TaskInfoList:[]}`（中台一台未受理） | **应**：所有 cp 落入未受理差集 → 返回业务错误或全 partial-failed。**当前实现**：返回空 `tasks`、`err=nil`，handler 视作「200 无任务」的成功 → 用例 **FAIL**（记为待修） |
| TC-16-003 | 【回归防护·C6】全受理时差集为空、逐台透出 | 集成 | P1 | 同上 | 注入 `installByURLResp` 覆盖 cp-x+cp-y 两条 taskInfoList | 差集为空、无回报噪声；`tasks` 含两台 `TaskID/InstanceID`（复用现有 `TestInstallByURLMapsRequestAndReturnsTasks` 的透出断言，作为 C6 修复后的正向锚点，防修复引入误报） |
| TC-16-004 | 【回归防护·C6】前置属主/解析失败仍不触达中台 | 安全 | P0 | 同上；`userB` 不拥有该手机 | `InstallByURL(userB,[idX],refs)`；另测 `resolveInstallSpecs` 返回 error | 整请求前置失败、`f.calls==0`、`resolved==false`——差集逻辑不得改变「前置失败零副作用」不变量（已由 `TestInstallByURLOwnershipFailsBeforeDispatch`/`TestInstallByURLResolveErrorPropagates` 覆盖，此处引用作 C6 修改的护栏） |

**可选 apptest（TC-16-005）**：登录前台 → 建/开通 2 台云机（依赖中台，apptest 默认 `ops=nil`，需 fake 中台或跳过）→ `POST /api/v1/phone/apps/install-by-url {phone_ids,apps}` → 断言响应体含未受理手机列表。因 apptest 默认中台未配置，本项作**可选 HTTP 端到端**，优先以 TC-16-001~004 的 internal 单测为准。

---

## 二、B4 · 运行时结算并发/重复重扣（billing/internal · runtimeengine_service.go）

> **现状**：`Settle`（`backend/modules/billing/internal/runtimeengine_service.go:24`）是「读 `sessionSettled(ref)` → 算 `newMin=totalMin-settled` → 写 `insertCharge` + `bumpSessionSettled(绝对值)`」的**读-算-写**序列，且跨多次独立 DB 操作（非单事务包裹）。幂等游标 `RuntimeSessionProgress`（主键 `RunSessionRef`）在**单实例串行**下靠 `sessionSettled` 挡住重扣（TC-10-064 已覆盖）；但 **HA 多实例并发** `Settle` 同一 `(session,window)` 时：两实例都读到 `settled=0` → 都算出同一 `newMin` → 都 `insertCharge`（`billing_runtime_charges` **无 `(RunSessionRef,WindowStart)` 唯一约束**，见 `runtimecharge_model.go:20`，仅有普通 index `idx_rtc_session`）→ 都把游标 bump 到同一绝对值。结果：**临时时长被扣两遍**、余额流水两条，而游标看似正常。对应 review **B4**（`billing/runtimeengine_service.go:75`，并发）。
>
> **失败注入**：sqlite 串行掩盖并发窗口，故用**重复调用**模拟并发赢家/重放竞态——先注入桩 `sessionSettled` 恒返回旧值（模拟第二实例读到未推进的游标），或直接绕过游标推进重放同窗口，断言 `insertCharge`/`consume` 只应生效一次。

| 用例ID | 标题 | 类型 | 优先级 | 前置条件 | 步骤 | 预期结果 |
| --- | --- | --- | --- | --- | --- | --- |
| TC-16-010 | 【发现待修·B4】重复 Settle 同窗口只扣一次时长 | 集成 | P0 | billing/internal 单测（真实 sqlite）；`FulfillRuntimePack(uid,1000)` 造临时时长；`cleanRuntime` 清理 | 用固定 `intervals`（同 `RunSessionRef=sess1`、同 `Start`）与固定 `windowEnd`，在**游标不推进的前提下并发/重放** `Settle`（模拟两 HA 实例读到 `settled=0`）：即注入 `sessionSettled` 桩恒返 0 两次调用 | **应**：`billing_runtime_charges` 中该 `(sess1,windowStart)` 仅 1 条 `temp` charge、余额只扣一次。**当前实现**：无唯一约束 → 落 2 条、扣 2 次 → 用例 **FAIL**。修复方向：`RuntimeCharge` 加 `uniqueIndex(RunSessionRef,WindowStart)` + `insertCharge` 用 `ON CONFLICT DO NOTHING`（条件写），并把「读游标→写 charge→bump」收敛进单事务/条件更新（`bumpSessionSettled` 改 CAS `WHERE settled_minutes = 旧值`） |
| TC-16-011 | 【发现待修·B4】唯一约束存在性守卫 | 集成 | P0 | 同上 | 直接 `insertCharge` 两条相同 `(RunSessionRef,WindowStart,QuotaType)` 的 `RuntimeCharge` | **应**：第二条因唯一索引冲突被拒（返回冲突错误或幂等吞掉）。**当前实现**：两条都落库 → 用例 **FAIL**（DDL 守卫：修复后 `AutoMigrate` 应建出该唯一索引，此用例转为「第二条冲突」的正向断言） |
| TC-16-012 | 【回归防护·B4】正常单实例连续 tick 幂等不重扣 | 集成 | P0 | 同上 | 同一窗口 `Settle` 两次（游标正常推进）；再推进 `windowEnd` 跨新整分钟再 `Settle` | 第二次同窗口 `ChargedTempMinutes==0`；跨新分钟只结算新整分钟——即 B4 修复不得破坏既有单实例幂等（复用 `runtimeengine_settle_test.go::TestSettle_TempConsumeAndIdempotent` 断言作护栏） |
| TC-16-013 | 【回归防护·B4】窗口首尾相接不重叠 | 集成 | P1 | 同上 | 多段 `advance` 平铺（temp/boot_slot/capped_free 混合）后连续 tick | 各 charge 窗口 `[WindowStart,WindowEnd)` 首尾相接、无重叠（游标 `base+settledBefore` 平铺不变量），唯一约束不误伤相邻窗口 |

---

## 三、B6 · 充值金额无上限（billing/internal · bizorder_service.go）

> **现状**：`CreateOrder` 的 `BizRecharge` 分支（`backend/modules/billing/internal/bizorder_service.go:135-138`）**仅**校验 `req.AmountCents <= 0` → 报 `充值金额必须大于0`（TC-10-012 已覆盖下限），**无任何上限**。`AmountCents` 为 `int64`（`bizorder_model.go:103`）；`wechat/alipay` 为桩网关，下单即视为到账并立即 `wallet.TopUp` 履约（TC-10-037）——因此可自助入账天文余额；反复累加还可能溢出 `int64` 或后续以余额支付放大。对应 review **B6**（`billing/bizorder_service.go:115`，健壮性）。
>
> **失败注入**：直接发超大额 `amount_cents`（如 `1e18` 或近 `MaxInt64`）下单，断言应被拒。层级优先 apptest（经公开 HTTP，桩网关即时 paid，观测余额是否被天量入账），internal 侧断言 `CreateOrder` 直接返回校验错误。

| 用例ID | 标题 | 类型 | 优先级 | 前置条件 | 步骤 | 预期结果 |
| --- | --- | --- | --- | --- | --- | --- |
| TC-16-020 | 【发现待修·B6】超大额充值应被拒 | 接口 | P1 | apptest；`registerUser` 拿前台令牌 | `POST /api/v1/billing/orders {biz_type:"recharge",amount_cents:100000000000000000,pay_method:"wechat"}`（1e17 分 = 1 万亿元） | **应**：422「充值金额超过单次上限」（新增上限校验，如 ≤ `MaxRechargeCents`），不建单、余额不变。**当前实现**：仅判 >0 → 桩网关即时 paid、`wallet.TopUp` 天量入账 → 用例 **FAIL**（记为待修）。修复方向：`BizRecharge` 分支加 `req.AmountCents > MaxRechargeCents → Validation` |
| TC-16-021 | 【发现待修·B6】接近 int64 上限拒绝防溢出 | 接口 | P1 | 同上 | `amount_cents = 9223372036854775807`（MaxInt64）下单 | **应**：干净 422 拒绝，不触发 `TotalCents`/`FeeCents` 累加溢出。**当前实现**：无上限，累加/手续费计算路径存在溢出隐患 → 记为待修 |
| TC-16-022 | 【回归防护·B6】正常额度充值不受影响 | 集成 | P0 | 同上 | `amount_cents:10000,pay_method:"wechat"` | 200、订单 paid、余额 +10000（复用 TC-10-010 语义 `bizorder_service_test.go:TestBizOrder_ThirdPartyRechargeCreditsBalance`）——上限校验不得误伤合理金额 |
| TC-16-023 | 【回归防护·B6】下限与余额支付禁用仍生效 | 安全 | P1 | 同上 | `amount_cents<=0`；及 `pay_method:"balance"` | 分别 422「充值金额必须大于0」/「充值不支持余额支付」（复用 TC-10-012/011），确认新增上限校验不覆盖既有下限/支付方式守卫 |

---

## 四、O6 · 明文密钥落访问日志（accesslog/internal · sanitize.go）

> **可利用性评估（关键，已核对代码）**：
> - 密钥管理路由 `/user/api-keys`（`RevealKey`/`CreateKey`）在 `openapi/internal/module.go:20-29` 挂在 `RegisterRoutes` 收到的 `router`（即 **`/api/v1` 组**，代码注释亦写「挂在 /api/v1 下」）。
> - `/api/v1` 组由 `framework/router.go:24-28` 无条件 `apiV1.Use(AccessLogMiddlewareFunc())` —— **经过访问日志中间件**。
> - 中间件（`accesslog/internal/middleware.go:72`）对前台 `scope=user` 默认不入库，但开 `ACCESS_LOG_USER_ENABLED=true` 后**会**捕获并落 `response_body`。
> - 脱敏白名单 `sensitiveBodyFields`（`sanitize.go:13-20`）含 `password/secret/token/…`，但**缺 `fullkey`**（`RevealKey`/`CreateKey` 响应体是 `{"fullKey":"gp_live_…"}`，见 `key_api.go:86` 与 `model.go:52`）。
> - **结论**：与误报「X-API-Key 明文落库」（那条走引擎根 `/api/open/v1`，不经中间件——见 review 误报表）**不同**，`/user/api-keys` 确经中间件。故开 `ACCESS_LOG_USER_ENABLED` 后，`fullKey` 明文进 `access_logs.response_body`，运营/DBA 可读到用户完整密钥 → **真实可利用**（非纯纵深防御）。对应 review **O6**（`openapi/key_api.go:86` / `accesslog/sanitize.go`，安全）。
>
> **失败注入**：单测层直接测 `sanitizeBody`/`sanitizeJSON` 对含 `fullKey`/嵌套 `token` 的 JSON 是否脱敏。

| 用例ID | 标题 | 类型 | 优先级 | 前置条件 | 步骤 | 预期结果 |
| --- | --- | --- | --- | --- | --- | --- |
| TC-16-030 | 【发现待修·O6】fullKey 响应体应脱敏 | 安全 | P1 | accesslog/internal 单测 | 调 `sanitizeBody(`{"fullKey":"gp_live_ABC…","key":{"id":1}}`, "application/json")` | **应**：`fullKey` 值被替换为 `******`。**当前实现**：白名单无 `fullkey` → 明文原样保留 → 用例 **FAIL**（记为待修）。修复方向：`sensitiveBodyFields` 补 `"fullkey":true`（配合小写匹配 `strings.ToLower`）与 `"full_key"` |
| TC-16-031 | 【回归防护·O6】既有敏感字段仍脱敏 | 安全 | P1 | 同上 | `sanitizeBody` 传含 `password/token/access_token/refresh_token/api_key/secret` 的嵌套 JSON | 全部值 → `******`；嵌套对象/数组内的敏感 key 也递归脱敏（`sanitizeJSON` 递归护栏，防补白名单时回归） |
| TC-16-032 | 【纵深防御·O6】Authorization/Cookie 头脱敏 | 安全 | P2 | 同上 | `sanitizeRequestHeaders({Authorization:"Bearer gp_live_…", Cookie:"s=…"})`；`sanitizeResponseHeaders({Set-Cookie:…})` | `Bearer ******` / `[REDACTED]`——密钥即便以 Bearer 出现在头也不落明文（既有行为回归） |
| TC-16-033 | 【发现待修·O6】开关开启后端到端不落明文 | 安全 | P1 | apptest；临时置 `framework.AppConfig.AccessLogUserEnabled=true`（测试内改回）；前台令牌 | `POST /api/v1/user/api-keys {name}` → 拿 `id` → `GET /api/v1/user/api-keys/:id/reveal`；随后查 `access_logs` 该请求行 `response_body` | **应**：`response_body` 中 `fullKey` 已脱敏为 `******`。**当前实现**：明文 `gp_live_…` 落库 → 用例 **FAIL**（记为待修）。**降级说明**：若判定运营侧 `ACCESS_LOG_USER_ENABLED` 生产恒关，则可降为纵深防御（低优），但因该路由确经中间件、开关一开即泄露，建议按可利用中危处理并落 TC-16-030 单测护栏 |

---

## 五、素材库 apptest 补齐（新建 `apptest/library_material_test.go` · TC-11 flagged）

> **背景**：TC-11「关联/备注」明确 Plan A 未含素材库独立 apptest，建议补经公开 HTTP 的端到端。本节给骨架：上传 → 全局去重命中 → 私有桶属主隔离（用户 B 拿不到 A 素材）→ 推送 per-phone 结果。
>
> **环境前提（已核对）**：apptest 默认 `framework.S3Library == nil`（见 `apptest/library_test.go:110` 的 503 契约用例），故完整「presign→直传→confirm」上传链需**配置素材库私有桶**（如本地 MinIO：`S3_LIBRARY_*` 环境变量）方可跑；未配置时上传用例应 **Skip**（对齐 `TestLibraryPresignWithoutS3Returns503` 的 `if framework.S3Library != nil` 门控写法）。推送 fan-out/属主隔离的核心逻辑已由 `phone/internal/push_from_library_test.go` 与 `library/internal/opencontent_test.go` 覆盖（TC-11「自动化覆盖」），apptest 侧作**跨模块 HTTP 端到端补充**。

| 用例ID | 标题 | 类型 | 优先级 | 前置条件 | 步骤 | 预期结果 |
| --- | --- | --- | --- | --- | --- | --- |
| TC-16-040 | 上传两段式端到端 | 集成 | P1 | 配置 S3_LIBRARY（否则 Skip）；`registerUser` 取 A 令牌 | `POST /api/v1/library/upload/presign {name,size_bytes,mime,folder_id:0,md5,slice_md5}` → 用 PUT URL 直传内容 → `POST /api/v1/library/upload/confirm {file_id}` | presign 返回 `file_id/upload_url/s3_key`、落 `status=uploading` 行；confirm 后 `status=active`、`used_bytes` 累加、`GET /api/v1/library/files` 可见（对齐 TC-11-001/003） |
| TC-16-041 | 全局去重命中（同内容复用 blob） | 集成 | P1 | 同上；A 已上传内容 X（`ref_count=1`） | A 再传相同 (md5,size,slice_md5) 内容并 confirm（或 presign 秒传 `instant=true`） | 底层 `library_blobs` 仍 1 份物理对象、`ref_count` 递增；A 得独立 `library_files` 行（对齐 TC-11-021/022）。可经 `framework.DB` 查 `library_blobs.ref_count` 与对象数断言 |
| TC-16-042 | 私有桶属主隔离（B 拿不到 A 素材） | 安全 | P0 | A 上传得 `fileA_id`；`registerUser` 另取 B 令牌 | B `GET /api/v1/library/files`；B `GET /api/v1/library/files/<fileA_id>/download`；B `DELETE /api/v1/library/files/<fileA_id>` | B 列表**不含** A 文件；下载/删除均 404「文件不存在」；A 数据与用量不受影响（对齐 TC-11-025/030/031/032，属 IDOR P0 安全回归） |
| TC-16-043 | 跨用户去重但逻辑行隔离 | 安全 | P0 | A 已上传内容 X | B 上传**相同内容** X 并 confirm | 物理层复用同 blob（`ref_count=2`、S3 仍 1 份对象）；但 B 的 `library_files` 是 B 独立行、B 列表只见自己——去重不得穿透属主隔离（对齐 TC-11-025，重点核对「秒传门控 slice_md5 不一致放行」防伪造 md5 冒领，见 TC-11-023） |
| TC-16-044 | 选素材推送云手机 per-phone 结果 | 集成 | P1 | A 有 active 文件；A 有 provisioned 云机（依赖中台，apptest `ops=nil` 则 Skip 或用 fake 中台）；`file_ids ≤10` | `POST /api/v1/phone/files/push-from-library {phone_ids,file_ids}` | 整请求 200；`results` 逐台 `{phone_id,ok}`；文件**只从 S3 下载一次**复用（内部单测已断言，apptest 断言 per-phone 回报结构）；`file_ids` 空或 >10 → 400「请选择 1-10 个文件」（对齐 TC-11-050/052） |
| TC-16-045 | 推送越权/锁定/空列表拒绝 | 安全 | P0 | 同上 | phone_ids 含 B 的手机；及 A 素材库超额锁定（`framework.DB` 造 `used_bytes>capacity`）；及 phone_ids 为空 | 分别：非属主手机整请求拒绝、不下发任何机；锁定用户下载阶段前置失败（`assertNotLocked`）；空手机列表 400（对齐 TC-11-053/054/056） |

---

## 六、自动化 apptest 补齐（新建 `apptest/automation_test.go` · TC-14 flagged）

> **背景**：TC-14「关联/备注·Plan A 交叉引用」明确 Plan A 未新增 automation apptest，建议补经公开 HTTP：登录→建脚本→下发→查详情，覆盖跨模块 phone 归属与真实中台链路的 E2E。
>
> **环境前提（已核对）**：automation `Init` 用 `newMidplatPort()`（`automation/internal/midplat.go:102`），中台三要素未配置即 `ops=nil`（apptest 默认如此，TC-14-061）；`ops=nil` 时脚本/任务/计划接口返回「云手机中台未配置」。因此 automation E2E apptest 需注入 **fake 中台**：可 (a) 在 `TestMain` 前用 `httptest.NewServer` 起假中台并设 `MIDPLAT_BASE_URL/ACCESS_KEY/SECRET_KEY` 环境变量再 `Init`（走真实 `sdkAdapter`）；或 (b) 由 automation 模块提供 test-only 导出钩子注入 `Service.ops`（apptest 不能 import internal，需模块侧显式暴露）。无 fake 中台时，仅能验证「未配置 → 云手机中台未配置」的降级契约（TC-16-053）。参数嵌套 console 序列化不变量的可判定断言已由 `automation/internal/params_test.go` 全面覆盖（TC-14-040~048），apptest 侧作 HTTP 贯通验证。

| 用例ID | 标题 | 类型 | 优先级 | 前置条件 | 步骤 | 预期结果 |
| --- | --- | --- | --- | --- | --- | --- |
| TC-16-050 | 登录→建脚本→绑定 scriptId | 集成 | P1 | fake 中台已注入；`registerUser` 取令牌 | `POST /api/v1/automation/scripts {name,luaContent,fileName}` | 200；本地落 `store=false,user_id=本人,status=enabled`；经 fake 中台上传得 `scriptId != 0`（对齐 TC-14-001 的 HTTP 端到端版；核心绑定逻辑已由 `service_test.go:TestCreateUserScriptUploadsAndBinds` 覆盖） |
| TC-16-051 | 我的脚本列表属主隔离（HTTP） | 安全 | P0 | A、B 各建脚本 | A `GET /api/v1/automation/scripts` | 仅返回 A 的脚本（`store=false AND user_id=A`），不含 B、不含商店脚本（对齐 TC-14-002，跨用户 HTTP 端到端） |
| TC-16-052 | 下发任务→查详情（含参数注入） | 集成 | P1 | fake 中台；脚本就绪；A 有 provisioned 云机（依赖 phone 归属，需 fake 中台开通或 seed）；脚本含 string/number 参数 | `POST /api/v1/automation/tasks/run {scriptId,cpIds,taskName,params}` → 取 `midTaskId` → `GET /api/v1/automation/tasks/:midTaskId` | run：每台经中台 `CreateTasks`、本地插 `AutomationTask(trigger=manual,status=WAITING_PUBLISH)`；fake 中台可断言收到的 `scriptParams` 为**嵌套 console 格式** `{"key":{"desc","type","required","value"}}`（TC-14-040 不变量的 HTTP 贯通核对）；detail：经中台 `TaskStatuses` 取实时状态，非本人任务 404（对齐 TC-14-030/033） |
| TC-16-053 | 目标云机归属过滤（HTTP） | 安全 | P0 | fake 中台；cpIds 含他人/未开通机 | A 下发任务，cpIds 混入 B 的机 | `filterOwned`（`phone.OwnedCpIDs`）过滤；无有效目标 → 422「未选择有效的云手机」，不误发（对齐 TC-14-031） |
| TC-16-054 | 必填参数缺失拒绝（HTTP） | 接口 | P1 | fake 中台；脚本有 required string 参数 | 下发不传该参数 | 422「… 为必填」，任务不下发（对齐 TC-14-043，HTTP 端到端核对 `serializeParams` 校验在真实请求链生效） |
| TC-16-055 | 中台未配置降级契约 | 集成 | P2 | **不**注入 fake 中台（apptest 默认 `ops=nil`） | 前台令牌调 `POST /api/v1/automation/tasks/run` / `GET /automation/scripts/usable` | 返回「云手机中台未配置」类错误，不 500、不空转 worker（对齐 TC-14-061；此项无需 fake 中台，是默认 apptest 环境即可跑的降级回归） |

---

## 七、低危可选（纵深防御 · 引用为主）

| 用例ID | 标题 | 类型 | 优先级 | 前置条件 | 步骤 | 预期结果 |
| --- | --- | --- | --- | --- | --- | --- |
| TC-16-060 | 【纵深防御·A7】登录时序侧信道 | 安全 | P2 | staff/internal 单测 | 对比「用户名不存在」与「用户名存在密码错」两条登录路径耗时 | **现状**：`Login`（`staff/internal/api.go:30`）在 `GetStaffByUsername` 失败时**短路跳过** `VerifyPassword`（bcrypt），存在时序差可枚举用户名——无 dummy-hash 兜底（review **A7**，`staff/api.go:30`）。用例记为**发现待修**（低优）：修复方向=用户名不存在时也跑一次固定 dummy bcrypt 抹平时序。断言方式：两路径耗时差应落在噪声阈内 |
| TC-16-061 | 【纵深防御·C7】folderPath 越界校验 | 安全 | P2 | phone/internal 单测（fakePort 记录 `folderPath`） | `FileUpload` 传 `folderPath="../../etc"`、绝对路径 `/data/x`、正常相对路径 | **现状**：`op_api.go:694` 把 `c.PostForm("folderPath")` **原样透传** `PhoneService.FileUpload` → 中台，无绝对/相对合法性校验（review **C7**，`phone/op_api.go:694`）。用例记为**发现待修**（低优）：修复方向=拒绝含 `..`/绝对路径的 folderPath。断言=非法路径应在到达中台前被拒 |
| TC-16-062 | 【回归防护·S3】CORS 反射式凭证已有覆盖 | 安全 | P1 | framework 单测（**已存在**） | 引用 `framework/middleware_test.go`：`TestCORSWhitelistedOriginEchoed`、`TestCORSNonWhitelistedOriginBlocked`、`TestCORSEmptyAllowlistEchoesAny`、`TestCORSWildcardEchoesAny` | 白名单命中才回显 Origin + `Allow-Credentials:true`；未命中不下发 CORS 头。**注**：review **S3**（`framework/middleware.go:26`）指出「未配置默认回显任意 Origin」仍是反模式——`TestCORSEmptyAllowlistEchoesAny`/`TestCORSWildcardEchoesAny` 记录了当前放行任意来源的行为，收紧后应把这两条改为「无配置默认拒绝/不发凭证」的断言。此处**引用现有测试**，不重复新建 |

---

## 关联/备注

- **环境**：默认 sqlite（`go test ./...` 无外部依赖）。B4 并发类因 sqlite 串行化，用「重复/重放 + 桩游标」等价复现，真并发建议切 MySQL/PG 压测复核（承 review 结语）。素材库上传 apptest 需 `S3_LIBRARY_*`（否则 Skip）；automation E2E apptest 需 fake 中台（`httptest.NewServer` + `MIDPLAT_*` 环境变量注入，或模块侧 test-only ops 钩子），否则仅能跑 TC-16-055 降级契约。O6 端到端需临时置 `framework.AppConfig.AccessLogUserEnabled=true`（测试内还原）。
- **注入/断言接缝汇总**：fakePort（`phone/internal/ops_test.go`，注入 `Service.ops`，用于 C6）；fake 中台 HTTP（`phone/internal/midplat_failedlist_test.go` 的 `httptest.NewServer` + `midplat.New{BaseURL}`，走真实 `sdkAdapter`，用于 automation apptest/O6 端到端）；`framework.DB` 直连造数/回拨（用于素材库去重/锁定与 O6 日志核对）；`framework.CleanTable`/唯一命名隔离（勿截断共享 seed 表）。
- **编排/工具**：模块 internal 单测经 `framework.SetupTestDB(m)` + `Module.Init`（真实 sqlite）；apptest 经 `setupRouter()`/`doJSON`/`decode`/`registerUser`/`adminToken`（`apptest/helpers_test.go`、`note_test.go`、`billing_trial_test.go`）。
- **跨文档引用**：C6 → TC-12（按 URL 安装）+ TC-14-063（C2 差集姊妹项，均属「中台失败静默」聚类）；B4 → TC-10 第六节（运行结算 B4）；B6 → TC-10-012（下限已覆盖，本文补上限）；O6 → TC-14-072/074（MCP 密钥体系）+ Plan A 安全矩阵；素材库 → TC-11 全篇；automation → TC-14 全篇；A7/C7/S3 → Plan A 安全回归矩阵与 review 低危表。
- **已知缺陷参考**（`docs/code-review-2026-07-02.md`）：C6（`phone/install_by_url.go:81`）、B4（`billing/runtimeengine_service.go:75`）、B6（`billing/bizorder_service.go:115`）、O6（`openapi/key_api.go:86` + `accesslog/sanitize.go`）、A7（`staff/api.go:30`）、C7（`phone/op_api.go:694`）、S3（`framework/middleware.go:26`）。**标注「发现待修」的用例（TC-16-001/002、010/011、020/021、030/033）当前预期先 FAIL**——须先落地对应生产码修复（差集回报 / 唯一约束+条件写 / 充值上限 / 脱敏白名单补 fullKey），再将其转为可判定的正向回归断言并计入 Plan A 常驻回归。「回归防护」类用例即刻应通过，作为修复的护栏。
