package staff

import (
	"errors"
	"net/http"

	"manager-backend/framework"
	"manager-backend/framework/auth"

	"github.com/gin-gonic/gin"
)

var authenticator Authenticator

// SetAuthenticator 设置认证器（由框架内置或业务层覆盖）
func SetAuthenticator(a Authenticator) {
	authenticator = a
}

// AuthMiddleware 后台 JWT 认证中间件（优先 Authorization: Bearer，其次配置的 JWT Cookie）。
// 仅接受 staff 身份域令牌——前台 user 令牌会被拒绝；并比对令牌版本号（改密/重置后旧令牌失效）。
func AuthMiddleware() gin.HandlerFunc {
	return auth.MiddlewareConfig{
		Secret:     framework.AppConfig.JWTSecret,
		Scope:      auth.ScopeStaff,
		CookieName: framework.AppConfig.JWTCookieName,
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

// SuperuserMiddleware 超级管理员权限中间件（需要先使用AuthMiddleware）
func SuperuserMiddleware() gin.HandlerFunc {
	return PermissionMiddleware()
}

// PermissionMiddleware 基于角色权限的中间件（需要先使用AuthMiddleware）
// 传入所需的权限标识，用户拥有其中任意一个即放行；超管用户直接放行。
// 不传参数时等同于仅校验超管。
func PermissionMiddleware(requiredPerms ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("userID")
		if !exists {
			framework.Fail(c, http.StatusUnauthorized, "未授权")
			c.Abort()
			return
		}

		if authenticator == nil {
			framework.Fail(c, http.StatusInternalServerError, "认证器未初始化")
			c.Abort()
			return
		}

		isSuperuser, err := authenticator.IsSuperuser(userID.(int))
		if err != nil {
			framework.Fail(c, http.StatusUnauthorized, "用户不存在")
			c.Abort()
			return
		}

		if isSuperuser {
			c.Set("is_superuser", true)
			c.Next()
			return
		}

		if len(requiredPerms) == 0 {
			framework.Fail(c, http.StatusForbidden, "权限不足，需要超级管理员权限")
			c.Abort()
			return
		}

		hasPerm, err := RoleService.HasPermission(userID.(int), requiredPerms...)
		if err != nil {
			framework.Fail(c, http.StatusInternalServerError, "权限检查失败")
			c.Abort()
			return
		}

		if !hasPerm {
			framework.Fail(c, http.StatusForbidden, "权限不足")
			c.Abort()
			return
		}

		c.Next()
	}
}
