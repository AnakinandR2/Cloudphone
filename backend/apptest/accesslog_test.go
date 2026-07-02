package apptest

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"manager-backend/framework"

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

// TestAccessLogListSizeClamped 验证 page/size 非法值（page=0、size=-1）被钳制，
// 不会触发 LIMIT(-1) 拉全表，也不 500；返回受钳制的正常分页（默认 size=20，list 长度 ≤200）。
// S4：size 未钳制补测。
func TestAccessLogListSizeClamped(t *testing.T) {
	r := setupRouter()
	tok := adminToken(t, r)

	// 直接造 250 行日志（超过默认页 20、也超过上限 200），验证钳制生效而非拉全表。
	marker := fmt.Sprintf("/s4-clamp/%d", time.Now().UnixNano())
	now := time.Now()
	for i := 0; i < 250; i++ {
		err := framework.DB.Exec(
			`INSERT INTO access_logs (user_id, scope, username, method, path, status_code, latency_ms, client_ip, created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
			0, "staff", "s4clamp", "GET", marker, 200, 1, "127.0.0.1", now,
		).Error
		if err != nil {
			t.Fatalf("seed access_logs 失败: %v", err)
		}
	}
	t.Cleanup(func() {
		framework.DB.Exec(`DELETE FROM access_logs WHERE path = ?`, marker)
	})

	// page=0、size=-1：修复前 size=-1 会传进 Limit(-1) 拉全表（250 行），或产生负 offset。
	w := doJSON(r, "GET", "/api/v1/access-log/list?page=0&size=-1", tok, nil)
	assert.Equal(t, http.StatusOK, w.Code, "非法分页参数不应 500")

	resp := decode(t, w)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	if !assert.True(t, ok, "Data 应为分页对象") {
		return
	}
	list, ok := data["list"].([]interface{})
	if !assert.True(t, ok, "list 应为数组") {
		return
	}
	// 钳制后应回落默认页大小（20），无论如何都不得超过上限 200（更不能是全表 250）。
	assert.LessOrEqual(t, len(list), 200, "size 未钳制则会拉全表")
	assert.Equal(t, 20, len(list), "size=-1 应回落默认 20")

	// total 反映过滤后的真实总数（≥250），确认造数生效、count 正常。
	total, ok := data["total"].(float64)
	assert.True(t, ok, "total 应为数字")
	assert.GreaterOrEqual(t, total, float64(250))
}
