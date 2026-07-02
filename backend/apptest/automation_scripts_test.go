package apptest

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// automation_scripts_test.go 覆盖 Plan B 任务 16「脚本/任务列表与 scope」（TC-14-002/003/004/
// 021/023/024/035）。脚本 CRUD/下发主体（POST /scripts、toggle、下发任务）需真实中台
// （newMidplatPort 在测试环境未配置返回 nil，requireOps 会拒绝），Plan A 已在
// modules/automation/internal 用 fakeOps 桩覆盖；本文件只聚焦「无需中台」的列表/隔离/鉴权点，
// 前置脚本数据一律用 framework.DB 直插 automation_scripts 表（只读 GET 路径不经 s.ops）。
//
// 承接关系：modules/automation/internal/api_test.go 的 TestListScriptHandlers 等已在单元层
// （fakeOps + 手工 gin.Context，不经真实路由/JWT scope）验证 handler 基本形状；本文件在 HTTP
// 集成层补齐「真实 A/B 用户令牌下的属主隔离」「后台 script:view/script:manage 的 403/401 边界」
// 「非法任务 ID 400」这些必须走完整中间件链才能验证的点。

// autoUniqueScriptName 生成本文件内跨用例不重复的脚本展示名，便于在列表响应里精确辨认。
func autoUniqueScriptName(tag string) string {
	return fmt.Sprintf("auto_script_%s_%d", tag, time.Now().UnixNano())
}

// autoInsertScript 直接向 automation_scripts 表插入一条记录（跳过需要中台的 createScript 链路），
// 返回自增 ID。store=true 时 userID 应传 0（与 AutomationScript 的属主约定一致）。
func autoInsertScript(t *testing.T, userID uint, store bool, name, status string) uint {
	t.Helper()
	res := framework.DB.Exec(
		`INSERT INTO automation_scripts (user_id, store, script_id, mid_name, name, description, version, lua_content, file_name, status, created_at, updated_at)
		 VALUES (?, ?, 0, ?, ?, '', '', 'log(1)', 'a.lua', ?, ?, ?)`,
		userID, store, name+"_mid", name, status, time.Now(), time.Now())
	require.NoError(t, res.Error, "插入脚本失败")
	var id uint
	require.NoError(t, framework.DB.Table("automation_scripts").Where("name = ?", name).Select("id").Scan(&id).Error)
	require.NotZero(t, id, "应能查到刚插入的脚本 id")
	return id
}

// autoCleanupScripts 按 id 精确删除本用例插入的脚本行，绝不 CleanTable 截断共享表。
func autoCleanupScripts(ids ...uint) {
	for _, id := range ids {
		if id != 0 {
			framework.DB.Exec("DELETE FROM automation_scripts WHERE id = ?", id)
		}
	}
}

// autoScriptNames 从 GET /automation/scripts* 系列接口的 []interface{} Data 里取出全部 name 字段，
// 便于用 assert.Contains/NotContains 断言属主隔离与状态过滤，不受并发脚本行干扰。
func autoScriptNames(t *testing.T, data interface{}) []string {
	t.Helper()
	list, ok := data.([]interface{})
	require.True(t, ok, "响应 Data 应为数组: %v", data)
	names := make([]string, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		require.True(t, ok, "列表元素应为对象: %v", item)
		names = append(names, m["name"].(string))
	}
	return names
}

// TestAutomationScripts_MyListOwnerIsolation 覆盖 TC-14-002：我的脚本列表按属主隔离，
// A 调 GET /automation/scripts 只应看到自己的脚本，既不含 B 的用户脚本，也不含商店脚本。
func TestAutomationScripts_MyListOwnerIsolation(t *testing.T) {
	r := setupRouter()
	const phoneA = "13916000001"
	const phoneB = "13916000002"
	tokenA := registerUser(t, r, phoneA)
	tokenB := registerUser(t, r, phoneB)
	uidA := userIDByPhone(t, phoneA)
	uidB := userIDByPhone(t, phoneB)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidA) })
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidB) })

	nameA := autoUniqueScriptName("mineA")
	nameB := autoUniqueScriptName("mineB")
	nameStore := autoUniqueScriptName("store")
	idA := autoInsertScript(t, uidA, false, nameA, ScriptEnabledForTest)
	idB := autoInsertScript(t, uidB, false, nameB, ScriptEnabledForTest)
	idStore := autoInsertScript(t, 0, true, nameStore, ScriptEnabledForTest)
	t.Cleanup(func() { autoCleanupScripts(idA, idB, idStore) })

	w := doJSON(r, "GET", "/api/v1/automation/scripts", tokenA, nil)
	require.Equal(t, http.StatusOK, w.Code, "我的脚本列表失败: %s", w.Body.String())
	names := autoScriptNames(t, decode(t, w).Data)

	assert.Contains(t, names, nameA, "A 的我的脚本列表应包含自己的脚本")
	assert.NotContains(t, names, nameB, "A 的我的脚本列表不应包含 B 的脚本")
	assert.NotContains(t, names, nameStore, "我的脚本列表不应混入商店脚本")

	// 对称校验：B 调同一接口只应看到自己的脚本，不含 A 的（双向隔离，而非只验证单侧）。
	wB := doJSON(r, "GET", "/api/v1/automation/scripts", tokenB, nil)
	require.Equal(t, http.StatusOK, wB.Code, "B 的我的脚本列表失败: %s", wB.Body.String())
	namesB := autoScriptNames(t, decode(t, wB).Data)
	assert.Contains(t, namesB, nameB, "B 的我的脚本列表应包含自己的脚本")
	assert.NotContains(t, namesB, nameA, "B 的我的脚本列表不应包含 A 的脚本")
}

// TestAutomationScripts_StoreListVisibleToAllUsers 覆盖 TC-14-003：脚本商店列表面向全体已登录
// 用户，返回全部 store=true 脚本；且不应包含任何用户私有脚本。
func TestAutomationScripts_StoreListVisibleToAllUsers(t *testing.T) {
	r := setupRouter()
	const phoneA = "13916000003"
	tokenA := registerUser(t, r, phoneA)
	uidA := userIDByPhone(t, phoneA)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidA) })

	nameStore := autoUniqueScriptName("store2")
	nameMine := autoUniqueScriptName("mine2")
	idStore := autoInsertScript(t, 0, true, nameStore, ScriptEnabledForTest)
	idMine := autoInsertScript(t, uidA, false, nameMine, ScriptEnabledForTest)
	t.Cleanup(func() { autoCleanupScripts(idStore, idMine) })

	w := doJSON(r, "GET", "/api/v1/automation/scripts/store", tokenA, nil)
	require.Equal(t, http.StatusOK, w.Code, "脚本商店列表失败: %s", w.Body.String())
	names := autoScriptNames(t, decode(t, w).Data)

	assert.Contains(t, names, nameStore, "商店列表应包含 store=true 脚本")
	assert.NotContains(t, names, nameMine, "商店列表不应包含用户私有脚本")
}

// TestAutomationScripts_UsableExcludesDisabled 覆盖 TC-14-004：可用脚本集 = 我的 ∪ 商店，
// 仅 status=enabled，disabled 脚本（无论我的还是商店）不应出现。
func TestAutomationScripts_UsableExcludesDisabled(t *testing.T) {
	r := setupRouter()
	const phoneA = "13916000004"
	tokenA := registerUser(t, r, phoneA)
	uidA := userIDByPhone(t, phoneA)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidA) })

	nameMineEnabled := autoUniqueScriptName("usableMineEn")
	nameMineDisabled := autoUniqueScriptName("usableMineDis")
	nameStoreEnabled := autoUniqueScriptName("usableStoreEn")
	nameStoreDisabled := autoUniqueScriptName("usableStoreDis")
	idMineEn := autoInsertScript(t, uidA, false, nameMineEnabled, ScriptEnabledForTest)
	idMineDis := autoInsertScript(t, uidA, false, nameMineDisabled, ScriptDisabledForTest)
	idStoreEn := autoInsertScript(t, 0, true, nameStoreEnabled, ScriptEnabledForTest)
	idStoreDis := autoInsertScript(t, 0, true, nameStoreDisabled, ScriptDisabledForTest)
	t.Cleanup(func() { autoCleanupScripts(idMineEn, idMineDis, idStoreEn, idStoreDis) })

	w := doJSON(r, "GET", "/api/v1/automation/scripts/usable", tokenA, nil)
	require.Equal(t, http.StatusOK, w.Code, "可用脚本列表失败: %s", w.Body.String())
	names := autoScriptNames(t, decode(t, w).Data)

	assert.Contains(t, names, nameMineEnabled, "可用脚本集应含我的启用脚本")
	assert.Contains(t, names, nameStoreEnabled, "可用脚本集应含商店启用脚本")
	assert.NotContains(t, names, nameMineDisabled, "可用脚本集不应含我的停用脚本")
	assert.NotContains(t, names, nameStoreDisabled, "可用脚本集不应含商店停用脚本")
}

// TestAutomationAdmin_StoreListRequiresScriptView 覆盖 TC-14-021：仅持 script:view 的后台
// 员工调 GET /admin/automation/store 应能看到商店脚本列表（权限守卫正确放行）。
func TestAutomationAdmin_StoreListRequiresScriptView(t *testing.T) {
	r := setupRouter()
	admin := adminToken(t, r)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	nameStore := autoUniqueScriptName("adminList")
	idStore := autoInsertScript(t, 0, true, nameStore, ScriptEnabledForTest)
	t.Cleanup(func() { autoCleanupScripts(idStore) })

	viewTok, viewStaffID, viewRoleID := govCreateStaffWithPerm(t, r, admin, suffix, "script:view")
	t.Cleanup(func() { govCleanupStaff(viewStaffID, viewRoleID) })

	w := doJSON(r, "GET", "/api/v1/admin/automation/store", viewTok, nil)
	require.Equal(t, http.StatusOK, w.Code, "script:view 应能查看商店列表: %s", w.Body.String())
	names := autoScriptNames(t, decode(t, w).Data)
	assert.Contains(t, names, nameStore, "商店列表应包含预置的商店脚本")
}

// TestAutomationAdmin_StoreManageForbiddenWithViewOnly 覆盖 TC-14-023：仅持 script:view 的员工
// POST /admin/automation/store（需要 script:manage）应被 PermissionMiddleware 拦截 → 403，
// 且响应 Message 含「权限不足」的具体拒绝文案，不允许弱化为「非 200 即可」的空断言。
func TestAutomationAdmin_StoreManageForbiddenWithViewOnly(t *testing.T) {
	r := setupRouter()
	admin := adminToken(t, r)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	viewTok, viewStaffID, viewRoleID := govCreateStaffWithPerm(t, r, admin, suffix, "script:view")
	t.Cleanup(func() { govCleanupStaff(viewStaffID, viewRoleID) })

	w := doJSON(r, "POST", "/api/v1/admin/automation/store", viewTok, map[string]interface{}{
		"name":       autoUniqueScriptName("forbiddenCreate"),
		"luaContent": "log(1)",
	})
	require.Equal(t, http.StatusForbidden, w.Code, "仅 script:view 调管理态创建应 403: %s", w.Body.String())
	assert.Contains(t, decode(t, w).Message, "权限不足", "应返回权限不足的具体拒绝文案")

	// 越权请求应被挡在 handler 之前：商店脚本表不应新增该名字的记录。
	var cnt int64
	framework.DB.Table("automation_scripts").
		Where("store = ? AND name LIKE ?", true, "auto_script_forbiddenCreate_%").
		Count(&cnt)
	assert.Equal(t, int64(0), cnt, "越权请求不应落库新脚本")
}

// TestAutomationAdmin_FrontendTokenRejectedByAdminScope 覆盖 TC-14-024：前台 user 令牌打
// /admin/automation/* 应因 scope 不符被后台鉴权中间件拒绝 → 401，具体文案「Token作用域不匹配」。
func TestAutomationAdmin_FrontendTokenRejectedByAdminScope(t *testing.T) {
	r := setupRouter()
	const phone = "13916000005"
	userTok := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	w := doJSON(r, "GET", "/api/v1/admin/automation/store", userTok, nil)
	require.Equal(t, http.StatusUnauthorized, w.Code, "前台令牌打后台自动化接口应 401: %s", w.Body.String())
	assert.Contains(t, decode(t, w).Message, "Token作用域不匹配", "应明确是 scope 不符被拒绝")
}

// TestAutomationTasks_InvalidTaskIDRejected 覆盖 TC-14-035：GET /automation/tasks/0（及非数字）
// 应在 handler 校验阶段被拒绝 → 400「任务 ID 非法」，不下钻到 Service.TaskDetail。
func TestAutomationTasks_InvalidTaskIDRejected(t *testing.T) {
	r := setupRouter()
	const phone = "13916000006"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	// 0：数值上非法（非正数）。
	w0 := doJSON(r, "GET", "/api/v1/automation/tasks/0", token, nil)
	require.Equal(t, http.StatusBadRequest, w0.Code, "任务 ID 为 0 应 400: %s", w0.Body.String())
	assert.Contains(t, decode(t, w0).Message, "任务 ID 非法", "应有明确非法文案")

	// 非数字。
	wAbc := doJSON(r, "GET", "/api/v1/automation/tasks/abc", token, nil)
	require.Equal(t, http.StatusBadRequest, wAbc.Code, "非数字任务 ID 应 400: %s", wAbc.Body.String())
	assert.Contains(t, decode(t, wAbc).Message, "任务 ID 非法", "应有明确非法文案")

	// 未登录调用同一非法 ID：401 应先于 400 之外的路径命中——这里保持不同请求各自独立，
	// 未授权单独断言，避免混淆两种失败原因。
	wUnauth := doJSON(r, "GET", "/api/v1/automation/tasks/0", "", nil)
	assert.Equal(t, http.StatusUnauthorized, wUnauth.Code, "未登录访问应先 401: %s", wUnauth.Body.String())
}

// ---- 本文件局部常量（与 modules/automation/internal 的 ScriptEnabled/ScriptDisabled 取值一致，
// apptest 不能 import 该模块 internal，这里只能复制字面量，避免和其它 apptest 文件重名）----

const (
	ScriptEnabledForTest  = "enabled"
	ScriptDisabledForTest = "disabled"
)
