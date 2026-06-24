// MCP 工具清单：按领域分组，仅含工具 code（描述/参数详情见 www /api-docs）。
export type McpToolGroupKey = 'phone' | 'app' | 'script' | 'proxy'

export interface McpToolGroup {
  group: McpToolGroupKey
  tools: string[]
}
