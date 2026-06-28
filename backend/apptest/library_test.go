package apptest

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// libOverview 取素材库概览（前台令牌），返回 data map。
func libOverview(t *testing.T, r *gin.Engine, token string) map[string]interface{} {
	t.Helper()
	w := doJSON(r, "GET", "/api/v1/library/overview", token, nil)
	require.Equal(t, http.StatusOK, w.Code, "概览失败: %s", w.Body.String())
	return decode(t, w).Data.(map[string]interface{})
}

// 端到端（modules 间集成，仅走公开 HTTP）：注册用户 → 套餐预览 → 经 billing 下单（wechat 桩即时 paid）
// → 断言订单 paid → 概览断言订阅生效（容量/到期/非免费）。再串一个升级，断言补差价>0、容量升、到期不变。
// 覆盖 billing ↔ library 接缝（RegisterBizType / quote / fulfill / meta_json 往返）。
func TestLibraryPackagePurchaseE2E(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })
	r := setupRouter()
	token := registerUser(t, r, "13700200001")

	const gib = int64(1024) * 1024 * 1024

	// 初始为免费态。
	ov := libOverview(t, r, token)
	sub := ov["subscription"].(map[string]interface{})
	assert.Equal(t, true, sub["is_free"], "新用户应为免费态")

	// --- 1) 预览 lib_new t100 / 90 天 ---
	newParams := map[string]interface{}{"tier_code": "t100", "days": 90}
	qw := doJSON(r, "POST", "/api/v1/library/package/quote", token, newParams)
	require.Equal(t, http.StatusOK, qw.Code, "套餐预览失败: %s", qw.Body.String())
	quote := decode(t, qw).Data.(map[string]interface{})
	assert.Equal(t, "new", quote["action"])
	previewTotal := int64(quote["total_cents"].(float64))
	assert.Greater(t, previewTotal, int64(0), "新购应有正价")

	// --- 2) 经 billing 下单（wechat 桩即时 paid，免余额） ---
	cw := doJSON(r, "POST", "/api/v1/billing/orders", token, map[string]interface{}{
		"biz_type":   "lib_new",
		"params":     newParams,
		"pay_method": "wechat",
	})
	require.Equal(t, http.StatusOK, cw.Code, "下单失败: %s", cw.Body.String())
	res := decode(t, cw).Data.(map[string]interface{})
	order := res["order"].(map[string]interface{})
	pay := res["pay"].(map[string]interface{})
	orderID := int(order["id"].(float64))
	// 断言订单 paid（第三方桩即时到账）。
	assert.Equal(t, "paid", pay["status"], "wechat 桩应即时 paid")
	assert.Equal(t, "lib_new", order["biz_type"])
	assert.Equal(t, previewTotal, int64(order["total_cents"].(float64)), "权威价应与预览一致")

	// 复核订单详情亦为 paid。
	gw := doJSON(r, "GET", fmt.Sprintf("/api/v1/billing/orders/%d", orderID), token, nil)
	require.Equal(t, http.StatusOK, gw.Code)
	assert.Equal(t, "paid", decode(t, gw).Data.(map[string]interface{})["status"])

	// --- 3) 概览断言订阅生效 ---
	ov = libOverview(t, r, token)
	sub = ov["subscription"].(map[string]interface{})
	usage := ov["usage"].(map[string]interface{})
	assert.Equal(t, "t100", sub["tier_code"])
	assert.Equal(t, false, sub["is_free"])
	assert.Equal(t, float64(100*gib), sub["capacity_bytes"], "t100 容量应为 100GiB")
	assert.Equal(t, float64(100*gib), usage["capacity_bytes"])
	require.NotNil(t, sub["expire_at"], "付费订阅应有到期")
	exp := parseRFC3339(t, sub["expire_at"].(string))
	wantExp := time.Now().Add(90 * 24 * time.Hour)
	assert.WithinDuration(t, wantExp, exp, 48*time.Hour, "到期应约为 now+90d")

	// --- 4) 串一个升级 t100 → t500：补差价>0、容量升到 500G、到期不变 ---
	upParams := map[string]interface{}{"tier_code": "t500"}
	uqw := doJSON(r, "POST", "/api/v1/library/package/quote", token, upParams)
	require.Equal(t, http.StatusOK, uqw.Code, "升级预览失败: %s", uqw.Body.String())
	uquote := decode(t, uqw).Data.(map[string]interface{})
	assert.Equal(t, "upgrade", uquote["action"], "选中更高月价档应判定为升级")
	upTotal := int64(uquote["total_cents"].(float64))
	assert.Greater(t, upTotal, int64(0), "升级补差价应 > 0")

	ucw := doJSON(r, "POST", "/api/v1/billing/orders", token, map[string]interface{}{
		"biz_type":   "lib_upgrade",
		"params":     upParams,
		"pay_method": "wechat",
	})
	require.Equal(t, http.StatusOK, ucw.Code, "升级下单失败: %s", ucw.Body.String())
	upRes := decode(t, ucw).Data.(map[string]interface{})
	assert.Equal(t, "paid", upRes["pay"].(map[string]interface{})["status"])

	ov = libOverview(t, r, token)
	sub = ov["subscription"].(map[string]interface{})
	assert.Equal(t, "t500", sub["tier_code"])
	assert.Equal(t, float64(500*gib), sub["capacity_bytes"], "升级后容量应为 500GiB")
	require.NotNil(t, sub["expire_at"])
	exp2 := parseRFC3339(t, sub["expire_at"].(string))
	assert.WithinDuration(t, exp, exp2, time.Minute, "升级到期应不变")
}

// presign 在未配置 S3（素材库私有桶客户端）时返回 503（契约用例）。测试环境默认无 framework.S3Library。
func TestLibraryPresignWithoutS3Returns503(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("users") })
	if framework.S3Library != nil {
		t.Skip("已配置 S3，跳过 503 契约用例")
	}
	r := setupRouter()
	token := registerUser(t, r, "13700200002")

	w := doJSON(r, "POST", "/api/v1/library/upload/presign", token, map[string]interface{}{
		"name": "a.png", "size_bytes": 1024, "mime": "image/png", "folder_id": 0,
	})
	assert.Equal(t, http.StatusServiceUnavailable, w.Code, "未配置 S3 应 503: %s", w.Body.String())
}

// parseRFC3339 解析 JSON 中的时间字符串（容错多种布局）。
func parseRFC3339(t *testing.T, s string) time.Time {
	t.Helper()
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05Z07:00"} {
		if tm, err := time.Parse(layout, s); err == nil {
			return tm
		}
	}
	require.Failf(t, "无法解析时间", "value=%s", s)
	return time.Time{}
}
