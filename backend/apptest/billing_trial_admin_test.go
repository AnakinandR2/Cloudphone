package apptest

import (
	"net/http"
	"strconv"
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// billing_trial_admin_test.go 覆盖任务 7：试用资格/策略校验（TC-10-073~078）。
// 承接 billing_trial_test.go 已覆盖的 B5/X4（限领上限守卫式不超领 + 后台建策略 code 唯一 409），
// 也承接 modules/billing/internal/api_handler_test.go 已覆盖的基础 CRUD/400 坏 ID 路径；
// 本文件只补差集：eligible() 的资格判定矩阵（新用户/邀请码/手动授权/停用）+ validateItems 的
// 发放项校验矩阵（此前 internal 无任何负向单测覆盖，纯差集）。

// trialAdminCleanup 按策略 code 精确清理本文件自造的试用相关行，绝不 CleanTable 截断共享表。
func trialAdminCleanup(t *testing.T, code string) {
	t.Helper()
	db := framework.DB
	db.Exec("DELETE FROM billing_license_units WHERE source_ref = ?", "trial:"+code)
	db.Exec("DELETE FROM billing_ledger_entries WHERE reason = ?", "trial:"+code)
	db.Exec("DELETE FROM billing_trial_grants WHERE policy_id IN (SELECT id FROM billing_trial_policies WHERE code = ?)", code)
	db.Exec("DELETE FROM billing_trial_eligibilities WHERE policy_id IN (SELECT id FROM billing_trial_policies WHERE code = ?)", code)
	db.Exec("DELETE FROM billing_trial_claims WHERE policy_id IN (SELECT id FROM billing_trial_policies WHERE code = ?)", code)
	db.Exec("DELETE FROM billing_trial_policy_items WHERE policy_id IN (SELECT id FROM billing_trial_policies WHERE code = ?)", code)
	db.Exec("DELETE FROM billing_trial_policies WHERE code = ?", code)
}

// TestTrialClaimEligibilityNewUserAndPaidUser 覆盖 TC-10-073 + CP-0076（#62）：
// 策略 allow_new_user=true 时，「新用户」= 未领取过试用者；无论有无已付订单，只要未达
// 领取上限均可 claim（有已付订单不再被拒，杜绝「先买后领」的用户被购买惩罚）。
func TestTrialClaimEligibilityNewUserAndPaidUser(t *testing.T) {
	r := setupRouter()
	const code = "apptest-trial-073"
	const phoneNew = "13907000001"  // 新用户：无已付订单
	const phonePaid = "13907000002" // 已付费用户：有已付订单

	t.Cleanup(func() { trialAdminCleanup(t, code) })

	adminTok := adminToken(t, r)

	// 建策略：per_user_limit=1，allow_new_user=true，不配邀请码/手动资格，发 1 个 boot_slot 30 天。
	createBody := map[string]interface{}{
		"code":           code,
		"name":           "新用户资格测试",
		"per_user_limit": 1,
		"allow_new_user": true,
		"items": []map[string]interface{}{
			{"subject": "boot_slot", "quantity": 1, "expire_days": 30},
		},
	}
	w := doJSON(r, "POST", "/api/v1/admin/billing/trials", adminTok, createBody)
	require.Equal(t, http.StatusOK, w.Code, "建策略应成功: %s", w.Body.String())

	// --- 新用户：无已付订单 → claim 放行 ---
	newTok, newUID := registerUserWithID(t, r, phoneNew)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uint(newUID)) })

	w = doJSON(r, "POST", "/api/v1/billing/trials/"+code+"/claim", newTok, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, "新用户（无已付订单）应可领取: %s", w.Body.String())
	assert.Equal(t, 0, decode(t, w).Code)

	// --- 已付费用户：制造一笔真实已付订单（充值余额购买 seat_new，下单即扣款+履约+置 paid）---
	paidTok, paidUID := registerUserWithID(t, r, phonePaid)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uint(paidUID)) })

	wTopup := doJSON(r, "POST", "/api/v1/billing/topup", paidTok, map[string]interface{}{
		"amount_cents": 100000,
	})
	require.Equal(t, http.StatusOK, wTopup.Code, "充值失败: %s", wTopup.Body.String())

	wOrder := doJSON(r, "POST", "/api/v1/billing/orders", paidTok, map[string]interface{}{
		"biz_type":       "seat_new",
		"quantity":       1,
		"duration_value": 1,
		"pay_method":     "balance",
	})
	require.Equal(t, http.StatusOK, wOrder.Code, "下单失败: %s", wOrder.Body.String())
	orderData := decode(t, wOrder).Data.(map[string]interface{})
	order := orderData["order"].(map[string]interface{})
	require.Equal(t, "paid", order["status"], "余额购买下单即置已付，制造已付订单前置态")

	// CP-0076：allow_new_user 现定义为「未领取过试用者」，有已付订单也应可领取（不再购买惩罚）。
	w = doJSON(r, "POST", "/api/v1/billing/trials/"+code+"/claim", paidTok, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, "已付费用户也应能领取 allow_new_user 试用（CP-0076）: %s", w.Body.String())
	assert.Equal(t, 0, decode(t, w).Code)
}

// TestTrialClaimInviteCode 覆盖 TC-10-074：策略配邀请码时，正确 invite_code 放行；
// 错误/缺码 → 422「不符合领取条件」。
func TestTrialClaimInviteCode(t *testing.T) {
	r := setupRouter()
	const code = "apptest-trial-074"
	const inviteCode = "INVITE-074"
	const phoneWrong = "13907000003" // 错误邀请码
	const phoneRight = "13907000004" // 正确邀请码

	t.Cleanup(func() { trialAdminCleanup(t, code) })

	adminTok := adminToken(t, r)

	// 建策略：不允许新用户直接领（allow_new_user=false），仅靠邀请码放行。
	createBody := map[string]interface{}{
		"code":           code,
		"name":           "邀请码测试",
		"per_user_limit": 1,
		"allow_new_user": false,
		"invite_code":    inviteCode,
		"items": []map[string]interface{}{
			{"subject": "seat", "quantity": 1, "expire_days": 30},
		},
	}
	w := doJSON(r, "POST", "/api/v1/admin/billing/trials", adminTok, createBody)
	require.Equal(t, http.StatusOK, w.Code, "建策略应成功: %s", w.Body.String())

	// 错误邀请码 → 422「不符合领取条件」。
	wrongTok, wrongUID := registerUserWithID(t, r, phoneWrong)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uint(wrongUID)) })

	w = doJSON(r, "POST", "/api/v1/billing/trials/"+code+"/claim", wrongTok, map[string]string{
		"invite_code": "WRONG-CODE",
	})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, "错误邀请码应被拒: %s", w.Body.String())
	assert.Contains(t, decode(t, w).Message, "不符合领取条件")

	// 缺码（不传 invite_code）→ 同样 422。
	w = doJSON(r, "POST", "/api/v1/billing/trials/"+code+"/claim", wrongTok, map[string]string{})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, "缺邀请码应被拒: %s", w.Body.String())
	assert.Contains(t, decode(t, w).Message, "不符合领取条件")

	// 正确邀请码 → 放行。
	rightTok, rightUID := registerUserWithID(t, r, phoneRight)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uint(rightUID)) })

	w = doJSON(r, "POST", "/api/v1/billing/trials/"+code+"/claim", rightTok, map[string]string{
		"invite_code": inviteCode,
	})
	assert.Equal(t, http.StatusOK, w.Code, "正确邀请码应放行: %s", w.Body.String())
	assert.Equal(t, 0, decode(t, w).Code)
}

// TestTrialAdminGrantEligibilityThenClaim 覆盖 TC-10-075：后台手动授权资格后可 claim，
// 且 GET /admin/billing/trials/:id/grants 能看到发放记录。
func TestTrialAdminGrantEligibilityThenClaim(t *testing.T) {
	r := setupRouter()
	const code = "apptest-trial-075"
	const phone = "13907000005"

	t.Cleanup(func() { trialAdminCleanup(t, code) })

	adminTok := adminToken(t, r)

	// 建策略：不允许新用户、无邀请码，唯一放行路径是手动授权资格。
	createBody := map[string]interface{}{
		"code":           code,
		"name":           "手动授权资格测试",
		"per_user_limit": 1,
		"allow_new_user": false,
		"items": []map[string]interface{}{
			{"subject": "runtime_minute", "quantity": 600, "expire_days": 0},
		},
	}
	w := doJSON(r, "POST", "/api/v1/admin/billing/trials", adminTok, createBody)
	require.Equal(t, http.StatusOK, w.Code, "建策略应成功: %s", w.Body.String())
	policyID := int(decode(t, w).Data.(map[string]interface{})["id"].(float64))

	userTok, uid := registerUserWithID(t, r, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uint(uid)) })

	// 未授权前：既非新用户放行也无手动资格/邀请码 → 422。
	w = doJSON(r, "POST", "/api/v1/billing/trials/"+code+"/claim", userTok, map[string]string{})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, "未授权前不应放行: %s", w.Body.String())

	// 后台手动授权资格。
	w = doJSON(r, "POST", "/api/v1/admin/billing/trials/"+strconv.Itoa(policyID)+"/eligibility", adminTok, map[string]interface{}{
		"user_id": uid,
	})
	require.Equal(t, http.StatusOK, w.Code, "手动授权资格应成功: %s", w.Body.String())

	// 授权后：claim 放行。
	w = doJSON(r, "POST", "/api/v1/billing/trials/"+code+"/claim", userTok, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, "手动授权后应可领取: %s", w.Body.String())
	assert.Equal(t, 0, decode(t, w).Code)

	// GET /admin/billing/trials/:id/grants 应能看到刚才领取产生的发放记录。
	w = doJSON(r, "GET", "/api/v1/admin/billing/trials/"+strconv.Itoa(policyID)+"/grants", adminTok, nil)
	require.Equal(t, http.StatusOK, w.Code, "查发放记录失败: %s", w.Body.String())
	grants := decode(t, w).Data.([]interface{})
	require.Len(t, grants, 1, "应恰有 1 条发放记录")
	g := grants[0].(map[string]interface{})
	assert.Equal(t, float64(uid), g["user_id"])
	assert.Equal(t, "runtime_minute", g["subject"])
	assert.Equal(t, float64(600), g["quantity"])
}

// TestTrialClaimDisabledPolicyRejected 覆盖 TC-10-076：策略 enabled=false → claim 422「试用已停用」。
func TestTrialClaimDisabledPolicyRejected(t *testing.T) {
	r := setupRouter()
	const code = "apptest-trial-076"
	const phone = "13907000006"

	t.Cleanup(func() { trialAdminCleanup(t, code) })

	adminTok := adminToken(t, r)

	disabled := false
	createBody := map[string]interface{}{
		"code":           code,
		"name":           "停用策略测试",
		"per_user_limit": 1,
		"allow_new_user": true, // 即便资格条件放行，enabled=false 也应先被拦
		"enabled":        &disabled,
		"items": []map[string]interface{}{
			{"subject": "seat", "quantity": 1, "expire_days": 30},
		},
	}
	w := doJSON(r, "POST", "/api/v1/admin/billing/trials", adminTok, createBody)
	require.Equal(t, http.StatusOK, w.Code, "建策略应成功: %s", w.Body.String())
	assert.Equal(t, false, decode(t, w).Data.(map[string]interface{})["enabled"], "策略应落 enabled=false")

	userTok, uid := registerUserWithID(t, r, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uint(uid)) })

	w = doJSON(r, "POST", "/api/v1/billing/trials/"+code+"/claim", userTok, map[string]string{})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, "停用策略应拒绝领取: %s", w.Body.String())
	assert.Contains(t, decode(t, w).Message, "试用已停用")
}

// TestTrialAdminPolicyItemsValidation 覆盖 TC-10-078：后台建/改策略时发放项校验矩阵，
// 分别断言具体 422 文案：items 为空/科目非法/科目重复/数量≤0/天数为负。
func TestTrialAdminPolicyItemsValidation(t *testing.T) {
	r := setupRouter()
	adminTok := adminToken(t, r)

	cases := []struct {
		name    string
		items   []map[string]interface{}
		wantMsg string
	}{
		{
			name:    "items为空",
			items:   []map[string]interface{}{},
			wantMsg: "至少配置一项发放",
		},
		{
			name: "科目非法",
			items: []map[string]interface{}{
				{"subject": "not_a_real_subject", "quantity": 1, "expire_days": 30},
			},
			wantMsg: "试用只能发放资源科目",
		},
		{
			name: "科目重复",
			items: []map[string]interface{}{
				{"subject": "seat", "quantity": 1, "expire_days": 30},
				{"subject": "seat", "quantity": 2, "expire_days": 30},
			},
			wantMsg: "发放科目重复",
		},
		{
			// quantity 字段 binding:"required"：JSON 数字 0 是零值，会被 ShouldBindJSON 的
			// required 校验挡在 400「请求参数错误」，走不到业务层。用负数触发业务层的
			// 「发放数量必须大于0」422，精确命中 validateItems 的该分支。
			name: "数量非正",
			items: []map[string]interface{}{
				{"subject": "seat", "quantity": -1, "expire_days": 30},
			},
			wantMsg: "发放数量必须大于0",
		},
		{
			name: "天数为负",
			items: []map[string]interface{}{
				{"subject": "seat", "quantity": 1, "expire_days": -1},
			},
			wantMsg: "有效天数不能为负",
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code := "apptest-trial-078-" + strconv.Itoa(i)
			t.Cleanup(func() { trialAdminCleanup(t, code) })

			body := map[string]interface{}{
				"code":           code,
				"name":           "发放项校验-" + tc.name,
				"per_user_limit": 1,
				"items":          tc.items,
			}
			w := doJSON(r, "POST", "/api/v1/admin/billing/trials", adminTok, body)
			assert.Equal(t, http.StatusUnprocessableEntity, w.Code, "%s 应 422: %s", tc.name, w.Body.String())
			assert.Contains(t, decode(t, w).Message, tc.wantMsg, "%s 应命中该文案", tc.name)
		})
	}

	// TC-10-078 明确覆盖「建/改」两条路径。上面是建（POST）；此处补【改（PUT）】：
	// UpdatePolicy 复用同一 validateItems，但 AdminUpdateTrialPolicy handler + TrialPolicyUpdate.Items
	// 指针语义是另一条 HTTP 入口，必须实际调用 PUT 才算真实覆盖到改策略的发放项校验。
	t.Run("改策略PUT-科目重复", func(t *testing.T) {
		code := "apptest-trial-078-put"
		t.Cleanup(func() { trialAdminCleanup(t, code) })
		// 先建一个合法策略作基线。
		cw := doJSON(r, "POST", "/api/v1/admin/billing/trials", adminTok, map[string]interface{}{
			"code":           code,
			"name":           "改策略校验基线",
			"per_user_limit": 1,
			"items":          []map[string]interface{}{{"subject": "seat", "quantity": 1, "expire_days": 30}},
		})
		require.Equal(t, http.StatusOK, cw.Code, "建基线策略应成功: %s", cw.Body.String())
		policyID := int(decode(t, cw).Data.(map[string]interface{})["id"].(float64))
		// 改为非法发放项（科目重复）→ 应命中同一 422 文案。
		uw := doJSON(r, "PUT", "/api/v1/admin/billing/trials/"+strconv.Itoa(policyID), adminTok, map[string]interface{}{
			"items": []map[string]interface{}{
				{"subject": "seat", "quantity": 1, "expire_days": 30},
				{"subject": "seat", "quantity": 2, "expire_days": 30},
			},
		})
		assert.Equal(t, http.StatusUnprocessableEntity, uw.Code, "改策略非法发放项应 422: %s", uw.Body.String())
		assert.Contains(t, decode(t, uw).Message, "发放科目重复", "改策略应命中重复科目文案")
	})
}
