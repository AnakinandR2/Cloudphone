# 自动化：参数编辑独立对话框 + 去掉 params_schema 列 — 设计

- 日期：2026-06-25
- 状态：待评审
- 背景：脚本参数能力已落地（见 [脚本参数能力设计](2026-06-25-automation-script-params-design.md)），
  schema 真源是脚本顶部 `--[[ ]]` 注释。但参数定义编辑器目前**内联**在脚本编辑对话框里，
  且后端冗余存了一份 `params_schema` 派生缓存列。本设计把参数编辑收进**独立对话框**，
  并删除冗余列，使脚本注释成为**唯一真源**。

## 1. 目标与范围

- **独立对话框**：参数编辑从内联区改为弹出的独立模态。打开时从脚本注释反序列化，保存时序列化写回脚本草稿。
- **去冗余列**：删除后端 `params_schema` 列；任何需要 schema 处都现 parse `luaContent` 注释。

### 关键决策（评审锁定）

1. **保存语义 = 写入脚本草稿**：参数对话框「保存」= 把参数序列化成顶部 `--[[ ]]` 注释写回当前 `luaContent`（Lua 编辑器立即可见），然后关闭。真正落库由外层脚本对话框的保存提交。
2. **唯一真源 = 脚本注释**：删 `params_schema` 列。后端校验、前端填参表单都现 parse `luaContent` 注释。
3. **行编辑器复用**：独立对话框内复用现有 `ParamsSchemaEditor`（行编辑器组件）不重写。

### 不在本期

- 参数对话框内的实时双向同步（仅在打开/保存边界与注释同步，编辑期独立）。
- 已有 `paramsComment.ts`（提取/注入注释、specs↔中台 object 互转）保持不变，直接复用。

## 2. 数据流

脚本顶部 `--[[ ]]` 注释 = **唯一真源**，贯穿：

```
作者（ParamsSchemaDialog）──保存──> luaContent 注释 ──外层保存──> 后端存 luaContent
                                          │
        ┌─────────────────────────────────┼─────────────────────────────┐
        ▼                                 ▼                               ▼
  外层按钮计数(N)                后端校验/构造(deriveSchema)        填参表单(parse 注释)
```

`params_schema` 列删除后，schema 不再有第二份存储。

## 3. 前端

### 3.1 新组件 `ParamsSchemaDialog.vue`（my + admin 各一份）

独立模态，内部复用 `ParamsSchemaEditor`：

- 接口：`v-model:open`（显隐）+ prop `lua: string`（当前 luaContent）+ emit `saved: [lua: string]`。
- **打开**：`extractSchemaComment(props.lua)` → `parseSchema` → JSON.stringify 注入行编辑器（反序列化）。
- **保存**：行编辑器 → `parseSchema` → `upsertSchemaComment(props.lua, specs)` → `emit('saved', newLua)` → 关闭。
- **取消**：丢弃，关闭。

### 3.2 外层脚本对话框

- `my/.../ScriptEditDialog.vue` 与 `admin/.../ScriptStoreView.vue`：
  - 删掉内联 `<ParamsSchemaEditor>` 与 `paramsSchema` 载体（ref / form 字段）。
  - 改为按钮 **「编辑参数 (N)」**：N = `parseSchema(extractSchemaComment(luaContent)).length`（computed，0 时显示「未定义」）。
  - 点击打开 `ParamsSchemaDialog`，`:lua="luaContent"`，`@saved="(v) => luaContent = v"`。
  - 外层保存直接发 `luaContent`（注释已嵌入），删除原 upsert 逻辑。
  - 删除上传 .lua 时解析注释的特例（注释随 luaContent 走，计数 computed 自动反映）。

### 3.3 填参表单 `TaskCreateDialog.vue`

- `ParamsForm` 不变（仍接收 `schema` 字符串）。
- `schema` / `hasParams` 改成从 `selectedScript.luaContent` 注释解析：
  `extractSchemaComment(selectedScript.luaContent)` → `parseSchema` → JSON.stringify。
- `usableScripts` 已返回 `luaContent`，无需后端改动。

### 3.4 类型与 mock

- `types/automation.ts`（my + admin）：`AutomationScript` 删 `paramsSchema`；`ScriptInput` 删 `paramsSchema`。
- `mock/automation.ts`（my + admin）：记录删 `paramsSchema` 字段与 `deriveSchema` 辅助；seed 的 Hello World 注释已在 luaContent 内（填参表单现 parse）。

## 4. 后端：删除 params_schema 列

- `model.go`：删 `AutomationScript.ParamsSchema` 字段。GORM `AutoMigrate` 不会删旧列（残留无害，开发期重置删库即清）。
- `script_service.go`：create/update 不再推导/存储 schema；**保留保存时校验**——`deriveSchema(in.LuaContent, "")` 出错即拒（422），只是不落库。
- `task_service.go` / `plan_service.go`：原 `validateSchema(script.ParamsSchema)` 改 `_, specs, err := deriveSchema(script.LuaContent, "")`（从注释现 parse）。
- `script_service.go` `ScriptInput` 删 `ParamsSchema`；`api.go` `scriptBody` 删 `paramsSchema` 入参。
- `deriveSchema` / `parseSchema` / `extractSchemaComment` 既有实现复用，无需新增。

## 5. 错误处理

- 注释 JSON 非法：参数对话框 `parseSchema` 宽松返回空行；用户手改注释致非法 → 外层保存时后端 `deriveSchema` 报错 → 422。
- 参数对话框始终写出合法 JSON（由行序列化），正常路径不会产生非法注释。

## 6. 测试

- 后端单测：把原先用 `ScriptInput.ParamsSchema:"[...]"` 的用例改成把 schema 放进 `LuaContent` 顶部注释（Go 注释解析支持数组格式，直接 `--[[ [..] ]]`）。`deriveSchema` 校验/Lua table 渲染等用例保留。
- 前端 `pnpm build`（my & admin）必过；mock 跑通「编辑参数对话框 → 写回注释 → 创建任务填参」。

## 7. 已考虑的替代

- 让参数对话框操作中间 `paramsSchema` ref（不碰 luaContent），外层保存时再 upsert——真源仍分裂，不符合「从脚本取/写入脚本」本意，不取。
