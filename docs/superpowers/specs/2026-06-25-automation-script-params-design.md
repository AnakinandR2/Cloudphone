# 自动化：脚本参数能力（${} 文本替换 + 注释声明 schema + 启动时填参）— 设计

- 日期：2026-06-25
- 状态：已实现（含机制纠正）

## 修订 2026-06-25（机制纠正，以本节为准）

拿到中台官方参数示例 [docs/external_params_example.lua](../../external_params_example.lua) 后，确认了之前"不确定"的设备端机制，原 §1 决策 1 被推翻：

- **机制 = 纯 `${placeholder}` 文本替换，替换值是一段 Lua 字面量**（不是运行时 `params.xxx` 表）。
  - `local s = '${string_param}'`（字符串：模板自带引号，值传原文）
  - `local n = ${int_param}` / `local b = ${bool_param}`（数字/布尔：裸字面量）
  - `local a = ${array_param}`（数组/对象：注入 Lua table 文本 `{'a','b','c'}`，**由我们渲染**）
- **schema 真源 = 脚本顶部 `--[[ ... ]]` 注释**（中台 object 格式：按参数名为 key，每项 `{desc,type,required}`；类型词表 `string/int/bool/array`）。我们解析它来渲染填参表单；编辑器在打开/上传时解析、保存时写回注释（双向同步）。
- **类型**：保留我们的全 JSON 类型超集（`string/number/boolean/enum/array/object`），与中台词表互映射（`int↔number`、`bool↔boolean`、`array/object→Lua table`、`enum→string`+options），注释里另存 `uiType` 做无损回环。
- **替换约定（中台示例风格）**：字符串 `'${x}'`、其余裸 `${x}`；标量按 JSON 原生发，array/object 由后端渲染成 Lua table 文本塞进 `scriptParams`。
- **`params_schema` 列**：从此是"由顶部注释推导的归一化缓存"（供填参表单），不是独立编辑的字段。
- **透传链路不变**：值仍走 §7.2/§7.3 `scriptParams`；下面 §2/§3 的"运行时参数表"措辞按本节理解为"`${}` 文本替换"。

以下原文保留备查（决策 1 已被本节取代）。

---

- 背景：脚本管理 / 任务计划 / 任务日志已落地（见 [2026-06-17 设计](2026-06-17-automation-scripts-tasks-design.md)）。
  但脚本是「纯静态代码」——无法在启动时把外部值喂进去（如 `hello {placeholder}` 里的 `placeholder`）。
  本设计给脚本系统加上**参数能力**：作者在脚本上声明类型化参数，运行者启动任务时填值，值透传给中台 `scriptParams`。

## 0. 现状盘点（为什么是「半截管道」）

| 层 | 现状 | 结论 |
|---|---|---|
| 设备端 Lua 运行时（`自动化Lua体系-README`） | 整条链只收 `script + taskId`，脚本被 `xpcall` 直接包起来跑，**无任何参数注入** | 不在本仓库（仓库只有 `backend/admin/my/ops/www`），本期不动 |
| 中台协议（手册 §7.2 / §7.3） | 定义了 `scriptParams`（JSON 字符串）：一次性按 `taskList[].scriptParams` 逐台、计划按全局 `scriptParams` | 管道存在 |
| 后端 SDK（`framework/midplat/auto_script.go`） | `CreateScriptTaskItem.ScriptParams` / `CreateScriptPlanRequest.ScriptParams` **字段已定义** | 已就绪，未被使用 |
| 后端执行流（`automation/internal`） | `RunNow` / `CreateTasks` 适配器 / `plan_service` **从不填 `scriptParams`**，也不采集参数 | 本期接上的核心 |

**「管道」两头都断**：我们这端没采集、没下发；设备端文档里也没写脚本怎么读 `scriptParams`。本期接通**我们这端**。

## 1. 目标与范围

把「参数」打通成前后端真实能力：

- **声明**：作者在脚本上挂一组**结构化参数定义**（params schema）：key / label / type / required / default / (enum) options。
- **填写**：运行者创建任务时，前端按 schema **动态渲染表单**并校验；支持「默认共用 + 可逐台覆盖」。
- **透传**：后端把每台机器的最终参数序列化成 JSON，填入中台 `scriptParams`（一次性逐台、计划全局）。

### 关键决策（评审锁定）

1. **参数模型 = 运行时参数表**（脚本运行时读 `params.xxx`），**不做**文本替换 `{{placeholder}}`。理由：文本替换需为每种参数组合向中台重传模板（§7.2 任务只引用预上传的 `scriptId`，不接受内联 lua），代价高且污染模板库。
2. **声明方式 = 结构化 schema**（非纯 JSON 输入、非脚本自动解析）。换取「界面能力」：友好表单 + 类型校验。
3. **类型覆盖全 JSON**：`string / number / boolean / enum / array / object`。简单类型给原生控件；`array / object` 给内嵌 JSON 编辑器（复用 CodeMirror）+ 校验。
4. **粒度 = 默认共用 + 可逐台覆盖**。逐台覆盖**仅一次性任务**（中台 §7.3 计划只有一个全局 `scriptParams`，协议无法逐台）。
5. **周期计划纳入本期**：只支持共用参数（与一次性共用同一对话框与表单组件，边际成本小）。

### 不在本期

- 设备端实际读取 `scriptParams` 的验证（设备端不在本仓库）。**建议作为紧接着的真机冒烟测试**（见 §8）。
- 文本替换 `{{placeholder}}` 路径（已决定不走）。
- 开放 API `run-script` 传参（可后续给 body 加 `params` → `scriptParams`）。
- 周期计划的逐台参数（中台协议限制）。
- 参数的条件显隐 / 依赖（如「A 选 X 才显示 B」）、参数分组、参数模板复用。

## 2. 数据模型

### 2.1 `automation_scripts` 新增列

| 字段 | 类型 | 说明 |
|---|---|---|
| `params_schema` | TEXT（JSON 数组，可空） | 参数定义数组；空 / NULL = 无参数，**向后兼容**老脚本 |

`AutoMigrate` 补列即可（无迁移系统，幂等建表）；老行该列为空，行为不变。

### 2.2 ParamSpec（单个参数定义）

```jsonc
{
  "key": "name",          // scriptParams JSON 的键；唯一 + 合法标识符 ^[A-Za-z_][A-Za-z0-9_]*$
  "label": "用户名",       // 表单显示名（为空时回退到 key）
  "type": "string",       // string | number | boolean | enum | array | object
  "required": true,
  "default": "world",     // 可选；类型须与 type 匹配
  "description": "可选帮助文本",
  "options": ["a", "b"]    // 仅 enum：候选值（非空）
}
```

- `array` / `object` 的 `default` 与运行值为任意 JSON，覆盖全 JSON 类型。
- 后端 `ParamSpec` 结构 + `ParamType` 枚举落在 `model.go`。

### 2.3 schema 校验（保存脚本时，`validateSchema`）

逐条校验，任一失败 → 422（`apperr.Validation`）：

- `key` 非空、匹配 `^[A-Za-z_][A-Za-z0-9_]*$`、在数组内唯一。
- `type` ∈ 白名单。
- `type == enum`：`options` 非空；`default`（若给）须 ∈ `options`。
- `default`（若给）类型与 `type` 匹配（number 是数字、boolean 是布尔、array 是数组、object 是对象、string 是字符串）。

## 3. 参数提交与透传（执行流）

### 3.1 请求新增字段（run / plan 创建）

| 字段 | 适用 | 说明 |
|---|---|---|
| `params` | run + plan | `{ key: value }`，**共用**参数 |
| `perPhoneParams` | 仅 run | `{ cpId: { key: value } }`，逐台覆盖（可选；计划忽略） |

### 3.2 后端流程

1. 取脚本 `params_schema`。对**每台机器的最终参数** = 共用 ∪ 该机覆盖（覆盖优先），调 `validateValues`：
   - 必填齐全、类型匹配、enum 命中候选、array/object 可解析。
   - 失败 → 422，错误信息点名是哪个 key、哪台 cpId。
2. 把每台最终参数序列化为 **JSON 字符串**填入中台字段：
   - **一次性（§7.2）**：`taskList[]` 每条 `scriptParams` = 该 cpId 最终参数 JSON。
   - **周期计划（§7.3）**：单个全局 `scriptParams` = 共用参数 JSON（无逐台）。
3. 无 schema / 无参数的脚本：`scriptParams` 留空（omitempty），与当前行为一致。

### 3.3 校验语义补充

- `params` / `perPhoneParams` 里出现 schema 未声明的 key：**忽略并告警**（不报错，容错）；或严格拒绝——本期取**忽略**（前端只渲染已声明字段，多余 key 仅可能来自手改请求）。
- `perPhoneParams` 的 cpId 不在本次目标机器列表内：忽略。

## 4. 后端改动清单

- `model.go`：`AutomationScript` 加 `ParamsSchema string`（`gorm:"type:text"`，`json:"paramsSchema"`）；新增 `ParamSpec` / `ParamType`；`validateSchema(raw string) ([]ParamSpec, error)`、`validateValues(specs []ParamSpec, values map[string]any) error`。
- `script_service.go`：`ScriptInput` 加 `ParamsSchema`；`createScript` / `updateScript` 落库前 `validateSchema`。
- `task_service.go`：`RunNow` 签名加 `params map[string]any, perPhoneParams map[string]map[string]any`；按 schema 校验每台；构造每台 `scriptParams` 字符串。
- `midplat.go`：`CreateTasks` 适配器入参加「每台参数 map」→ 填 `CreateScriptTaskItem.ScriptParams`（无参数则不填）。
- `plan_service.go`：创建计划接收 `params` → `validateValues`（共用）→ 填 `CreateScriptPlanRequest.ScriptParams`。
- `api.go`：run / plan 创建 handler 解析 `params` / `perPhoneParams`。
- 门面 `automation.go`：若 `RunNow` 暴露在门面/openapi，相应调整签名（保持单向依赖）。

## 5. 前端

### 5.1 作者侧 — 声明参数

- `my/src/views/automation/ScriptEditDialog.vue` 加「参数定义」区：增删行编辑器，每行 = key / label / type 下拉 / required 开关 / default 输入 /（type=enum 时）options 编辑。保存进 `paramsSchema`。
- `admin/` 脚本商店编辑对话框复用同一编辑器（商店脚本也能声明参数）。
- 抽出可复用组件 `ParamsSchemaEditor.vue`；my 与 admin 是独立应用，**各端各放一份**（同一实现思路，不跨应用共享代码）。

### 5.2 运行者侧 — 填参数

- `my/src/views/automation/TaskCreateDialog.vue` 选定脚本后按 `paramsSchema` **动态渲染表单**：
  - string→输入框，number→数字框，boolean→开关，enum→下拉，**array/object→内嵌 JSON 编辑器**（复用 `LuaEditor` 的 CodeMirror，切 JSON 语言）+ 解析校验。
  - default 预填；required 前端校验。
  - **「逐台覆盖」折叠区**（仅一次性，默认折叠）：对选中的每台机器给紧凑表单覆盖个别值；切「周期计划」时隐藏。
- 抽出可复用组件 `ParamsForm.vue`（渲染 + 收集值 + 校验）。
- `types/automation.ts`：脚本类型加 `paramsSchema`；run / plan 请求类型加 `params` / `perPhoneParams`。
- `api/modules/automation.ts` + `mock/automation.ts`：带上新字段。
- i18n：补 `script.params.*` / `taskCreate.params.*`（中英）。

## 6. 错误处理

- schema 形状非法（保存脚本）→ 422，点名 key 与原因。
- 参数值校验失败（创建任务/计划）→ 422，点名 key（逐台时点名 cpId）。
- array/object 的 JSON 解析失败：前端即时提示；后端兜底再校验。
- 中台 `scriptParams` 始终是字符串：后端 `json.Marshal` 后赋值，空则 omitempty。

## 7. 测试

- 后端 `automation/internal`（fake midplat port）：
  - `validateSchema`：合法 / key 重复 / 非法 key / 缺 enum options / default 类型不匹配。
  - `validateValues`：必填缺失 / 类型不符 / enum 未命中 / array/object 正常。
  - `RunNow` 透传：共用参数逐台填同串；逐台覆盖覆盖正确；无 schema 时 `scriptParams` 为空（向后兼容）。
  - `plan` 透传：全局 `scriptParams` = 共用参数 JSON。
- `framework/midplat/auto_script_test.go`：`scriptParams` 编解码（已有结构，补断言）。
- 前端：`pnpm build`（vue-tsc）必过；mock 跑通「声明参数 → 创建任务渲染表单 → 提交带参」全链路。

## 8. 后续（强烈建议紧接着做）

- **真机冒烟验证设备端确实读 `scriptParams`**：写一个 `report_result(params.xxx)` 或 `log("hello "..params.name)` 的脚本，带参跑一次一次性任务，查 §7.6.8 报告确认值落到脚本里。
  - 若设备端**不读** `scriptParams`：回退方案是「后端文本替换 + 每参数组合重传模板（内容哈希缓存复用 scriptId）」，届时另开设计。本期界面与数据模型可原样复用。

## 9. 依赖与接线

- 无新增模块；改动收敛在 `automation` 模块 + `midplat` SDK + my/admin 前端。
- `params_schema` 列经 `RegisterSetup` 的 `AutoMigrate` 幂等补列。
- `framework/arch_test.go` 自动覆盖既有模块边界（无新跨模块依赖）。
