# 自动化：脚本管理 / 任务计划 / 任务日志 — 设计

- 日期：2026-06-17
- 状态：已评审，待实现
- 背景：hello-world 最小闭环已真机调通（仅 Lua）。本设计把「脚本管理 / 任务计划 / 任务日志」三块从前端原型升级为真实功能，对接中台手册 §7（自动化脚本任务）。

## 1. 目标与范围

把三块 automation 功能落地为前后端打通的真实能力：

- **脚本管理**：客户上传/编写自己的 Lua 脚本；运营提供「脚本商店」（面向全体客户的公共脚本）；运营治理用户脚本。
- **任务计划**：客户对自己的云手机下发一次性任务，或建立周期计划（每 N 分钟 / 每天）。
- **任务日志**：客户查看任务执行记录与详情报告（日志 / 截图 / 结果）。

### 不在本期

- 脚本商店付费 / 购买（billing 集成）。本期商店全免费、直接可用。
- `ApiMcpView`（API & MCP）保持原型占位。
- 任意 cron 表达式（中台只支持 INTERVAL / DAILY）。
- 计划自动改挂新版脚本（编辑脚本生成新版，旧计划继续指旧版，仅提示）。
- EXECUTING 任务的取消（中台 §7.7 语义未明）。

## 2. 关键决策（评审锁定）

1. **归属模型**：脚本仿现有「应用市场」模式——本地表 + `store` 标记区分商店/用户脚本，`user_id` 属主隔离。任务/计划/日志为客户端功能；运营另有商店脚本管理 + 用户脚本治理。
2. **数据架构**：本地镜像脚本、计划、任务（建三张表）；状态 / 报告 / 截图走中台实时拉取。
3. **语言**：仅 Lua（原型里的 JS/Python 是占位，去掉）。
4. **调度映射**：一次性 = §7.2；周期 = §7.3 的 `INTERVAL`（每 N 分钟）/ `DAILY`（每天 HH:mm:ss）。不做任意 cron。
5. **脚本录入**：上传 `.lua` 文件 + 内置 Lua 代码编辑器（CodeMirror 6）。
6. **商店**：免费、直接可用；不做显式「获取/安装」——任务创建时脚本选择器 = 我的脚本 ∪ 商店脚本。

## 3. 架构

### 3.1 新模块 `automation`（modulith）

```
backend/modules/automation/
  automation.go            # 公开门面
  internal/
    model.go               # AutomationScript / AutomationPlan / AutomationTask
    repository.go           # GORM 实现，SQL/事务收敛
    midplat.go              # 出站端口接口 + sdkAdapter（封装 SDK 自动化调用）
    script_service.go       # 脚本：上传/编辑/启停/删除/列表（含商店与属主隔离）
    task_service.go         # 任务：一次性创建 + 任务日志列表 + 报告
    plan_service.go         # 周期计划：创建/启停/删除
    worker.go              # 同步 worker（plan 派生任务发现 + 任务状态刷新）
    api.go                 # my（user）路由处理
    admin_api.go           # admin（staff）路由处理
    module.go              # 注册 / 建表 / 路由 / worker 生命周期
```

依赖方向：`automation → phone`（经 phone 门面校验 cpId 归属）；`phone` 不依赖 `automation`。billing 不参与（免费）。

### 3.2 phone 门面新增

`backend/modules/phone/phone.go` 导出（委托 internal）：

- `OwnedCpIDs(userID int) ([]string, error)` — 该客户名下全部已开通 cpId。
- `OwnsCpID(userID int, cpID string) (bool, error)` — 校验单台归属。

automation 创建任务/计划时，逐台校验目标 cpId 属于该客户，非法即拒。

### 3.3 三张本地表

**automation_scripts**（脚本库，仿应用市场）

| 字段 | 说明 |
|---|---|
| id | 主键 |
| user_id | 上传者；商店脚本（store=true）为 0 |
| store | bool，true=商店脚本（admin 上传，面向全体）/ false=用户脚本 |
| script_id | int64，中台 scriptId（§7.5.2 模板主键） |
| name / description / version | 元信息 |
| lua_content | text，源码（供编辑器重开/编辑；编辑=重传得新 scriptId） |
| file_name | 上传文件名 |
| status | enabled / disabled |
| created_at / updated_at | 时间戳 |

**automation_plans**（周期计划，§7.3）

| 字段 | 说明 |
|---|---|
| id / user_id | 主键 / 属主 |
| plan_id | int64，中台计划主键 |
| plan_uid | string，中台计划 UID（worker 按此查派生任务） |
| script_local_id | 关联 automation_scripts.id |
| name | 计划名 |
| frequency | INTERVAL / DAILY |
| interval_value | int，分钟（INTERVAL 时） |
| execution_time | HH:mm:ss（DAILY 时） |
| start_time / end_time | 计划生效起止 |
| cp_ids | json，目标 cpId 列表 |
| status | 本地维护：NOT_STARTED / ENABLING / PAUSED / FINISHED |
| created_at / updated_at | 时间戳 |

中台 §7 未提供 plan 单查/分页接口，plan 状态以本地为准、由启停删操作驱动。

**automation_tasks**（任务索引，供日志列表）

| 字段 | 说明 |
|---|---|
| id / user_id | 主键 / 属主 |
| mid_task_id | int64，中台任务主键（§7.2 返回 id；查询/报告入参） |
| task_no | string，中台任务编号 taskId |
| script_local_id | 关联脚本 |
| plan_local_id | nullable，来自哪个计划（一次性为 NULL） |
| cp_id | 目标云手机 |
| task_name | 任务名 |
| last_status | 缓存的英文状态（worker 同步，§7.6.2 枚举） |
| run_start / run_end | 运行起止 |
| last_synced | 上次同步时间 |
| created_at / updated_at | 时间戳 |

一次性任务在创建时插入；计划派生任务由 worker 发现后 upsert。列表查询走本地（快、owner 隔离、分页）；详情报告按需实时拉。

### 3.4 同步 worker

`framework.PeriodicRunner`（DB 租约），周期 ~1 分钟，仅中台已配置时运行：

1. 对每个活跃 plan（非 FINISHED）：`task/page?planUid` → upsert 新派生任务进 `automation_tasks`（task_no / cp_id / mid_task_id / plan_local_id / 运行时间）。
2. 对 `automation_tasks` 中非终态任务：`query-by-ids` 批量刷 `last_status` / `run_start` / `run_end`。
3. 终态（COMPLETED / FAILED / CANCELLED）任务不再刷。

## 4. 中台 SDK（`framework/midplat/auto_script.go` 扩展）

已有并复用：`UploadLuaTemplate`（§7.5.1 multipart）、`ListScriptTemplatesByName`、`CreateScriptTasks`（§7.2）、`QueryScriptTasksByIDs`（§7.6.2）、`GetScriptTaskReport`（§7.6.8）。

新增：

- 模板：`ListScriptTemplates`（§7.5.2，分页 + isPublic/status/name/ids 过滤）、`ToggleTemplates`（batch-toggle 启停）、`DeleteTemplate`（templates/{id} 或 batch-delete）。
- 计划：`CreateScriptPlan`（§7.3）、`StartPlan`/`PausePlan`/`DeletePlan`（§7.4，**POST + query `?id=` 无 body** 的特殊风格，单独构造）。
- 任务：`TaskPage`（§7.6.1，支持 planUid / cpId / scriptId 过滤）。

### 4.1 实测数据修正（来自真机 report dump）

1. `screenshotUrl` 实测是**数组** `["https://…"]` → `ScriptTaskReport.ScreenshotURL` 类型改 `[]string`。否则整个 report 反序列化失败、字段静默全空。
2. report 的 `taskStatus` 实测是**中文**（"已完成"），与 §7.6.2 英文枚举分裂 → 终态判定只用 §7.6.2 英文枚举，report 仅取日志/截图/结果。
3. report 实测含 `runDurationMs`，比秒级 `runDuration` 稳 → 优先用 ms 字段。
4. report 含 `runLogList[].timestamp/content`（`level` 实测不存在）→ 渲染容错。

## 5. 调度映射（§7 对接细节）

| 业务动作 | 中台接口 | 参数要点 |
|---|---|---|
| 一次性立即运行 | §7.2 `autoscript/task/create-scheduled` | `publishTime`=UTC+8 当前（中台无时区按北京时间解析）；`taskList` 每台一条 |
| 周期：每 N 分钟 | §7.3 `scriptPlan/create` | `executionFrequency=INTERVAL` + `intervalValue` |
| 周期：每天 HH:mm | §7.3 `scriptPlan/create` | `executionFrequency=DAILY` + `executionTime=HH:mm:ss` |
| 目标机器 | §7.2/§7.3 | 用字符串 `cpIdList`（推荐，避免 cpIds 数字主键混淆） |
| 计划启动/暂停/删除 | §7.4 | `POST .../scriptPlan/{start,pause,delete}?id={id}`，无 body |
| 任务状态轮询 | §7.6.2 `query-by-ids` | ids 为 Long 数组 |
| 任务报告 | §7.6.8 `task/report` | 入参 `{id}` |
| 计划派生任务发现 | §7.6.1 `task/page` | 按 `planUid` 过滤 |

## 6. 前端

### 6.1 my（客户端，沿用现有 `automation/` 路由与菜单）

- `views/automation/ScriptView.vue`（重做，两 Tab：我的脚本 / 脚本商店）
  - `ScriptEditDialog.vue`（上传 .lua + 编辑器 + 名称/描述）
  - `LuaEditor.vue`（CodeMirror 6 + lua 高亮，懒加载）
- `views/automation/TaskScheduleView.vue`（重做，周期计划列表）
  - `TaskCreateDialog.vue`（顶部切「立即运行（一次性）/ 周期计划」，选脚本 + 多选目标机器 + 频率）
- `views/automation/TaskLogView.vue`（重做，本地任务索引分页 + 状态筛选）
  - `TaskReportDialog.vue`（从现有 `ScriptTestDialog` 抽出的共用报告展示：状态/日志/截图/#RESULT# 结果）
- `api/modules/automation.ts` + `types/automation.ts` + `mock/automation.ts`
- i18n：补齐 `script.*` / `taskSchedule.*` / `taskLog.*` 真实字段（中英）
- `PhoneView` 的「测试脚本」入口升级为「运行脚本」：可选任意脚本（不再写死 hello-world），复用 `TaskReportDialog`。hello-world 闭环代码迁入 automation 模块。

### 6.2 admin（运营端，新 `automation` 区）

- `views/cloudphone/ScriptStoreView.vue`（商店脚本管理，仿 `AppsView`：上传/编辑/启停/删除 store=true 脚本）
- `views/ops/UserScriptsView.vue`（用户脚本治理：查看全量用户脚本 + 下架）
- `api/modules/automation.ts` + 路由 + 菜单 + i18n（中英）
- 权限：`script:view` / `script:manage`，在 `modules/staff/internal/permissions.go` 登记（超管默认放行）

## 7. 错误处理

- 创建任务/计划前逐台校验 cpId 归属（`phone.OwnsCpID`），非法 → 422。
- 目标机器须已开机（脚本在设备内运行）；非 RUNNING / UNKNOWN → 拒绝并提示。
- 中台 §7.4 启停删错误码不一致（实测 HTTP 400 `FAIL` / 500 Quartz 异常）→ 门面统一兜底为可读错误。
- report 反序列化容错（数组截图、缺失 level、中文状态）。
- worker 单台/单 plan 查询失败 best-effort 跳过，不中断整轮。

## 8. 测试

- 后端 `automation/internal` 单测（fake midplat port）：脚本上传落库 + 商店/属主隔离；任务创建透传正确 cpId + 归属校验；计划创建/启停映射；worker 同步 upsert + 刷状态；screenshotUrl 数组反序列化；report 结果抽取。
- `framework/midplat/auto_script_test.go`：新端点编解码 + POST+query 风格。
- `framework/arch_test.go` 自动覆盖新模块边界。
- 前端 `pnpm build`（vue-tsc）必过；mock 跑通三视图增删改查与轮询。

## 9. 依赖与接线

- my 新增 CodeMirror 6（`codemirror` + `@codemirror/lang-lua` 等），懒加载，走 npmmirror 安装。
- modulith 接线：`automation/internal/module.go` 的 `init()` 注册 + `RegisterSetup` 建三表；`main.go` / `apptest/main_test.go` 加 blank import（scaffold:module-imports 锚点）；`permissions.go` 加权限组。

## 10. 已知中台坑（设计内规避，记录备查）

- §7.2 不校验 scriptId 租户归属（P0-A20）——我们只用自家 scriptId。
- §7.4 启停删 POST+query 特殊风格 + 错误码不一致 + 启动可能抛 Quartz 异常。
- §7 无 plan 单查/分页接口——plan 状态本地维护。
- §7.6.8 字段中文/类型不稳——已在 §4.1 修正。
- §7.2/§7.3 publishTime/startTime 无时区，按北京时间——客户端转 UTC+8。
