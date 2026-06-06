package staff

import "manager-backend/framework/auth"

// Claims 后台令牌声明（复用平台层定义）。
type Claims = auth.Claims

// GenerateToken 生成后台（staff 身份域）JWT Token，令牌版本为 0。
// 用于不关心版本失效的场景（如测试）；登录请用 GenerateTokenWithVersion。
func GenerateToken(userID int, username string, secret string, expireHours int) (string, error) {
	return auth.Generate(userID, username, auth.ScopeStaff, 0, secret, expireHours)
}

// GenerateTokenWithVersion 生成带令牌版本的后台 JWT（登录/SSO 用，写入用户当前版本）。
func GenerateTokenWithVersion(userID int, username string, tokenVersion int, secret string, expireHours int) (string, error) {
	return auth.Generate(userID, username, auth.ScopeStaff, tokenVersion, secret, expireHours)
}

// ParseToken 解析并校验 JWT Token。
func ParseToken(tokenString string, secret string) (*Claims, error) {
	return auth.Parse(tokenString, secret)
}
