package openapi

import (
	"net/http"
	"strings"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// keyAuthMiddleware 用 API 密钥（Authorization: Bearer / X-API-Key）鉴权，解析到属主用户。
func keyAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := bearerToken(c)
		if raw == "" {
			framework.Fail(c, http.StatusUnauthorized, "缺少 API 密钥")
			c.Abort()
			return
		}
		uid, ok := Service.Authenticate(raw)
		if !ok {
			framework.Fail(c, http.StatusUnauthorized, "API 密钥无效或已撤销")
			c.Abort()
			return
		}
		c.Set("userID", uid) // 与各 service handler 一致
		c.Next()
	}
}

// bearerToken 从 Authorization: Bearer xxx 或 X-API-Key 取原始密钥。
func bearerToken(c *gin.Context) string {
	if h := c.GetHeader("Authorization"); h != "" {
		if strings.HasPrefix(h, "Bearer ") {
			return strings.TrimSpace(h[len("Bearer "):])
		}
		return strings.TrimSpace(h)
	}
	return strings.TrimSpace(c.GetHeader("X-API-Key"))
}
