package apptest

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 管理侧合作商全链路：创建 → 列表 → 更新 → 删除（adminToken）。
// 不触碰 /admin/partners/upload（需 S3）。
func TestPartnerAdminCRUDHTTP(t *testing.T) {
	r := setupRouter()
	admin := adminToken(t, r)

	// 创建（唯一命名）
	w := doJSON(r, "POST", "/api/v1/admin/partners", admin, map[string]interface{}{
		"name":      "apptest-partner-alpha",
		"promo_url": "https://example.com/?ref=apptest",
		"intro":     "集成测试合作商",
		"sort":      5,
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	created := decode(t, w).Data.(map[string]interface{})
	pid := int(created["id"].(float64))
	assert.Equal(t, true, created["enabled"]) // 留空默认启用

	// admin 列表（kw 过滤）能找到
	data := decode(t, doJSON(r, "GET", "/api/v1/admin/partners?kw=apptest-partner-alpha", admin, nil)).Data.(map[string]interface{})
	assert.Equal(t, float64(1), data["total"])

	// 更新（改名 + 禁用）
	disabled := false
	w = doJSON(r, "PUT", fmt.Sprintf("/api/v1/admin/partners/%d", pid), admin, map[string]interface{}{
		"name":    "apptest-partner-alpha-2",
		"enabled": &disabled,
	})
	require.Equal(t, http.StatusOK, w.Code)
	upd := decode(t, w).Data.(map[string]interface{})
	assert.Equal(t, "apptest-partner-alpha-2", upd["name"])
	assert.Equal(t, false, upd["enabled"])

	// 删除
	require.Equal(t, http.StatusOK, doJSON(r, "DELETE", fmt.Sprintf("/api/v1/admin/partners/%d", pid), admin, nil).Code)
	// 删除后再删 → 404
	assert.Equal(t, http.StatusNotFound, doJSON(r, "DELETE", fmt.Sprintf("/api/v1/admin/partners/%d", pid), admin, nil).Code)
}

// 非法请求 + 权限：缺 required → 400；坏 ID → 400；普通 staff 无权限 → 403；前台令牌打 admin → 401。
func TestPartnerAdminInvalidAndAuthHTTP(t *testing.T) {
	r := setupRouter()
	admin := adminToken(t, r)

	// 缺 required（name/promo_url）→ 400
	assert.Equal(t, http.StatusBadRequest,
		doJSON(r, "POST", "/api/v1/admin/partners", admin, map[string]interface{}{"name": "只有名字"}).Code)

	// 坏 ID → 400
	assert.Equal(t, http.StatusBadRequest,
		doJSON(r, "DELETE", "/api/v1/admin/partners/abc", admin, nil).Code)

	// 无 partner:view/manage 权限的普通 staff → 403
	createUser(t, r, admin, "staff_partner_noperm", "pass123", false)
	staffTok := login(t, r, "staff_partner_noperm", "pass123")
	assert.Equal(t, http.StatusForbidden, doJSON(r, "GET", "/api/v1/admin/partners", staffTok, nil).Code)
	assert.Equal(t, http.StatusForbidden,
		doJSON(r, "POST", "/api/v1/admin/partners", staffTok, map[string]interface{}{
			"name": "x", "promo_url": "https://example.com",
		}).Code)

	// 前台令牌打 admin 路由 → 401（身份域不匹配）
	userTok := registerUser(t, r, "13900030009")
	assert.Equal(t, http.StatusUnauthorized, doJSON(r, "GET", "/api/v1/admin/partners", userTok, nil).Code)
}

// 公开：ListPartners 只回启用项；ClickPartner 对存在/不存在/禁用一律返回成功（匿名）。
func TestPartnerPublicListAndClickHTTP(t *testing.T) {
	r := setupRouter()
	admin := adminToken(t, r)

	// 造一个启用 + 一个禁用合作商
	enabled := decode(t, doJSON(r, "POST", "/api/v1/admin/partners", admin, map[string]interface{}{
		"name": "apptest-pub-enabled", "promo_url": "https://example.com/e",
	})).Data.(map[string]interface{})
	enabledID := int(enabled["id"].(float64))

	disabled := false
	disabledRes := decode(t, doJSON(r, "POST", "/api/v1/admin/partners", admin, map[string]interface{}{
		"name": "apptest-pub-disabled", "promo_url": "https://example.com/d", "enabled": &disabled,
	})).Data.(map[string]interface{})
	disabledID := int(disabledRes["id"].(float64))

	// 公开列表只含启用项：能找到 enabled、找不到 disabled
	list := decode(t, doJSON(r, "GET", "/api/v1/partner/list", "", nil)).Data.([]interface{})
	var sawEnabled, sawDisabled bool
	for _, it := range list {
		m := it.(map[string]interface{})
		switch m["name"] {
		case "apptest-pub-enabled":
			sawEnabled = true
		case "apptest-pub-disabled":
			sawDisabled = true
		}
	}
	assert.True(t, sawEnabled, "公开列表应含启用合作商")
	assert.False(t, sawDisabled, "公开列表不应含禁用合作商")

	// 匿名点击启用项 → 200
	assert.Equal(t, http.StatusOK,
		doJSON(r, "POST", fmt.Sprintf("/api/v1/partner/%d/click", enabledID), "", map[string]string{"channel": "apptest"}).Code)
	// 点击禁用项 → 仍 200（静默跳过，不阻断前端跳转）
	assert.Equal(t, http.StatusOK,
		doJSON(r, "POST", fmt.Sprintf("/api/v1/partner/%d/click", disabledID), "", nil).Code)
	// 点击不存在的合作商 → 仍 200
	assert.Equal(t, http.StatusOK,
		doJSON(r, "POST", "/api/v1/partner/99999999/click", "", nil).Code)
	// 坏 ID → 400
	assert.Equal(t, http.StatusBadRequest,
		doJSON(r, "POST", "/api/v1/partner/abc/click", "", nil).Code)

	// 点击明细：启用项应有至少 1 条
	clicks := decode(t, doJSON(r, "GET", fmt.Sprintf("/api/v1/admin/partners/%d/clicks", enabledID), admin, nil)).Data.(map[string]interface{})
	assert.GreaterOrEqual(t, clicks["total"].(float64), float64(1))

	// 清理本测试造的数据
	doJSON(r, "DELETE", fmt.Sprintf("/api/v1/admin/partners/%d", enabledID), admin, nil)
	doJSON(r, "DELETE", fmt.Sprintf("/api/v1/admin/partners/%d", disabledID), admin, nil)
}
