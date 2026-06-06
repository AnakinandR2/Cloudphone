package framework

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware 基于白名单的 CORS。
//
// 与 Cookie 鉴权配合时，绝不能用通配 "*" + Allow-Credentials（浏览器会拒绝）。
// 这里对命中白名单的请求回显其具体 Origin 并允许携带凭证；未命中则不下发 CORS 头，
// 浏览器据此拦截跨域。allowed 为空或包含 "*" 时回显任意 Origin（仅建议开发环境）。
func CORSMiddleware(allowed []string) gin.HandlerFunc {
	allowAll := len(allowed) == 0
	set := make(map[string]bool, len(allowed))
	for _, o := range allowed {
		if o == "*" {
			allowAll = true
		}
		set[o] = true
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && (allowAll || set[origin]) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers",
				"Content-Type, Authorization, X-Requested-With, Accept, Origin, Cache-Control")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// SecurityHeaders 设置一组基础安全响应头。
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Next()
	}
}
