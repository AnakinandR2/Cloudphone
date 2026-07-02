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

// I1/X10 回归：前台经通用 /phone/update 把 status 篡改为 RECYCLED 应被服务端白名单丢弃，
// 否则可让实例逃过 listNonRecycledByUser 的席位统计（绕过席位计费）或让回收站实例逃逸清理。
// 真实 HTTP：registerUser → 直接 INSERT 一个 active seat → 建机 → PUT update{status:RECYCLED}
// → 200 且响应 data.status 保持服务端值；再查 cloud_phones 该行 status 列仍为原值（未被篡改）。
func TestPhoneUpdateStatusMassAssignmentBlockedHTTP(t *testing.T) {
	r := setupRouter()

	// 唯一手机号，避免撞库（不截断共享表）；用完删自己这一行，支持 -count>1 重跑。
	phone := "13900920401"
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM users WHERE phone = ?", phone) })
	token := registerUser(t, r, phone)

	// 由手机号回查前台用户 ID（apptest 不能 import user internal，走 framework.DB 原生查 users 表）。
	var userID int
	require.NoError(t, framework.DB.Raw("SELECT id FROM users WHERE phone = ?", phone).Scan(&userID).Error)
	require.NotZero(t, userID, "注册后应能按手机号查到用户 ID")

	// 建机需要席位：直接 INSERT 一个未过期的 active seat 授权单元（billing_license_units）。
	// 该用户初始非回收实例数 0 < 容量 1 → 允许建 1 台。
	now := time.Now()
	require.NoError(t, framework.DB.Exec(
		`INSERT INTO billing_license_units
		 (user_id, kind, status, current_instance_id, source, source_ref, order_item_id, expire_at, created_at, updated_at)
		 VALUES (?, 'seat', 'active', '', 'grant', 'test:I1', 0, ?, ?, ?)`,
		userID, now.Add(24*time.Hour), now, now,
	).Error)
	t.Cleanup(func() {
		framework.DB.Exec("DELETE FROM billing_license_units WHERE user_id = ?", userID)
	})

	// 建机（无中台配置 → 走本地档案，直接置 CREATED；不触中台）。唯一 name。
	name := fmt.Sprintf("I1机-%d", now.UnixNano())
	w := doJSON(r, "POST", "/api/v1/phone/create", token, map[string]interface{}{"name": name})
	require.Equal(t, http.StatusOK, w.Code, "建机应 200: %s", w.Body.String())
	created := decode(t, w).Data.(map[string]interface{})
	phoneID := int(created["id"].(float64))
	require.NotZero(t, phoneID)
	initialStatus := created["status"].(string) // 本地降级建机 → CREATED
	t.Cleanup(func() {
		framework.DB.Exec("DELETE FROM cloud_phones WHERE id = ?", phoneID)
	})
	require.Equal(t, "CREATED", initialStatus, "本地降级建机初始状态应为 CREATED")

	// 越权尝试：通用 update 携带 status:RECYCLED（外加合法 name 改动，确保确实进了 update 分支）。
	uw := doJSON(r, "PUT", fmt.Sprintf("/api/v1/phone/update/%d", phoneID), token, map[string]interface{}{
		"name":   name + "-改名",
		"status": "RECYCLED",
	})
	require.Equal(t, http.StatusOK, uw.Code, "update 应 200: %s", uw.Body.String())
	resp := decode(t, uw)
	require.Equal(t, 0, resp.Code)
	updated := resp.Data.(map[string]interface{})

	// 断言①：响应 data.status 未被前台篡改为 RECYCLED，保持服务端状态机的值（原始 CREATED）。
	assert.NotEqual(t, "RECYCLED", updated["status"], "前台不得通过 update 把 status 置为 RECYCLED")
	assert.Equal(t, initialStatus, updated["status"], "status 应保持服务端值，未被篡改")
	// 合法字段 name 应生效，证明 update 确实执行、仅 status 被白名单丢弃。
	assert.Equal(t, name+"-改名", updated["name"], "合法字段 name 应更新成功")

	// 断言②：直查 cloud_phones 该行 status 列仍为原值（DB 层未被写脏）。
	var dbStatus string
	require.NoError(t, framework.DB.Raw("SELECT status FROM cloud_phones WHERE id = ?", phoneID).Scan(&dbStatus).Error)
	assert.Equal(t, initialStatus, dbStatus, "DB 中 status 列不应被前台 update 篡改")
	assert.NotEqual(t, "RECYCLED", dbStatus)
}

// createProxyFor 为某前台用户建一条代理，返回其 ID（唯一命名，避免污染共享表）。
func createProxyFor(t *testing.T, r *gin.Engine, token, name, host string) int {
	t.Helper()
	w := doJSON(r, "POST", "/api/v1/proxy/create", token, map[string]interface{}{
		"name": name, "host": host, "port": 1080,
	})
	require.Equal(t, http.StatusOK, w.Code, "建代理失败: %s", w.Body.String())
	return int(decode(t, w).Data.(map[string]interface{})["id"].(float64))
}

// TestPhoneBindProxyOwnershipHTTP 云手机绑定代理必须校验属主+存在性（I2/BOLA）：
//   - userA 建机传 userB 的 proxyID → 应被拒（404，隐藏他人资源），且不得入库；
//   - userA 建机（不带代理）后 update 传 userB 的 proxyID → 应被拒（404）；
//   - 对照：userA 绑自己的 proxyA → 成功且 proxy_id 生效。
//
// 中台未配置（apptest 降级）时 Create 走本地分支直接落库，正好聚焦「绑定校验」这层。
func TestPhoneBindProxyOwnershipHTTP(t *testing.T) {
	r := setupRouter()
	phoneA, phoneB := "13900021001", "13900021002"
	t.Cleanup(func() {
		framework.DB.Exec("DELETE FROM users WHERE phone IN (?, ?)", phoneA, phoneB)
	})
	tokenA := registerUser(t, r, phoneA)
	tokenB := registerUser(t, r, phoneB)

	// 建机受席位门禁（checkSeatAvailable）约束，与本测试聚焦的「绑定代理属主校验」无关；
	// 直接给 A 授予 3 个 active seat（覆盖修复前「越权绑定误入库也占一个席位」+ 修复后
	// 两次合法建机的用量上限），仿 I1 回归测试的做法。
	var userIDA int
	require.NoError(t, framework.DB.Raw("SELECT id FROM users WHERE phone = ?", phoneA).Scan(&userIDA).Error)
	require.NotZero(t, userIDA, "注册后应能按手机号查到用户 ID")
	now := time.Now()
	for i := 0; i < 3; i++ {
		require.NoError(t, framework.DB.Exec(
			`INSERT INTO billing_license_units
			 (user_id, kind, status, current_instance_id, source, source_ref, order_item_id, expire_at, created_at, updated_at)
			 VALUES (?, 'seat', 'active', '', 'grant', ?, 0, ?, ?, ?)`,
			userIDA, fmt.Sprintf("test:I2:%d", i), now.Add(24*time.Hour), now, now,
		).Error)
	}
	t.Cleanup(func() {
		framework.DB.Exec("DELETE FROM billing_license_units WHERE user_id = ?", userIDA)
	})

	proxyA := createProxyFor(t, r, tokenA, "phonesec-proxyA", "203.0.113.21")
	proxyB := createProxyFor(t, r, tokenB, "phonesec-proxyB", "203.0.113.22")
	t.Cleanup(func() {
		framework.DB.Exec("DELETE FROM proxies WHERE id IN (?, ?)", proxyA, proxyB)
	})

	// A 建机绑 B 的代理 → 拒绝（404），且不落库。
	w := doJSON(r, "POST", "/api/v1/phone/create", tokenA, map[string]interface{}{
		"name": "phonesec-A-badbind", "proxy_id": proxyB,
	})
	assert.Equal(t, http.StatusNotFound, w.Code,
		"越权绑定他人代理必须被拒(404)，实际=%d body=%s", w.Code, w.Body.String())

	// A 建机（不带代理）→ 成功。
	w = doJSON(r, "POST", "/api/v1/phone/create", tokenA, map[string]interface{}{
		"name": "phonesec-A-phone",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	phoneID := int(decode(t, w).Data.(map[string]interface{})["id"].(float64))
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM cloud_phones WHERE id = ?", phoneID) })

	// A 把该机 update 绑 B 的代理 → 拒绝（404）。
	w = doJSON(r, "PUT", fmt.Sprintf("/api/v1/phone/update/%d", phoneID), tokenA, map[string]interface{}{
		"proxy_id": proxyB,
	})
	assert.Equal(t, http.StatusNotFound, w.Code,
		"越权 update 绑他人代理必须被拒(404)，实际=%d body=%s", w.Code, w.Body.String())

	// 对照：A 绑自己的 proxyA → 成功且生效。
	w = doJSON(r, "PUT", fmt.Sprintf("/api/v1/phone/update/%d", phoneID), tokenA, map[string]interface{}{
		"proxy_id": proxyA,
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, float64(proxyA), decode(t, w).Data.(map[string]interface{})["proxy_id"],
		"绑自己的代理应成功生效")

	// 对照：A 建机时直接绑自己的 proxyA → 成功。
	w = doJSON(r, "POST", "/api/v1/phone/create", tokenA, map[string]interface{}{
		"name": "phonesec-A-goodbind", "proxy_id": proxyA,
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	goodPhoneID := int(decode(t, w).Data.(map[string]interface{})["id"].(float64))
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM cloud_phones WHERE id = ?", goodPhoneID) })
	assert.Equal(t, float64(proxyA), decode(t, w).Data.(map[string]interface{})["proxy_id"])
}
