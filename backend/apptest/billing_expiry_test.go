package apptest

import (
	"net/http"
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// seatTotal 打 GET /billing/overview 取 Data.seat.total（未过期席位容量）。
func seatTotal(t *testing.T, r *gin.Engine, token string) int {
	t.Helper()
	w := doJSON(r, "GET", "/api/v1/billing/overview", token, nil)
	require.Equal(t, http.StatusOK, w.Code, "overview 请求失败: %s", w.Body.String())
	resp := decode(t, w)
	require.Equal(t, 0, resp.Code, "overview 业务码非 0: %s", w.Body.String())
	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "Data 不是对象: %v", resp.Data)
	seat, ok := data["seat"].(map[string]interface{})
	require.True(t, ok, "Data.seat 不是对象: %v", data["seat"])
	total, ok := seat["total"].(float64)
	require.True(t, ok, "seat.total 不是数字: %v", seat["total"])
	return int(total)
}

// X9：到期链路 HTTP 端到端。seed 2 个未过期 seat 单元 → overview seat.total=2；
// 把其中 1 个 expire_at 回拨到 now-1s（模拟到期，不等 30 天）→ overview seat.total=1，
// 证明 activeUnits 的 expire_at>now 读时过滤生效。
func TestBillingSeatExpiryHTTP(t *testing.T) {
	r := setupRouter()

	// 唯一手机号，避免与其他测试撞用户/共享表污染。
	phone := "13911100099"
	token := registerUser(t, r, phone)

	// 从 users 表回查该用户的 user_id（注册接口用唯一手机号，回查确定）。
	var uid int64
	require.NoError(t, framework.DB.
		Raw("SELECT id FROM users WHERE phone = ?", phone).Scan(&uid).Error)
	require.NotZero(t, uid, "未查到注册用户 id")

	// 造数：给该 user 直接 INSERT 2 个 active 的 seat 单元，expire_at = now+30d。
	// 结束时按 user_id 清理自己造的行（绝不 truncate 共享表）。
	t.Cleanup(func() {
		framework.DB.Exec("DELETE FROM billing_license_units WHERE user_id = ?", uid)
		framework.DB.Exec("DELETE FROM users WHERE id = ?", uid)
	})

	now := time.Now()
	future := now.Add(30 * 24 * time.Hour)
	for i := 0; i < 2; i++ {
		require.NoError(t, framework.DB.Exec(
			`INSERT INTO billing_license_units
			 (user_id, kind, status, current_instance_id, source, source_ref, order_item_id, expire_at, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			uid, "seat", "active", "", "grant", "test:x9", 0, future, now, now,
		).Error)
	}

	// 初始：两个未过期 seat → seat.total = 2。
	require.Equal(t, 2, seatTotal(t, r, token), "seed 后应有 2 个未过期席位")

	// 回拨其中 1 个的 expire_at 到 now-1s（模拟到期，不真等时间）。status 仍 active。
	past := time.Now().Add(-1 * time.Second)
	res := framework.DB.Exec(
		`UPDATE billing_license_units SET expire_at = ?
		 WHERE id = (SELECT id FROM billing_license_units WHERE user_id = ? AND kind = ? ORDER BY id ASC LIMIT 1)`,
		past, uid, "seat",
	)
	require.NoError(t, res.Error)
	require.EqualValues(t, 1, res.RowsAffected, "应恰好回拨 1 行")

	// 再查：activeUnits 的 expire_at>now 过滤生效 → seat.total = 1。
	require.Equal(t, 1, seatTotal(t, r, token), "1 个到期后应只剩 1 个未过期席位")
}
