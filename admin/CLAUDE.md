# admin（后台管理系统）— 工程约定

Vue3 + TS + Vite7 + Pinia + VueRouter4 + Tailwind v4 + shadcn-vue(reka-ui) + @tanstack/vue-table + vue-i18n + pnpm。后端 Go/Gin（:9981，BasePath `/api/v1`）。

## 网络（最容易卡住，先看）
- 外网只能走代理 `10.30.201.18:3128`（TLS 拦截，自签 CA）。直连必超时。
- **别用 corepack**（其 fetch 不走代理），用独立 `pnpm`。npm/pnpm 已配 registry=npmmirror、`cafile=/etc/ssl/certs/ca-certificates.crt`。
- Node 原生 fetch 默认不走代理。需联网的 node 脚本加 `NODE_USE_ENV_PROXY=1 NODE_EXTRA_CA_CERTS=/etc/ssl/certs/ca-certificates.crt`。
- **加 shadcn 组件用 `./scripts/shadcn.sh add <c>`**（已注入上面两个变量）。新组件常带错误导入 `@lucide/vue` → 全量替换为 `lucide-vue-next`：
  `grep -rl "@lucide/vue" src/ | xargs -r sed -i 's#@lucide/vue#lucide-vue-next#g'`

## 构建 / pnpm
- `pnpm build` = `vue-tsc -b && vite build`，提交前必须过。
- pnpm11：`pnpm-workspace.yaml` 的 `allowBuilds:` 里 esbuild/vue-demi/simple-git-hooks 必须 `true`，否则 install/build 报 `ERR_PNPM_IGNORED_BUILDS`。出现 `xxx: set this to true or false` 就改 true 再 `pnpm install`。

## 主题 / 样式
- **所有主题 token 集中在 `src/assets/index.css`**：亮/暗变量、11 个 `[data-theme=...]` 预设、`--radius`、页面动效 class。改主题只动这里；新增主题色/动效同步改 `types/settings.ts`、`locales/*`、`SettingsPanel.vue`。
- Tailwind v4：`<button>` 默认不是手型，`index.css @layer base` 已全局补 `cursor:pointer`。
- 字体 `@fontsource`（Plus Jakarta Sans + Fira Code）本地引入，不依赖 Google Fonts。

## reka-ui / shadcn 坑
- Switch/RadioGroup/Checkbox 用 `modelValue`→`v-model` 可用。
- RadioGroup 值是 `AcceptableValue`，**不接受 boolean** → 布尔用字符串代理（`'1'/'0'`）。
- Checkbox `@update:model-value` 回调是 `boolean|'indeterminate'` → 用 `(v)=>fn(v===true)`，别标 `(v:boolean)`。
- Select `@update:model-value` 传 `AcceptableValue` → 形参 `unknown`，再 `Number(v)`/`String(v)`。

## i18n
- vue-i18n `legacy:false`。**新增文案必须同时加 `zh-CN.ts` 和 `en.ts`**。
- 路由 `meta.title` 是 i18n key；组件外（守卫/工具）用 `@/locales` 的全局 `t()`。
- **表格列放 `computed(()=>getCols(t))`**，否则切语言不重渲染。

## Mock（vite-plugin-fake-server）
- basename `mock-api`；fake `url` 写 `/v1/...`（实际匹配 `/mock-api/v1/...`）。
- baseURL：`VITE_APP_MOCK==='true'` → `/mock-api/v1`；否则 `/api/v1`（proxy 到 :9981）。
- 后端响应 `{code,message,data}`，**code===0 成功**。后端时间是带纳秒 ISO 串，**必须用 `@/utils/date` 格式化**。
- 某个 mock 文件运行时报错会让全部 fake 路由失效 → 请求落到 SPA HTML（200，~419B）。接口返回 HTML 时先查 `VITE_APP_MOCK` 与 mock 报错。

## 组件 / 约定
- **DataTable**（`src/components/DataTable.vue`）：客户端 TanStack，列显示切换 + 全局搜索(`v-model:search-value`) + 分页 + 行展开(`expandable`+`#expanded`) + 插槽单元格(`#cell-<id>`/`#actions`/`#filters`) + `:loading`(骨架屏)；**无排序**(刻意)。`pageSize` 必须是页大小下拉选项 `[20,50,100,200]` 之一，否则下拉空白。列：`{id, accessorKey, header:已翻译字符串, meta:{label:i18nKey, headClass?, cellClass?}}`。
- 删除确认用就近 `Popconfirm`（确认按钮 `variant="destructive"`），不用居中弹框。
- 列表加载用 DataTable `:loading` 骨架屏；详情/展开面板也用 `Skeleton`。
- 搜索同步 URL：`useQuerySync(reactiveState, defaults)` + DataTable `v-model:search-value`。
- 面包屑：仅「本身有页面(path)」的级别可点，当前级不可点；顶部 logo/名称点击回 `/`。
- 双层菜单 = 官方 shadcn `Sidebar` 嵌套双 Sidebar + 递归 `SidebarTree.vue`；新菜单图标要在 `Icon.vue` 同时加 import 和 registry 项（未注册回退 `CircleDot`）。
- 设置系统：`settings.ts`(默认) + `stores/settings.ts`（持久化 + `apply()` 写 dark class/`data-theme`/`--radius`/locale 到 `<html>`，main.ts 启动调用）；开发态右下角 tweak 按钮。

## admin 业务特征（与 my 区分）
- 登录：**account + 密码**；有 **RBAC**（`permissions`/角色、`useAuth`、`v-auth`、`meta.auth`、菜单按权限过滤）；接口 `/auth/login`、`/auth/me`(含 permissions)。
- 模块：系统管理（用户/角色/访问日志）、示例**全量 CRUD**。Mock 账号 admin/test。
- store 字段 account/name/avatar/isSuperuser/permissions；守卫用 `permissions.length===0` 决定是否拉信息。

## 本地验证
- `VITE_APP_MOCK=true pnpm dev` 可无后端联调。
- **别用 `pkill -f vite`**（命令行含 "vite" 会杀掉当前 shell）。用固定端口 `pnpm dev --port 56xx --strictPort` + 存 PID `kill`，或 `ps -eo pid,cmd | grep vite/dist/node` 精确杀（kill pnpm 父进程杀不掉 vite 子进程，会残留占端口）。
