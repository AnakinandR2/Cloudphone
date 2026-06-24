// Package mcp（internal）把开放 API 的能力以 MCP 工具暴露给 Claude Desktop / Cursor 等客户端。
//
// 进程内 + Streamable HTTP：在引擎根挂 /api/mcp，复用开放 API 同一套 gp_live_ 密钥鉴权
// （openapi.Authenticate），工具处理器直接调用 phone / app / automation / proxy 门面，不走 HTTP 回环。
package mcp

import (
	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/mark3labs/mcp-go/server"
	"gorm.io/gorm"
)

type mcpModule struct{}

func (m *mcpModule) Name() string { return "mcp" }

func (m *mcpModule) Init(_ *gorm.DB) error { return nil }

// RegisterRoutes 注册 /api/v1 下的公开路由：工具清单（免鉴权，仅暴露工具 code 等公开元数据，供 my 前台列出）。
// MCP Streamable 端点仍挂在引擎根 /api/mcp（见 registerMCPRoutes），与此无关。
func (m *mcpModule) RegisterRoutes(r *gin.RouterGroup, _ ...gin.HandlerFunc) {
	r.GET("/mcp/tools", func(c *gin.Context) {
		framework.OKWithData(c, toolCatalog())
	})
}

func (m *mcpModule) OnStart() error { return nil }
func (m *mcpModule) OnStop() error  { return nil }

// registerMCPRoutes 把 MCP Streamable HTTP 端点挂到引擎根 /api/mcp（与 /api/open/v1 同机制）。
// 鉴权经 WithHTTPContextFunc 从请求头取密钥注入 context，由各工具自行校验。
func registerMCPRoutes(r *gin.Engine) {
	streamable := server.NewStreamableHTTPServer(
		buildServer(),
		server.WithHTTPContextFunc(httpContextFunc),
		server.WithStateLess(true),
	)
	r.Any("/api/mcp", gin.WrapH(streamable))
	r.Any("/api/mcp/*any", gin.WrapH(streamable))
}

func init() {
	framework.GlobalModule.Register(&mcpModule{})
	framework.RegisterRootRoutes(registerMCPRoutes)
}
