package user

import (
	"errors"

	"manager-backend/framework"
	"manager-backend/framework/auth"

	"github.com/gin-gonic/gin"
)

// jwtSecret 返回前台令牌签名密钥：优先用独立的 UserJWTSecret（纵深防御），
// 未配置时回退到后台 JWTSecret（此时仅靠 Scope 隔离前后台）。
func jwtSecret() string {
	if s := framework.AppConfig.UserJWTSecret; s != "" {
		return s
	}
	return framework.AppConfig.JWTSecret
}

// UserAuth 前台鉴权中间件：仅接受 user 身份域令牌（后台 staff 令牌会被拒绝）。
// 令牌优先取 Authorization 头，其次取配置的前台 Cookie（UserJWTCookieName）；
// 并比对令牌版本号与库中当前值（登出/改密后版本递增即令旧令牌失效）。
// 通过公开门面 user.AuthMiddleware() 暴露，供其他前台模块复用。
func UserAuth() gin.HandlerFunc {
	return auth.MiddlewareConfig{
		Secret:     jwtSecret(),
		Scope:      auth.ScopeUser,
		CookieName: framework.AppConfig.UserJWTCookieName,
		Validate: func(claims *auth.Claims) error {
			current, err := Service.CurrentTokenVersion(claims.UserID)
			if err != nil {
				return err
			}
			if current != claims.TokenVersion {
				return errors.New("token version mismatch")
			}
			return nil
		},
	}.Middleware()
}
