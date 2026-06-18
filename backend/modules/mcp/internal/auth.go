package mcp

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"manager-backend/modules/openapi"
)

// ctxKey 是注入到 tool context 的私有键类型，避免跨包碰撞。
type ctxKey int

const userIDKey ctxKey = iota

// httpContextFunc 在每个 MCP HTTP 请求进入时运行：从请求头解析 API 密钥，
// 解析成功则把属主 userID 注入 context；失败/缺失则原样返回（由各 tool 自行拒绝）。
//
// 复用开放 API 同一套密钥（gp_live_…）：Authorization: Bearer <key> 优先，回退 X-API-Key。
func httpContextFunc(ctx context.Context, r *http.Request) context.Context {
	key := extractKey(r)
	if key == "" {
		return ctx
	}
	uid, err := openapi.Authenticate(key)
	if err != nil {
		return ctx
	}
	return context.WithValue(ctx, userIDKey, uid)
}

// extractKey 取请求头里的明文密钥。
func extractKey(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[len("Bearer "):])
	}
	return strings.TrimSpace(r.Header.Get("X-API-Key"))
}

// userIDFrom 从 tool context 取已鉴权的属主 userID；未鉴权返回错误（工具据此返回未授权）。
func userIDFrom(ctx context.Context) (int, error) {
	uid, ok := ctx.Value(userIDKey).(int)
	if !ok || uid <= 0 {
		return 0, errors.New("未授权：请在请求头提供有效 API 密钥（Authorization: Bearer gp_live_… 或 X-API-Key）")
	}
	return uid, nil
}
