package staff

import (
	"net/http"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// SetJWTCookie 将 JWT 写入 HttpOnly Cookie（与 JSON 中的 token 并行，供浏览器携带）
func SetJWTCookie(c *gin.Context, token string) {
	cfg := framework.AppConfig
	if cfg == nil || cfg.JWTCookieName == "" {
		return
	}
	maxAge := cfg.JWTExpireHours * 3600
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     cfg.JWTCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cfg.JWTCookieSecure,
	})
}
