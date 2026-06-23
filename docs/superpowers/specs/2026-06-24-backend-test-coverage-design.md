# 后端测试覆盖率补强 — 设计文档

- 日期：2026-06-24
- 范围：`backend/`（Go + Gin + GORM 模块化单体）
- 目标读者：实现者（据此产出 writing-plans 实现计划）

## 1. 背景与目标

后端已有 76 个测试文件、基础不差，但覆盖率分布不均：业务模块的 **HTTP handler 层几乎全为 0%**，
service / repository 层覆盖更好。本设计在不改动生产代码的前提下，系统性补齐薄弱模块的单元测试与 apptest。

**目标**：让所有目标业务模块的按包覆盖率达到 **≥ 60%**（`go test ./...` 默认按包统计），
并补充端到端 apptest 提升真实链路保真度。

**度量方式**：以 `go test ./...` 的逐包覆盖率为准（与项目现有约定一致，**不**引入 `-coverpkg`）。
这意味着：
- **包内单测**（`internal/` 包内、`gin.New()` + `httptest`）是覆盖率主力，因为它计入该包覆盖率；
- **apptest**（`manager-backend/apptest` 包、经 `framework.SetupRouter` 打真实全量路由）执行了模块代码，
  但**不计入**该模块的逐包覆盖率数字——它的价值是端到端回归/契约保真，不是覆盖率指标。

## 2. 已敲定的决策

1. **广覆盖、阈值 60%**：所有目标业务模块拉到 60%+。
2. **跳过纯外部封装**（不作为覆盖率目标）：
   - `framework/midplat`（中台 HTTP 客户端，3116 行）、`framework/s3`；
   - 各模块 `internal/midplat.go` 的薄封装端口（`sdkAdapter`、`newMidplatPort`、`opCtx`）；
   - `module.go` 装配代码（`Init` / `RegisterRoutes` / `OnStart` / `OnStop`）。
3. **测试类型按可行性自动分配**：依赖中台的逻辑用包内单测 + fake 端口（service/handler 层）；
   不碰中台的纯 DB CRUD/认证既补包内 handler 单测（覆盖率主力）也补 apptest（端到端保真）。
4. **mcp 例外**：mcp 是「中台工具网关」，15+ 工具都走中台、可测代码少，现实上限约 **48%**（可测代码 85%+）。
   不为覆盖率去 mock 整个中台 SDK；mcp 以「可测代码覆盖 85%+」为达标口径，不强求 60% 绝对值。
5. **cloudphone 纳入**：虽是纯网关模块（无 DB），但 handler 含真实参数解析与响应映射逻辑，
   用 fake `http.Client` 可干净测到 78%。
6. **S3 阻塞的上传 handler**：`framework.S3` 是全局变量、未做依赖注入。
   `partner.UploadPartnerImage`、`app` 商店上传等上传 handler **跳过深覆盖**，
   只做基础路径/参数校验测试（未配置→503、缺文件、超限、扩展名非法），不为覆盖率改生产代码。
7. **一次全做**：单一实现计划覆盖全部 11 个模块（下文按 ROI 分梯队仅为可读性，非分期门）。

## 3. 测试策略

### 3.1 包内 handler 单测（覆盖率主力）

统一复用 `modules/staff/internal/api_test.go` 已验证的范式：

```
package <m>          // 在 internal 包内，可访问未导出符号、可注入 fake
gin.New() 建路由 → 注入鉴权中间件 / fake 端口 → httptest.NewRecorder → r.ServeHTTP → 断言
```

每个 handler 的测点矩阵：
- **成功路径** → 200 + 响应体/`data` 结构正确，且验证底层 service 被正确调用；
- **缺认证** → 401（前台 `user.AuthMiddleware`、后台 `staff.PermissionMiddleware`）；
- **无权限** → 403（后台权限标识不足）；
- **参数非法** → 400（`parseID` / 必填校验失败）；
- **service 错误** → `framework/apperr` 经 `FailErr` 映射的对应 HTTP 码（404/409/422…）。

### 3.2 service / repository 单测

- service 依赖 `midplatPort` interface 的，注入 **fake 端口**（复用或扩展已有 fake）断言本地副作用
  （落库字段、权限隔离、中台不可用时的降级）；
- repository 经 `framework.SetupTestDB` 跑**真实 sqlite**，直接 INSERT/SELECT/DELETE 验证 WHERE 条件、JOIN、分页边界。

### 3.3 apptest（端到端保真，不计覆盖率）

对不碰中台或可经公开 API 走通的模块补端到端流：登录拿令牌（`adminToken` / `registerUser`）→
经公开 HTTP API 准备数据 → 断言完整业务流与**身份域隔离**（前台令牌打后台接口必须 401，反之亦然）。

## 4. 共享测试基础设施（先搭，后复用）

| 基础设施 | 说明 | 复用/新建 |
|---|---|---|
| `setupRouter()` 测试辅助 | 各模块 `internal/` 内：`gin.New()` + `<m>Module.RegisterRoutes` + 注入鉴权中间件 | 仿 `staff/internal/helpers_test.go` 新建（每模块一份） |
| 鉴权注入 | 前台 `c.Set(userID)` / `user.AuthMiddleware`；后台 `staff.PermissionMiddleware`（含权限标识） | 复用框架真实中间件 |
| fake 中台端口 | `midplatPort` 的假实现 | **复用**：phone `fakeOps`、automation `fakeOps`+`withFakeOps`、app `fakeUploadPort`；**新建**：billing 需 fake phone 端口（`runningInstanceCount`/`instanceMeta`）、openapi 需 fake phone/app/automation 服务、cloudphone 需 fake `http.Client`（经 `midplat.Client.SetHTTPClient` 注入） |
| 测试数据工厂 | `createTestApp` / `createTestStaff`（带权限）/ `seedPhone` 等 | 部分已有（automation `seedPhone`），其余新建 |

> **注意**：fake 端口与数据工厂尽量做成各模块 `internal/` 内的测试辅助，避免跨模块依赖（Modulith 边界）。

## 5. 逐模块测试计划

> 每个模块标注：当前覆盖率 → 现实目标、新增测试函数估算、关键测试文件与场景。
> 「跳过」项指本设计明确不追求覆盖的代码（外部封装/装配/S3 上传实体）。

### 梯队 T1 — 快赢（纯 DB、无中台、无阻塞）

#### note  37.1% → 75%（约 10 个新增）
- `api_test.go`（新）：`GetNoteList/GetNote/CreateNote/UpdateNote/DeleteNote/currentUserID` 全路径 + **属主隔离**（用户不能访问他人笔记）。
- `service_test.go`（补）：`GetList`/`Create` 错误路径、`Update` 空字段无操作分支。
- `apptest/note_test.go`（增强）：分页、标题搜索、排序、非法请求、空内容边界。

#### user  39.8% → 65%（约 24 个新增，无阻塞）
- `api_test.go`（新）：`Register/Login/Logout/GetProfile/issueToken` —— 注册成功+发令牌+设 Cookie、校验失败、登录密码错/禁用、登出递增 `token_version` 使旧令牌失效、Profile 过期/版本错误 → 401。
- `admin_api_test.go`（新）：`AdminListUsers/AdminGetUser/AdminSetUserStatus` —— 分页过滤、404、启停用递增令牌版本、无 `user:view`/`user:manage` 权限拒绝（需注入 `staff.PermissionMiddleware`）。
- `middleware_test.go`（新）：`UserAuth` —— 有效令牌、过期、版本不匹配、Cookie 回退、后台 scope 令牌打前台 → 401。
- `service_test.go`（补）：列表全过滤组合、`SetActive` 启用路径。

#### proxy  39.9% → 70%+（约 20 个新增）
- `internal/api_test.go`（新）：14 个 handler（List/Options/Get/Create/Update/Delete/BatchImport/Probe/Test/Admin*）—— 注入 `fakeProber` 进 `ProxyService`，测鉴权 401、`parseID` 400、happy path。
- `internal/service_test.go`（补）：`adminQuery` 状态过滤两分支、`normalizeProtocol('')`、`AdminDelete` NotFound。
- **跳过**：`realProber.Probe`/`lookupIPInfo`（需真实网络，用 `fakeProber` 范式规避）。
- `apptest/proxy_test.go`（可选）：CRUD 端到端。

#### partner  31.5% → 62-65%（约 11 个新增）
- `api_test.go`（新）：admin CRUD（`AdminListPartners/CreatePartner/UpdatePartner/DeletePartner/AdminListClicks`，注入带 `partner:view`/`partner:manage` 的测试 staff）+ 公开 `ListPartners/ClickPartner/optionalUserID`（匿名/登录/Cookie 回退/混合软 ID）。
- `service_test.go`（补）：`AdminList` 关键词/启用过滤/分页/排序、各错误路径。
- **跳过深覆盖**：`UploadPartnerImage`（S3 全局变量）—— 仅基础测试（未配置→503、缺文件、超限、扩展名非法）。
- `apptest/partner_test.go`（可选）：CRUD 端到端。

### 梯队 T2 — 主攻（handler 重、fake 多已存在）

#### automation  28.9% → 65%（约 40 个新增）
- `api_test.go`（新）：13 个 handler（`ListScripts/CreateScript/UpdateScript/ToggleScript/DeleteScript/ListPlans/CreatePlan/planActionHandler/RunTask/ListTasks/TaskDetail` 等）—— `planActionHandler` 测 start/pause/delete 三动作。
- `admin_api_test.go`（新）：8 个 admin handler（`AdminStore*` / `AdminUserScript*`）。
- `service_test.go`（扩展）：store 列表过滤、toggle（验证 `ToggleTemplate` 调用）、delete（带/不带 scriptId 的 best-effort `DeleteTemplate`）、admin 变体、工具函数（`parseStrList` JSON 错误、`renderScriptLog` 空列表、`extractScriptResult` 无/双标记边界）。复用 `withFakeOps`+`seedPhone`。
- 可选 repo 直测：`activePlans` 状态过滤、`nonTerminalTasks` 批处理、`upsertTaskByMidID` insert/update 分支。
- `apptest/automation_test.go`（可选）：worker 后台同步（`syncTasks/discoverPlanTasks`，需容忍周期定时 ±50s）。

#### phone  32.0% → 62%（约 25 个新增）
- `api_test.go`（新）：11 个 CRUD handler（`GetCloudPhoneList/Get/Create/Update/Delete/RecycleBin*/Admin*/AdminListPhoneTags`）—— 成功 + 401/404/400 失败路径。
- `op_api_test.go`（新）：32 个操作 handler 分批 —— 常规操作（Power/Restart/Reset/NewDevice/Destroy/WebRTC*/Screenshot/Volume/Rotate/Shake）、应用+ADB+Root（InstalledApps/Install/Uninstall/Start/Stop/KillAll/AdbInfo/EnableAdb/DisableAdb/Root）、脚本+文件+标签（RunHelloScript/ScriptTaskStatus/File*/PhoneTags）。注入 `fakePort`。
- `metering_test.go`（新）：`runSettlement`（结算 6h 窗口）、`runRuntimeGuard`（护栏关停超额）、`shutdownSessions` —— 需 fake `billing.SettleRuntime`/`BootSlotCapacity`（在 `main_test.go` 注入）。
- `recycle_test.go`（补）：`checkSeatAvailable` 失败路径、`runReconcilePatrol` 全用户遍历。

#### billing  53.6% → 68%（约 42 个新增）
- `api_test.go`（新）：旧模型 6 个 handler（`GetMyAccount/GetMyLedger/Topup/AdminGetAccount/AdminAdjustBalance/currentUserID`）。
- `billing2_api_handler_test.go`（新）：26 个 handler 分组测 —— 概览/配置、报价/订单生命周期（创建→支付→履约，含 seat/boot_slot/runtime 各科目）、时长日志分页+时间筛选、后台配置 10 个 GET/PUT、后台订单、资源赠送、`parseTimeQuery` 多格式。`GetBillingOverview` 需 fake phone 端口。
- `trial_api_handler_test.go`（新）：9 个试用 handler（前台 `ListMyTrials/ClaimTrial` + 后台策略/资格/发放记录）。
- repo/service 单测：`bizorder`（`getOwned/get/listOwned/listAll`）、`license`（`activeUnits/countByKind/ListActiveUnits/Capacity`）、`runtimewallet`（`dailyCharged/remaining`，UTC+8 日期边界）、`pricingconfig`（`get/update`，首读默认初始化）。
- `apptest/billing2_test.go`（可选）：新购买模型端到端（概览→报价→下单→支付→历史；后台改价/标记已付；跨用户互不可见）。

#### openapi  35.3% → 65%（约 25 个新增）
- `key_api_test.go`（新）：`ListKeys/CreateKey/RevealKey/RevokeKey/currentUserID/paramUint` —— 纯 DB，无需 fake。
- `middleware_test.go`（新）：`keyAuthMiddleware/bearerToken` —— 缺/无效/已撤销密钥 401、正确密钥解析 userID、`Bearer` 与 `X-API-Key` 两种格式。
- `open_api_test.go`（扩展）：手机/应用/脚本操作 handler（`OpenListPhones/OpenGetPhone/OpenCreate/OpenDestroy/OpenPower/OpenRestart/OpenApps/OpenInstall/OpenUninstall/OpenRunScript/OpenTaskDetail`）—— 依赖 phone/app/automation，注入 fake 服务或经真实 service + mock 中台。
- `apptest`（可选）：创建密钥 → 用密钥调开放 API 端到端。

### 梯队 T3 — 难啃（中台重 / 边界 / 例外）

#### app  5.1% → 55%（约 18 个新增）
- `repository_test.go`（新）：10 个 GORM CRUD 方法（`listByUser/getByIDs/update/deleteByIDs/listAll`（LEFT JOIN users）`/getAllByIDs/deleteAllByIDs/listStore/getStoreByIDs/deleteStoreByIDs`）—— 低成本高收益。
- `service_core_test.go`（新）：`refreshStatus`（中台可用→刷新、不可用→原样）、`List/StoreList`、`Upload/StoreUpload`（整链落库校验）、`BatchDelete/AdminBatchDelete/StoreDelete`（权限隔离、中台同步删、空列表、降级）、`AdminList`（join users）、`firstNonEmpty` 边界。扩展 `fakeUploadPort`（`ListApps`/`BatchDeleteApps`）。
- `service_upload_test.go`（补）：`CreateFromUpload` 补全路径（nil created 回退、字段优先级、uploadID=0 秒传、save 错误传播）。
- `api_test.go`（新）：基础 handler 路径（List/Market/BatchDelete/Admin* + `currentUserID/parseUploadID` 工具）。
- **跳过**：`sdkAdapter`/`newMidplatPort`/`opCtx`（外部封装）、`module.go`（装配）；分片上传流水线（有状态、跨多请求）只做基础校验或留给 apptest。

#### cloudphone  0.0% → 78%（约 13 个新增）
- `api_test.go`（新）：经 `midplat.Client.SetHTTPClient` 注入 fake `http.Client`。简单 handler（`ListZones/ListVMs/ListVMEnums/ListImageList`）测三态：`client==nil`→503、client 错误→502、OK→`OKWithData`。聚合 handler（`ListSpecs` 多 zone 聚合 + kind=phone/vm 分支 + 单 zone 失败跳过、`ListBootPlans` specId≤0→400、`ListSpecImages` 去重 + 单 plan 失败跳过）。
- 测试夹具默认 `client==nil`（`framework.AppConfig` 未配置），仅需 client 的测试单独注入 fake。
- `apptest/cloudphone_test.go`（可选）：端点级权限（`cloudphone:view`）+ 401 验证。

#### mcp  33.6% → 48%（可测代码 85%+，约 18 个新增）
- `auth_test.go`（新）：`extractKey`（Bearer 提取、`X-API-Key` 回退、空）、`httpContextFunc`（密钥注入 context、`openapi.Authenticate` 调用链）。
- `tools_test.go`（扩展）：工具辅助（`jsonResult` 序列化失败、`fail`、`toInt64Slice`、`proxyInputFrom` 必填校验）；各工具**仅测参数校验与错误映射**（不重复中台交互）：phoneTools（page/size 矫正、id 必填、operation 枚举映射）、appTools（id/appIds 必填）、scriptTools（id/scriptId 必填、未开通→错误）、proxyTools 边界。
- **达标口径**：mcp 以可测代码覆盖 85%+ 为准，绝对覆盖率约 48%（不强求 60%）。

## 6. 验证与度量

- **总命令**：`go test ./...`（默认 sqlite，无需外部数据库）。
- **逐模块覆盖率**：`go test ./modules/<m>/internal/ -cover`，确认 ≥ 60%（mcp 除外，见 §2.4）。
- **覆盖率明细定位**：`go test ./modules/<m>/internal/ -coverprofile=c.out && go tool cover -func=c.out`，
  逐函数确认目标函数已从 0% 提升。
- **边界自检**：`go test ./framework/ -run TestModuleBoundaries` 必须仍通过（apptest 不得 import 任何模块 internal）。
- **格式**：提交前 `gofmt -w`（全 LF）、`go vet ./...`。
- **达标判定**：每个目标模块逐包覆盖率 ≥ 60%（mcp 以可测代码 85%+ 计），全量 `go test ./...` 绿。

## 7. 风险与权衡

- **apptest 不计逐包覆盖率**：已在 §1 明确度量口径——apptest 是端到端保真，不承担覆盖率指标；
  覆盖率由包内单测驱动。同一 handler 可能既有包内单测又有 apptest，二者断言对象不同，属健康冗余。
- **fake 端口的真实性**：fake 仅模拟中台契约，无法捕捉中台真实行为变化；通过断言「本地副作用 + 调用参数」降低偏差。
- **mcp / cloudphone 的覆盖率天花板**：二者大量代码是中台透传，绝对覆盖率受限（mcp ~48%、cloudphone 因无 DB 占比小）；
  已分别用「可测代码口径」和「fake http.Client」处理。
- **S3 全局变量**：上传 handler 深覆盖被阻塞；本设计选择跳过而非重构生产代码（YAGNI），仅做基础校验测试。
- **工作量**：约 246 个新增测试函数、一次全做；通过统一 `setupRouter` 范式与 fake 复用控制重复劳动。
