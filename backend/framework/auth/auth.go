// Package auth 是平台层通用的 JWT 鉴权能力，供各身份上下文（后台 staff、前台 user）复用。
//
// 关键点：令牌带有 Scope（身份域）声明，鉴权中间件按期望的 Scope 校验，
// 因此后台令牌无法用于前台接口、反之亦然——即使共用同一签名密钥，作用域校验也能隔离二者。
// 如需更强隔离，可为不同上下文配置不同的 Secret。
package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// 身份域常量
const (
	ScopeStaff = "staff" // 后台管理人员
	ScopeUser  = "user"  // 前台用户
)

// Claims JWT 声明（含身份域 Scope 与令牌版本 TokenVersion）。
// TokenVersion 用于服务端按需失效令牌：与库中用户当前版本比对，不一致即视为失效
// （递增用户版本号即可使其此前签发的所有令牌作废，实现"强制登出/登出全部设备"）。
type Claims struct {
	UserID       int    `json:"user_id"`
	Username     string `json:"username"`
	Scope        string `json:"scope"`
	TokenVersion int    `json:"tv"`
	jwt.RegisteredClaims
}

// Generate 签发带身份域与令牌版本的 JWT。不使用版本机制时 tokenVersion 传 0。
func Generate(userID int, username, scope string, tokenVersion int, secret string, expireHours int) (string, error) {
	claims := Claims{
		UserID:       userID,
		Username:     username,
		Scope:        scope,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// Parse 校验并解析 JWT。
func Parse(tokenString, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

// MiddlewareConfig 鉴权中间件配置。
type MiddlewareConfig struct {
	Secret     string // 验签密钥
	Scope      string // 期望身份域；token 的 Scope 不匹配则拒绝（为空表示不校验）
	CookieName string // 非空时允许从该 Cookie 读取令牌（Authorization 头优先）
	// Validate 可选的附加校验（在验签+Scope 通过后调用），返回非 nil 即拒绝。
	// 典型用途：比对令牌版本号 TokenVersion 与库中当前值，实现服务端失效。
	Validate func(*Claims) error
}

// Middleware 返回鉴权中间件：校验令牌与身份域，并将身份写入 gin context
// （userID / username / scope）。失败统一返回 401。
func (cfg MiddlewareConfig) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var token string
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				abort401(c, "Token格式错误")
				return
			}
			if token = strings.TrimSpace(parts[1]); token == "" {
				abort401(c, "Token格式错误")
				return
			}
		} else if cfg.CookieName != "" {
			if v, err := c.Cookie(cfg.CookieName); err == nil {
				token = strings.TrimSpace(v)
			}
		}

		if token == "" {
			abort401(c, "未提供认证Token")
			return
		}

		claims, err := Parse(token, cfg.Secret)
		if err != nil {
			abort401(c, "Token无效或已过期")
			return
		}
		if cfg.Scope != "" && claims.Scope != cfg.Scope {
			abort401(c, "Token作用域不匹配")
			return
		}
		if cfg.Validate != nil {
			if err := cfg.Validate(claims); err != nil {
				abort401(c, "登录态已失效")
				return
			}
		}

		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("scope", claims.Scope)
		c.Next()
	}
}

func abort401(c *gin.Context, msg string) {
	framework.Fail(c, http.StatusUnauthorized, msg)
	c.Abort()
}
