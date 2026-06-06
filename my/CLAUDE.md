# my（C 端用户前台 / customer）— 工程约定

由 `admin/` 复制改造而来。Vue3 + TS + Vite7 + Pinia + VueRouter4 + Tailwind v4 + shadcn-vue(reka-ui) + @tanstack/vue-table + vue-i18n + pnpm。后端 Go/Gin（:9981，BasePath `/api/v1`）。

## ⚠️ my 与 admin 的本质差异（别把 admin 的东西带回来）
- 登录是**手机号 phone + 密码**（不是 account）；有 `/register` 注册页。
- **无 SSO、无权限/角色体系**（已删除 `useAuth`、`v-auth`、`meta.auth`、权限过滤、SSO）。**不要恢复**。
- 用户接口：`/customer/auth/{login,register,logout}`、`/customer/me`（响应 `{code,message,data}`，data=AuthResult{id,phone,nickname,avatar,token} 或 Customer）。
- `stores/user.ts`：token/id/phone/nickname/avatar + `loaded` 标志；`displayName=nickname||phone`；守卫用 `loaded`（不是 permissions）判断是否拉 getInfo。
- 模块：工作台(dashboard) + 组件库(components) + **示例(demo)**；多级菜单在 `/demo/nested/*`。
- **「示例」接后端 `note` 模块**（不是 example）：`/api/v1/note/{list,:id,create,update/:id,delete/:id}`，**需登录、按 customer 属主隔离、完整增删改查**。代码在 `views/demo/note/`、`api/modules/note.ts`、`types/note.ts`、`mock/note.ts`，菜单键 `menu.note`=我的笔记。example 已删除。
- Mock 账号：手机号 `13800138000` + 任意密码（`mock/customer.ts`）。存储前缀 `my(_dev)`。

## 网络（最容易卡住，先看）
- 外网只能走代理 `10.30.201.18:3128`（TLS 拦截，自签 CA）。直连必超时。
- **别用 corepack**，用独立 `pnpm`（registry=npmmirror、`cafile=/etc/ssl/certs/ca-certificates.crt` 已配）。
- 加 shadcn 组件用 `./scripts/shadcn.sh add <c>`；新组件常带错误导入 `@lucide/vue` → 全量替换 `lucide-vue-next`。

## 构建 / pnpm
- `pnpm build` = `vue-tsc -b && vite build`，提交前必须过。
- `pnpm-workspace.yaml` 的 `allowBuilds:` 里 esbuild/vue-demi/simple-git-hooks 必须 `true`，否则 install/build 报错。

## 主题 / 样式
- **主题 token 全部在 `src/assets/index.css`**：亮/暗变量、`[data-theme=...]` 预设、`--radius`、动效 class。新增主题色/动效同步改 `types/settings.ts`、`locales/*`、`SettingsPanel.vue`。
- Tailwind v4 按钮默认无手型 → `index.css @layer base` 已全局补 `cursor:pointer`。字体 `@fontsource` 本地引入。

## reka-ui / shadcn 坑
- Switch/RadioGroup/Checkbox 用 `v-model`（modelValue）。
- RadioGroup 值不接受 boolean → 布尔用字符串代理（`'1'/'0'`）。
- Checkbox `@update:model-value` 回调 `boolean|'indeterminate'` → `(v)=>fn(v===true)`。
- Select `@update:model-value` 传 `AcceptableValue` → 形参 `unknown` 再转。

## i18n
- vue-i18n `legacy:false`。**新增文案同时加 `zh-CN.ts` 和 `en.ts`**。
- 路由 `meta.title` 是 i18n key；组件外用 `@/locales` 全局 `t()`；表格列放 `computed(()=>getCols(t))`。

## Mock（vite-plugin-fake-server）
- basename `mock-api`；fake `url` 写 `/v1/...`。baseURL：`VITE_APP_MOCK==='true'`→`/mock-api/v1`，否则 `/api/v1`（proxy 到 :9981）。
- 后端 `{code,message,data}`，code===0 成功；后端时间带纳秒 ISO → 用 `@/utils/date` 格式化。
- 某 mock 文件报错会让全部 fake 路由失效 → 落到 SPA HTML（200，~419B）；接口返回 HTML 时查 `VITE_APP_MOCK` 与 mock 报错。

## 组件 / 约定
- **DataTable**（`src/components/DataTable.vue`）：客户端 TanStack，列显示切换 + 全局搜索(`v-model:search-value`) + 分页 + 行展开 + 插槽单元格(`#cell-<id>`/`#actions`/`#filters`) + `:loading`(骨架屏)；**无排序**。`pageSize` 必须是 `[20,50,100,200]` 之一。
- 加载态统一骨架屏；**后台轮询刷新要静默**（`load(silent)` 不切骨架屏，避免表格抖动），列表行用 `:get-row-id` 按业务 id 标识以便原地 patch。
- 搜索同步 URL：`useQuerySync(state, defaults)` + DataTable `v-model:search-value`。

### 列表行操作 / 二次确认（通用约定，新列表沿用）
- **操作分级摆放**：
  - 常用/主操作行内直显；「进入」类主操作（如远程控制/连接）排在最前。
  - 次要操作（编辑、应用管理等）收进「更多」下拉——触发器用**文字「更多」**（`crud.more`）而非 `...` 图标。
  - **查看详情用行展开**（DataTable `expandable` + `#expanded`），不在操作列放「查看」。
- **二次确认按影响分级**：
  - 危险/不可逆（销毁、删除）→ **destructive 红**，就近 `Popconfirm`（默认 `tone="danger"`）。
  - 警告/可逆但有影响（关机/停止等）→ **警告色 amber**（按钮 `border-amber-500 text-amber-600 …`）+ 轻量 `Popconfirm tone="warning"` 气泡二次确认（就近弹出，**不用居中弹框**）。
  - 普通可逆（开机等）→ 无需二次确认。
- 状态机过渡态（创建中/开机中/关机中/销毁中）用**脉冲徽章**（`animate-pulse` + 描边色）并触发列表轮询；展示状态以中台实时态为准。
- 面包屑仅「有页面(path)」级别可点，当前级不可点；顶部 logo/名称点击回 `/`。
- 双层菜单 = 官方 shadcn `Sidebar` 嵌套 + 递归 `SidebarTree.vue`；新图标在 `Icon.vue` 同时加 import 和 registry 项。

## 本地验证
- `VITE_APP_MOCK=true pnpm dev`（手机号 13800138000 + 任意密码）。
- **别用 `pkill -f vite`**（会杀当前 shell）。用固定端口 + 存 PID `kill`，或 `ps -eo pid,cmd | grep vite/dist/node` 精确杀。
