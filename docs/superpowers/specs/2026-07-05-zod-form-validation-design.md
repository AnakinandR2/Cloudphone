# my / admin 引入 zod 表单校验 — 设计文档

日期：2026-07-05
状态：已批准，直接实现（用户要求跳过独立 plan 阶段）

## 目标

给 `my`（C 端）和 `admin` 两个前端引入 zod 表单校验能力，并把「组件库示例」里的表单（`views/components/ComponentsFormView.vue`）接上校验，必填字段用 `*` 标记。

## 范围

- **纳入**：两仓装 zod/vee-validate 依赖 + shadcn-vue Form 基座组件；改造示例表单；i18n 文案；schema 单测。
- **不纳入**：真实业务表单（登录/注册等）的迁移 —— 基座就绪后另起单独任务。

## 技术路线

zod + vee-validate（shadcn-vue 生态标准），而非自写 composable。

## 一、依赖

两仓 `package.json` 各新增：

- `zod`
- `vee-validate`
- `@vee-validate/zod`（提供 `toTypedSchema()`）

用独立 `pnpm`（非 corepack）安装，遵循仓库网络约定。

## 二、shadcn-vue Form 基座组件

用 `./scripts/shadcn.sh add form` 在两仓各生成 `src/components/ui/form/`：
`Form / FormField / FormItem / FormControl / FormLabel / FormMessage / FormDescription` + `useFormField`。
这是对 vee-validate `Field`/`useForm` 的封装，即「可复用基座」，后续真实表单沿用。

生成后按仓库约定检查并修正错误导入（如 `@lucide/vue` → `lucide-vue-next`）。

## 三、必填 `*` 标记 —— 方案 A（显式 prop）

`FormLabel.vue` 增加 `required?: boolean` prop；为真时在 label 文本后渲染
`<span class="text-destructive ml-0.5">*</span>`。

选 A（显式）而非「从 schema 自动推导」：后者对 `optional/default/refine` 判定脆弱、易误标。

## 四、改造 `ComponentsFormView.vue`（两仓内容保持一致）

- 用 `useForm({ validationSchema })` 替换现有 `ref` 对象与手写的 `agree` 检查分支。
- 每个字段结构：
  `<FormField name="…" #default="{ componentField, value }">`
  → `FormItem > FormLabel(:required) > FormControl(reka-ui 控件 v-bind="componentField") > FormMessage`。
- reka-ui 控件（Select / RadioGroup / Checkbox / Switch / Calendar / Input / Textarea）通过 `componentField` 绑定 `modelValue` + `onUpdate:modelValue`。
- 提交走 `handleSubmit(onValid)`：通过才 `toast.success`；失败由 `FormMessage` 就地展示，删除原 `formAgreeWarn` 手写逻辑。
- 右侧「实时预览」绑定 vee-validate 的 `values`。
- 重置按钮改用 `resetForm()`。

### 校验规则

| 字段 | 必填 | 规则 |
|---|---|---|
| name | 是 * | `z.string().min(2)` |
| email | 是 * | `z.string().email()` |
| role | 是 * | `z.enum(['admin','editor','viewer'])` |
| agree | 是 * | `z.literal(true)`（必须勾选） |
| bio | 否 | `z.string().max(200).optional()` |
| date | 否 | 可选 |
| plan | 否 | 有默认值 `'pro'`，不校验 |
| notify | 否 | 布尔，默认 `true`，不校验 |

## 五、i18n（两仓 × zh-CN + en = 4 文件）

校验文案走 i18n。schema 定义为 `computed(() => z.object({ name: z.string().min(2, t('valid.nameMin')), ... }))`，
随语言切换响应式更新。新增键（zh-CN + en 同步）：

- `valid.nameRequired` / `valid.nameMin`
- `valid.emailInvalid`
- `valid.roleRequired`
- `valid.agreeRequired`
- `valid.bioMax`

## 六、测试

两仓各加 schema 单测（vitest），直接对校验 schema `safeParse`：

- 空 name / 无效 email / 未勾选 agree / 缺 role → 断言失败
- 合法数据 → 断言通过
- 无需挂载组件

## 验收

- `pnpm build`（`vue-tsc -b && vite build`）两仓均通过。
- 示例表单：必填项显示 `*`；留空/错误邮箱/未勾选提交时就地报错、不提交；合法数据 toast 成功。
- 新单测通过。
