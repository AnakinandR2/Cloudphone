// Package mcp 提供把开放 API 能力暴露为 MCP 工具的服务（Streamable HTTP，挂在 /api/mcp）。
//
// 复用开放 API 同一套 API 密钥鉴权；工具经 phone/app/automation/proxy 门面委托。
// 模块实现位于 modules/mcp/internal，受 Go internal 机制保护；本包仅用于 blank-import 触发自注册。
package mcp

import _ "manager-backend/modules/mcp/internal"
