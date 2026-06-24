# my「API 与 MCP」页：引导跳转 API 文档 + 动态 MCP 工具清单 — 设计

- 日期：2026-06-25
- 状态：已评审，待实现
- 背景：my 前台「API 与 MCP」页（`my/src/views/automation/ApiMcpView.vue`）的 API 密钥卡里手写了「开放 API 基址」和「装应用/跑脚本前先调 GET /apps、GET /scripts 获取 appId/scriptId」这类零散说明。www 已有用 Scalar 渲染的完整交互式 OpenAPI 文档页 `/api-docs`，零散说明应让位于该文档。同时 MCP 服务卡里「共 18 个工具」是写死的，应改为从后端动态列出工具清单，避免与 `tools.go` 漂移。

## 1. 目标

1. API 密钥卡：删除「开放 API 基址」行与「装应用/跑脚本前…」提示，改为一条「查看完整 API 文档」外链，跳转到 www 的 `/api-docs`。
2. MCP 服务卡：从后端动态接口拉取工具清单，按四个领域分组列出工具 code；写死的「共 18 个工具」改为动态计数。
3. 新增免鉴权后端接口 `/api/v1/mcp/tools`，与 MCP 工具注册同源、不漂移。

## 2. 非目标（YAGNI）

- 不翻译工具描述、接口不返回描述（工具 code 自解释；详情留给 `/api-docs`）。
- 不改动 www `/api-docs` 页本身。
- MCP 服务卡不重复放文档外链（外链只在 API 密钥卡）。
- 不引入 my→www 的其它跨应用配置（只加文档 URL 一个）。

## 3. A. API 密钥卡改动

文件：`my/src/views/automation/ApiMcpView.vue`、`my/src/locales/{zh-CN,en}.ts`、`my/.env.{development,production}`。

### 3.1 模板

- **保留** curl 示例块（`curlExample`）。其内部仍引用 `baseUrl` 计算属性，故 `baseUrl` computed 保留。
- **删除**「如何调用」块中的两行：
  - 基址行 `{{ t('apimcp.baseUrl') }}：<code>{{ baseUrl }}</code>`（当前 L206）。
  - 提示行 `{{ t('apimcp.howToHint') }}`（当前 L207）。
- **新增**一条文档外链，位于 curl 块下方：

  ```vue
  <a :href="docsUrl" target="_blank" rel="noopener noreferrer"
     class="text-primary inline-flex items-center gap-1 text-xs hover:underline">
    {{ t('apimcp.apiDocs') }} <ArrowUpRight class="size-3.5" />
  </a>
  ```

  （图标用 lucide `ArrowUpRight`，加入现有 `lucide-vue-next` import。）

### 3.2 docsUrl 来源

```ts
const docsUrl = import.meta.env.VITE_DOCS_URL || '/api-docs'
```

新增环境变量 `VITE_DOCS_URL`，写入两个 env 文件并附注释：

- `my/.env.development`、`my/.env.production` 均加：

  ```
  # www 完整 API 文档（Scalar）地址。
  # 同源部署（www 在根、my 在 /my/）默认 /api-docs 即可；
  # 跨域部署时填完整地址，如 https://www.example.com/api-docs。
  VITE_DOCS_URL=/api-docs
  ```

### 3.3 i18n（zh-CN + en 同步）

- **删除** key：`apimcp.baseUrl`、`apimcp.howToHint`。
- **新增** key：`apimcp.apiDocs` = `查看完整 API 文档` / `View full API docs`。

## 4. B. MCP 服务卡：动态工具清单

文件：`my/src/views/automation/ApiMcpView.vue`、新增 `my/src/api/modules/mcp.ts`、`my/src/types/mcp.ts`、`my/src/mock/mcp.ts`、i18n。

### 4.1 数据流

- `onMounted` 调 `mcpApi.getTools()` 拉取分组清单。
- 失败/加载态**静默**：失败则清单区不渲染（不阻塞页面其余部分与 API 密钥功能）。

### 4.2 渲染

在 MCP 服务卡内（JSON 配置块下方）按 4 组渲染：

- 每组一个小标题（i18n 本地化），组内平铺工具 code，用等宽 `<code>` 小徽章样式。
- 组顺序固定：云手机 → 应用 → 脚本 → 代理（与后端返回顺序一致）。
- 新增 i18n key：
  - `apimcp.group.phone` = `云手机` / `Cloud phones`
  - `apimcp.group.app` = `应用` / `Apps`
  - `apimcp.group.script` = `脚本` / `Scripts`
  - `apimcp.group.proxy` = `代理` / `Proxies`

### 4.3 「共 N 个工具」改为动态

- `apimcp.mcpHint` 当前写死「共 18 个工具，与开放 API 能力一致。」改为带占位符：
  - zh：`共 {count} 个工具，与开放 API 能力一致。`
  - en：`{count} tools, matching the open API.`
- `count` 用拉取到清单的工具总数（求和各组 `tools.length`）。清单未加载成功时该提示可省略计数或不渲染（与 4.1 静默策略一致）。

### 4.4 前端 api 模块 / 类型 / mock

- `my/src/types/mcp.ts`：

  ```ts
  export interface McpToolGroup {
    group: 'phone' | 'app' | 'script' | 'proxy'
    tools: string[]
  }
  ```

- `my/src/api/modules/mcp.ts`（沿用现有 `R<T>` 封装；接口免鉴权但走同一 axios 实例无妨）：

  ```ts
  import type { McpToolGroup } from '@/types/mcp'
  import api from '../index'
  interface R<T> { code: number, message: string, data: T }
  export default {
    getTools: () => api.get<unknown, R<McpToolGroup[]>>('mcp/tools'),
  }
  ```

- `my/src/mock/mcp.ts`：fake `url` 写 `/v1/mcp/tools`，返回 4 组 18 个 code 的 `{code:0,message:'',data:[...]}`，供 `VITE_APP_MOCK=true` 开发用。

## 5. 后端：免鉴权工具清单接口

文件：`backend/modules/mcp/internal/tools.go`（或同包新增 `catalog.go`）、`backend/modules/mcp/internal/module.go`。

### 5.1 toolCatalog()

新增函数，复用现有分组构造器，读取每个 `mcp.Tool.Name`，输出有序分组——与 MCP 注册**同一数据源**，不会漂移：

```go
type toolGroupView struct {
    Group string   `json:"group"`
    Tools []string `json:"tools"`
}

func toolCatalog() []toolGroupView {
    names := func(ts []regTool) []string {
        out := make([]string, 0, len(ts))
        for _, t := range ts {
            out = append(out, t.tool.Name)
        }
        return out
    }
    return []toolGroupView{
        {Group: "phone", Tools: names(phoneTools())},
        {Group: "app", Tools: names(appTools())},
        {Group: "script", Tools: names(scriptTools())},
        {Group: "proxy", Tools: names(proxyTools())},
    }
}
```

### 5.2 路由

mcp 模块当前 `RegisterRoutes` 为空。在其中注册**公开 GET**（不挂任何鉴权中间件，元数据非敏感）：

```go
func (m *mcpModule) RegisterRoutes(r *gin.RouterGroup, _ ...gin.HandlerFunc) {
    r.GET("/mcp/tools", func(c *gin.Context) {
        framework.OKWithData(c, toolCatalog())  // 框架统一 {code,message,data} 成功封装
    })
}
```

最终路径 `/api/v1/mcp/tools`。响应：`{ "code": 0, "message": "", "data": [{"group":"phone","tools":["list_phones", ...]}, ...] }`（`framework.OKWithData` 即现有 openapi handler 的写法，见 `modules/openapi/internal/open_api.go`）。

## 6. 测试

### 6.1 后端漂移守卫（核心）

`backend/modules/mcp/internal/` 新增测试：

- 断言 `toolCatalog()` 展开后的 code 集合**等于** `allTools()` 注册的全部工具名（数量相等 + 名字集合逐一匹配）。任何新增/改名工具若没同步进分组即测试失败。
- 断言分组顺序为 phone/app/script/proxy 且各组非空。

### 6.2 后端接口测试

`GET /api/v1/mcp/tools`：返回 200、`code==0`、4 组、工具 code 总数 == 18、信封字段齐全。免鉴权（不带令牌也能拿到）。

### 6.3 前端

- `pnpm build`（`vue-tsc -b && vite build`）必须过。
- 本地 `VITE_APP_MOCK=true pnpm dev` 验证：API 卡只剩 curl + 文档外链（点击新标签打开 `/api-docs`）；MCP 卡按 4 组列出工具、提示显示「共 18 个工具」。

## 7. 影响面与回归点

- API 密钥的增删查、撤销、显示等功能不受影响（仅删两行说明 + 加外链）。
- `baseUrl` computed 保留（curl 仍用）；删除的是其在模板里的展示行。
- 新接口免鉴权，仅暴露工具 code（公开信息），无敏感数据泄露。
- i18n：删 2 个 key、改 1 个 key（mcpHint）、加 5 个 key（apiDocs + 4 个 group），zh/en 同步。
