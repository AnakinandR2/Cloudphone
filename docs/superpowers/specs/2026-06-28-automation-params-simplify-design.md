# 脚本参数体系简化：移除 enum、修复 bool 必填、类型对齐 example

日期：2026-06-28
状态：设计已与用户确认，待评审

## 背景

脚本参数体系（`${}` 占位 + 中台 scriptParams 注入）已跑通。中台替换的真相已查明：
任务下发的 `scriptParams` 必须是「每个参数带完整定义 + value」的嵌套结构（中台控制台抓包实证）：

```json
{"int_param":{"desc":"...","type":"int","required":true,"value":"11"}}
```

中台据每项 `type` 决定 `${}` 注入形态：`int`→裸数字、`bool`→裸布尔、`array`→Lua 表、`string`→进引号；`value` 始终是字符串。

现要做三件事，目标是**简化、去掉误导、修一个校验 bug**：

1. **移除 enum 类型**（连同候选项 options 的一切）。
2. **类型对齐** `docs/external_params_example.lua` 的语义：最终只保留 string / number / bool / table 四种。
3. **修 bool 必填 bug**：勾了「必填」的 bool 参数，默认值是 false 时，前端校验误判为「未填」。

非目标（本次不做）：array/table 的注入机制改造（本就是自由 Lua 文本，已支持 `{[1]=2,[26]=5}` 形式）；scriptParams 嵌套格式（已完成）。

## 关键设计决定（用户已确认）

- **编辑器只显示 string / number / bool / table 四个类型词。** 不用 int/array——对用户误导太大、局限太强。int/array 只作为「发给中台 scriptParams 时的 type 词」存在于线缆格式里，用户不可见。
- **bool 行不显示「必填」开关**：复选框天生总有 true/false 值，不存在「未填」，对 bool 谈必填无意义。

## 内部类型 vs 三处「type 词」的对照（务必分清，这是坑区）

| 概念 | 取值 | 谁用 | 备注 |
|---|---|---|---|
| 内部 `ParamType`（前后端一致） | `string` / `number` / `boolean` / `table` | 代码逻辑、前端控件分支 | 移除 `enum` |
| **编辑器显示标签** | `string` / `number` / `bool` / `table` | 用户在参数编辑器看到/选择的 | 用户可见，**本次只保留这 4 个** |
| 脚本注释 `type`（schema 真源，我们写/读） | `toMidType`：number→`int`、boolean→`bool`、table→`table`、string→`string` | `buildSchemaComment` 写、`parseSchema`/`deriveSchema` 读（配合 `uiType` 无损回环） | 中台不读注释做替换，纯属我们内部 round-trip |
| **中台 scriptParams 的 `type`（线缆格式）** | `midType`：number→`int`、boolean→`bool`、table→`array`、string→`string` | `serializeParams` 下发任务时 | **中台据此注入**，必须是 int/bool/array/string |

> 编辑器显示 `table`，但下发中台时 `serializeParams` 把它写成 `"type":"array"`——两者不冲突，分别服务「人」与「中台」。

## 改动清单

### 后端 `backend/modules/automation/internal/params.go`

- 删 `ParamEnum` 常量；`validParamTypes` 去掉 enum。
- `ParamSpec` 删 `Options []string` 字段。
- `validateSchema`：删「enum 必须有 options」「enum 默认值须在 options 内」两段校验。
- `valueMatchesType`：删 `ParamEnum` 分支（原与 `ParamString` 合并）。
- `buildParams`：删 `if sp.Type == ParamEnum { containsStr(...) }` 分支；**新增 boolean 缺值兜底**：
  缺值时优先用 default，否则 **boolean → `false`**（即便 `required:true` 也不报错，兼容手写注释如 example 的 `bool_param required:true`），其余类型 required 缺值仍报错。
- `mapMidType`：删 `"enum"` 分支（default 落 string 即可）。
- `midType`：删 enum（default 落 string）。线缆映射保持 number→int、boolean→bool、table→array、string→string。
- 删 `containsStr` 若仅 enum 在用（确认无其它引用再删）。

### 后端测试 `params_test.go`

- 删 `TestRunNowEnumInvalid`。
- `TestSerializeParamsMidConsoleFormat`：去掉其中的 `mode`/enum 参数项。
- **新增** `TestRunNowBoolRequiredDefaultsFalse`：comment 含 `{"key":"flag","type":"boolean","required":true}`（无 default），RunNow 不传该值 → 不报错，下发 scriptParams 里 `flag` 的 `value` 为 `"false"`、`type` 为 `"bool"`。

### 前端类型 `my/src/types/automation.ts`

- `ParamType` 去掉 `'enum'`：`'string' | 'number' | 'boolean' | 'table'`。
- `ParamSpec` 删 `options?: string[]`。

### 前端 `my/src/utils/paramsComment.ts`

- `OUR_TYPES` 去掉 `'enum'`。
- `mapMidType`：删 `'enum'` 分支（default 落 string）。
- `toMidType`：删 enum 分支。
- `normalizeSpec`：删 options 处理与 `type === 'enum'` 分支。
- `buildSchemaComment`：删 `options` 写入与 enum 分支。
- `parseSchema`（object 分支）：删 `options` 透传。

### 前端编辑器 `my/src/views/automation/ParamsSchemaEditor.vue`

- `types` 列表 = `string / number / bool / table` 四项（label 分别 `string`/`number`/`bool`/`table`；value 为内部类型 `string`/`number`/`boolean`/`table`）。
- 删「候选项」Popover、`optionsText`、`optionsOf()`、enum 的「值」分支（NativeSelect of options）。
- `Row` 接口删 `optionsText`。
- **bool 行「必填」列不渲染 Checkbox**（显示 `—` 灰字占位）。`rowToSpec`：`type === 'boolean'` 时不写 `required`（即便历史 r.required=true 也归零）。

### 前端填参表单 `my/src/views/automation/ParamsForm.vue`

- 删 enum 分支（NativeSelect）。
- **seed**：`seedDefaults` 模式下，boolean 若无 default 则 seed 为 `false`（值始终具体，复选框与提交一致）。逐台覆盖模式（`seedDefaults=false`）不强制 seed bool。
- **validate**：required 空值检查**跳过 boolean**（`s.required && s.type !== 'boolean'`）；其余类型不变。
- array/table 的 Textarea 占位提示更新，同时给出 `{1, 2}` 与 `{[1]=2, [26]=5}` 两种写法示意。

### i18n `my/src/locales/{zh-CN,en}.ts`（admin 若有对应键同步）

- 删 `script.params` 下 enum 专用键：`optionsCol`、`enumPh`、`choose`、`options`。其余保留。

## 数据流（不变，仅去 enum 分支）

```
编辑器(表格) → buildSchemaComment(注释, type=toMidType + uiType) → 写入 luaContent 顶部 --[[ ]]
                                                                         │
建任务: deriveSchema(luaContent) → specs → buildParams(specs, 值) → serializeParams(specs, 值)
        → scriptParams 嵌套格式 {key:{desc,type=midType,required,value}} → 中台 create-scheduled
```

## bool 必填 bug：根因与修复

- **根因**：`validate()` 把 `undefined` 当「未填」。手写注释（如 example）的 bool 参数 `required:true` 且无 default 时，`seedDefaults` 不会 seed（`s.default===undefined`），model 里该键为 `undefined`，复选框显示 false 但校验判空 → 误报。
- **修复（双保险）**：
  1. 前端 seed：boolean 无 default 时 seed `false` → model 始终有具体布尔值。
  2. 前端 validate：boolean 跳过必填空值检查（false 合法）。
  3. 后端 buildParams：boolean 缺值 → false，不因 required 报错。
- **UI 层面**：编辑器对 bool 不再暴露「必填」开关，从源头杜绝「给 bool 设必填」。

## 测试

- 后端：`go test ./modules/automation/internal/`（删 enum 用例 + 新增 bool-required 用例，全绿）。
- 前端：`pnpm build`（vue-tsc 通过）。
- Playwright 端到端（mock）：用 example 同构脚本（string/number/bool/table，其中 bool 必填）走「建任务」：
  - 默认值正确回填（含 bool=false 显示为未勾选）；
  - bool 必填不再误报「未填」，可直接提交；
  - 提交后 scriptParams 为嵌套格式，bool 的 `type=bool`/`value=false`、table 的 `type=array`/`value` 为 Lua 文本。

## 风险与注意

- **三处 type 词别搞混**（见上表）：编辑器显示 `table`、注释写 `table`、中台 scriptParams 写 `array`。改任何一处先对照本表。
- 删 `Options` 字段会触及前后端多文件，删前 grep 确认无遗留引用（`options`、`ParamEnum`、`optionsOf`、`optionsText`、`choose`、`optionsCol`、`enumPh`）。
- 历史已存的含 enum 的脚本注释：`mapMidType` 删 enum 分支后，`"type":"enum"` 会落到 default→`string`，旧 enum 参数退化为普通字符串输入（可接受；本产品无线上 enum 脚本）。
- admin 应用无 automation 参数 UI（已确认），无需同步前端组件改动；仅 i18n 若有同名键才同步。
