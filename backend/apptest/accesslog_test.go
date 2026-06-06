package apptest

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// 说明：访问日志的列表分页/过滤/排序等数据正确性由 modules/accesslog/internal 的服务单测覆盖。
// 应用级测试只验证 HTTP 路由的鉴权与基本响应（无法跨模块边界直接 seed 日志数据）。

func TestAccessLogListRequiresAuth(t *testing.T) {
	r := setupRouter()
	w := doJSON(r, "GET", "/api/v1/access-log/list", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAccessLogListForbiddenForNormalUser(t *testing.T) {
	r := setupRouter()
	admin := adminToken(t, r)
	createUser(t, r, admin, "al_normal", "pass123", false)
	token := login(t, r, "al_normal", "pass123")

	w := doJSON(r, "GET", "/api/v1/access-log/list", token, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAccessLogListWithSuperuser(t *testing.T) {
	r := setupRouter()
	w := doJSON(r, "GET", "/api/v1/access-log/list?page=1&size=10", adminToken(t, r), nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0, decode(t, w).Code)
}

func TestAccessLogDetailRequiresAuth(t *testing.T) {
	r := setupRouter()
	w := doJSON(r, "GET", "/api/v1/access-log/1", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAccessLogDetailNotFound(t *testing.T) {
	r := setupRouter()
	w := doJSON(r, "GET", "/api/v1/access-log/999999", adminToken(t, r), nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
