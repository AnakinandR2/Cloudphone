package user

import (
	"net/http"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// setCookie 将前台 JWT 写入 HttpOnly Cookie（与响应体中的 token 并行，供浏览器自动携带）。
// 仅当配置了 UserJWTCookieName 时生效；Secure 标志复用 JWTCookieSecure。
func setCookie(c *gin.Context, token string) {
	cfg := framework.AppConfig
	if cfg == nil || cfg.UserJWTCookieName == "" {
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     cfg.UserJWTCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   cfg.JWTExpireHours * 3600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cfg.JWTCookieSecure,
	})
}

// clearCookie 删除前台 JWT Cookie（MaxAge<0 令浏览器立即过期）。
func clearCookie(c *gin.Context) {
	cfg := framework.AppConfig
	if cfg == nil || cfg.UserJWTCookieName == "" {
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     cfg.UserJWTCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cfg.JWTCookieSecure,
	})
}
