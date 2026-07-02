package apptest

import (
	"fmt"
	"net/http"
	"testing"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registerUserWithID 注册前台用户并返回 (token, userID)。
// registerUser helper 只回 token，这里的 DB 断言需要 user_id，故本文件内另写一个。
func registerUserWithID(t *testing.T, r *gin.Engine, phone string) (string, int) {
	t.Helper()
	w := doJSON(r, "POST", "/api/v1/user/auth/register", "", map[string]string{
		"phone": phone, "password": "pass123",
	})
	require.Equal(t, http.StatusOK, w.Code, "注册失败: %s", w.Body.String())
	data := decode(t, w).Data.(map[string]interface{})
	return data["token"].(string), int(data["id"].(float64))
}

// TestTrialClaimOverLimitAndUniqueCodeHTTP 覆盖 B5/X4：
//   - per_user_limit=1 的试用策略，前台首领 200、再领 409（守卫式限领，不超领）；
//   - DB 侧断言 billing_trial_claims 恰 1 条、billing_license_units 恰生成 qty 条 active seat；
//   - 后台重复建同 code 策略 → 409（code 唯一约束）。
func TestTrialClaimOverLimitAndUniqueCodeHTTP(t *testing.T) {
	r := setupRouter()

	// 唯一命名，避免污染共享表 / 与其它用例冲突。
	const code = "apptest-trial-b5x4"
	phone := "13900" + fmt.Sprintf("%06d", 900501)
	const seatQty = 2

	// 只清理本用例自造的行（绝不 CleanTable 截断共享表）。
	t.Cleanup(func() {
		framework.DB.Exec("DELETE FROM billing_license_units WHERE source_ref = ?", "trial:"+code)
		framework.DB.Exec("DELETE FROM billing_trial_grants WHERE policy_id IN (SELECT id FROM billing_trial_policies WHERE code = ?)", code)
		framework.DB.Exec("DELETE FROM billing_trial_claims WHERE policy_id IN (SELECT id FROM billing_trial_policies WHERE code = ?)", code)
		framework.DB.Exec("DELETE FROM billing_trial_policy_items WHERE policy_id IN (SELECT id FROM billing_trial_policies WHERE code = ?)", code)
		framework.DB.Exec("DELETE FROM billing_trial_policies WHERE code = ?", code)
		framework.DB.Exec("DELETE FROM billing_ledger_entries WHERE reason = ?", "trial:"+code)
		framework.DB.Exec("DELETE FROM users WHERE phone = ?", phone)
	})

	admin := adminToken(t, r)
	userTok, uid := registerUserWithID(t, r, phone)

	// 后台建策略：per_user_limit=1，发放 seatQty 个 seat（30 天到期），允许新用户领取。
	createBody := map[string]interface{}{
		"code":           code,
		"name":           "上线验收-新人试用",
		"per_user_limit": 1,
		"allow_new_user": true,
		"items": []map[string]interface{}{
			{"subject": "seat", "quantity": seatQty, "expire_days": 30},
		},
	}
	w := doJSON(r, "POST", "/api/v1/admin/billing/trials", admin, createBody)
	require.Equal(t, http.StatusOK, w.Code, "建策略应成功: %s", w.Body.String())
	require.Equal(t, 0, decode(t, w).Code)

	// 前台首次领取 → 200。
	w = doJSON(r, "POST", "/api/v1/billing/trials/"+code+"/claim", userTok, map[string]string{})
	require.Equal(t, http.StatusOK, w.Code, "首次领取应成功: %s", w.Body.String())
	require.Equal(t, 0, decode(t, w).Code)

	// 再次领取 → 409（已达领取上限），不超领。
	w = doJSON(r, "POST", "/api/v1/billing/trials/"+code+"/claim", userTok, map[string]string{})
	assert.Equal(t, http.StatusConflict, w.Code, "二次领取应被拒: %s", w.Body.String())

	// DB 断言：领取头恰 1 条。
	var claims int64
	require.NoError(t, framework.DB.
		Raw("SELECT COUNT(*) FROM billing_trial_claims WHERE policy_id IN (SELECT id FROM billing_trial_policies WHERE code = ?) AND user_id = ?", code, uid).
		Scan(&claims).Error)
	assert.Equal(t, int64(1), claims, "限领 1：只应有 1 条领取头")

	// DB 断言：仅生成 seatQty 条 active seat 授权单元（不超领）。
	var seats int64
	require.NoError(t, framework.DB.
		Raw("SELECT COUNT(*) FROM billing_license_units WHERE user_id = ? AND kind = ? AND status = ? AND source_ref = ?", uid, "seat", "active", "trial:"+code).
		Scan(&seats).Error)
	assert.Equal(t, int64(seatQty), seats, "应恰生成 %d 条 active seat 授权单元", seatQty)

	// 后台重复建同 code 策略 → 409（code 唯一约束 / 服务层冲突守卫）。
	w = doJSON(r, "POST", "/api/v1/admin/billing/trials", admin, createBody)
	assert.Equal(t, http.StatusConflict, w.Code, "重复 code 应冲突: %s", w.Body.String())
}
