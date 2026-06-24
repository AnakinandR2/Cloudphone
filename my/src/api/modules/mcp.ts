import type { McpToolGroup } from '@/types/mcp'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

// MCP：工具清单（免鉴权的公开元数据，仅工具 code，与后端 MCP 注册同源）。
export default {
  getTools: () => api.get<unknown, R<McpToolGroup[]>>('mcp/tools'),
}
