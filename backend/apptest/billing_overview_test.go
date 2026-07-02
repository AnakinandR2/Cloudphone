package apptest

import (
	"net/http"
	"testing"
	"time"

	"manager-backend/framework"
	"manager-backend/modules/billing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// billing_overview_test.go 覆盖计划任务 5：概览一致性与开机名额（TC-10-054/083/053）。
// 承接 Plan A：billing_expiry_test.go 已用 seatTotal() 覆盖 overview.seat.total 的到期读时过滤；
// modules/billing/internal/api_handler_test.go 的 TestGetBillingOverview_OK_WithFakePhonePort 与
// TestBootSlotInUse_MinOfRunningAndTotal 已在单元层覆盖 boot_slot.in_use=min(运行数,名额数) 的算法本身。
// 本文件在 apptest 接口层补：overview 一次性返回的 balance/seat/boot_slot/runtime_minutes 四项
// 与账本（billing_ledger_entries）、单元池（billing_license_units/license-units 接口）、运行时长钱包
// 的交叉一致性（TC-054/083），以及 CanBoot 门面的容量探针（TC-053）。

// overviewData 打 GET /billing/overview，返回 Data 顶层 map，供各字段断言复用。
func overviewData(t *testing.T, r *gin.Engine, token string) map[string]interface{} {
	t.Helper()
	w := doJSON(r, "GET", "/api/v1/billing/overview", token, nil)
	require.Equal(t, http.StatusOK, w.Code, "overview 请求失败: %s", w.Body.String())
	resp := decode(t, w)
	require.Equal(t, 0, resp.Code, "overview 业务码非 0: %s", w.Body.String())
	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "Data 不是对象: %v", resp.Data)
	return data
}

// ovwBillingLicenseUnitCount 直查 billing_license_units 表统计某用户某 kind 的未过期（active 且
// expire_at>now）单元数，作为 overview.{seat,boot_slot}.total 的独立真值来源（与接口口径分别实现，
// 交叉验证不是「读同一段代码两次」）。
func ovwBillingLicenseUnitCount(t *testing.T, uid uint, kind string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, framework.DB.Table("billing_license_units").
		Where("user_id = ? AND kind = ? AND status = ? AND expire_at > ?", uid, kind, "active", time.Now()).
		Count(&n).Error)
	return n
}

// ovwOccupiedUnitCount 直查 billing_license_units 统计某用户某 kind 未过期单元中
// current_instance_id 非空的行数，作为 overview.seat.used 的独立真值来源。
func ovwOccupiedUnitCount(t *testing.T, uid uint, kind string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, framework.DB.Table("billing_license_units").
		Where("user_id = ? AND kind = ? AND status = ? AND expire_at > ? AND current_instance_id <> ''",
			uid, kind, "active", time.Now()).
		Count(&n).Error)
	return n
}

// ovwLedgerSum 直查 billing_ledger_entries 统计某用户某 subject 的 delta 之和，作为账本口径真值。
// COALESCE 兜底：该 subject 无任何流水时 SUM 为 NULL，归一为 0，避免 Scan 到零值 int64 出错。
func ovwLedgerSum(t *testing.T, uid uint, subject string) int64 {
	t.Helper()
	var sum int64
	require.NoError(t, framework.DB.Table("billing_ledger_entries").
		Where("user_id = ? AND subject = ?", uid, subject).
		Select("COALESCE(SUM(delta), 0)").Scan(&sum).Error)
	return sum
}

// ovwRuntimeWalletRemaining 直查 billing_runtime_minute_wallets.remaining_minutes，作为
// overview.runtime_minutes_remaining 的独立真值来源。
func ovwRuntimeWalletRemaining(t *testing.T, uid uint) int64 {
	t.Helper()
	var remaining int64
	err := framework.DB.Table("billing_runtime_minute_wallets").
		Where("user_id = ?", uid).Select("remaining_minutes").Scan(&remaining).Error
	require.NoError(t, err)
	return remaining
}

// ovwInsertRunSession 直接向 run_sessions 插入一条运行中（power_off_at IS NULL）会话，驱动
// phone 模块已注册到 billing 的 SetRunningInstanceCountProvider（apptest 不能 import phone/billing
// internal，只能借真实 HTTP 未覆盖的运行态经 framework.DB 精确插行来触发生产口径，而不是重新实现
// 一份「运行数」逻辑）。logNo 必须全局唯一（run_sessions.log_no 有唯一索引）。
func ovwInsertRunSession(t *testing.T, uid uint, cpID, logNo string) {
	t.Helper()
	now := time.Now()
	require.NoError(t, framework.DB.Exec(
		`INSERT INTO run_sessions
		 (log_no, cp_id, user_id, vm_uid, power_on_at, power_off_at, session_status, power_off_reason_code, synced_at)
		 VALUES (?, ?, ?, ?, ?, NULL, ?, ?, ?)`,
		logNo, cpID, uid, "", now, "RUNNING", "", now,
	).Error)
}

// ovwCleanupRunSessions 精确删除本用例插入的 run_sessions 行（按 user_id），不截断共享表。
func ovwCleanupRunSessions(uid uint) {
	framework.DB.Exec("DELETE FROM run_sessions WHERE user_id = ?", uid)
}

// TestBillingOverviewConsistencyHTTP 覆盖 TC-10-054/083：
//   - balance_cents 与充值后的 billing_accounts.balance_cents 一致；
//   - seat.total 与未过期 seat 单元数一致；seat.used 与 current_instance_id 非空的物化占用数一致
//     （用 billing.ReconcileSeats 落座 1/2 席位，制造「部分占用」的三态场景）；
//   - boot_slot.total 与未过期 boot_slot 单元数一致；boot_slot.in_use = min(运行中台数, 名额数)，
//     用 run_sessions 直插 3 条运行中会话、只有 2 个 boot_slot 名额，验证封顶语义（承接
//     modules/billing/internal 单测已验证的算法本身，这里验证 HTTP 层与账本/单元池的口径一致）；
//   - runtime_minutes_remaining 与 billing_runtime_minute_wallets.remaining_minutes 一致；
//   - 全部四项资源分别与 billing_ledger_entries 的 adjust_grant 流水汇总（delta 之和）一致
//     （TC-083「与账本…一致」）。
func TestBillingOverviewConsistencyHTTP(t *testing.T) {
	r := setupRouter()

	const phone = "13905000001"
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)

	t.Cleanup(func() {
		ovwCleanupRunSessions(uid)
		cleanupUserAndBillingByID(t, uid)
	})

	// 1) 余额：充值 4200 分。
	wTopup := doJSON(r, "POST", "/api/v1/billing/topup", token, map[string]interface{}{
		"amount_cents": 4200,
	})
	require.Equal(t, http.StatusOK, wTopup.Code, "充值失败: %s", wTopup.Body.String())

	// 2) seat：发 2 个 seat 名额，用 ReconcileSeats 把其中 1 个落座到一个虚构实例 cp-ovw-seat-1，
	//    制造「2 个名额、1 个在用」的部分占用三态。
	require.NoError(t, billing.GrantSeatLicensesForTest(int(uid), 2))
	_, err := billing.ReconcileSeats(int(uid), []billing.InstanceRef{
		{CpID: "cp-ovw-seat-1", CreatedAt: time.Now()},
	})
	require.NoError(t, err, "ReconcileSeats 落座失败")

	// 3) boot_slot：发 2 个名额，但制造 3 台「运行中」会话（run_sessions 直插），
	//    验证 in_use 封顶到名额数（=2），而不是运行数（=3）。
	require.NoError(t, billing.GrantBootSlotLicensesForTest(int(uid), 2))
	ovwInsertRunSession(t, uid, "cp-ovw-boot-1", "ovw-log-1")
	ovwInsertRunSession(t, uid, "cp-ovw-boot-2", "ovw-log-2")
	ovwInsertRunSession(t, uid, "cp-ovw-boot-3", "ovw-log-3")

	// 4) 临时时长：发 777 分钟。
	require.NoError(t, billing.GrantRuntimeMinutesWalletForTest(int(uid), 777))

	// ---- 断言：overview 与各独立真值来源一致 ----
	data := overviewData(t, r, token)

	// balance_cents。
	balCents, ok := data["balance_cents"].(float64)
	require.True(t, ok, "balance_cents 应为数字: %v", data["balance_cents"])
	assert.Equal(t, float64(4200), balCents, "balance_cents 应等于充值额")
	var balInDB int64
	require.NoError(t, framework.DB.Table("billing_accounts").
		Where("user_id = ?", uid).Select("balance_cents").Scan(&balInDB).Error)
	assert.Equal(t, balInDB, int64(balCents), "overview.balance_cents 应与 billing_accounts 一致")

	// seat：total/used。
	seat, ok := data["seat"].(map[string]interface{})
	require.True(t, ok, "seat 应为对象: %v", data["seat"])
	seatTotalGot := int64(seat["total"].(float64))
	seatUsedGot := int64(seat["used"].(float64))
	assert.Equal(t, int64(2), seatTotalGot, "seed 2 个 seat 名额")
	assert.Equal(t, int64(1), seatUsedGot, "ReconcileSeats 落座 1 个，used 应为 1")
	assert.Equal(t, ovwBillingLicenseUnitCount(t, uid, "seat"), seatTotalGot,
		"overview.seat.total 应与未过期 seat 单元池计数一致（TC-083）")
	assert.Equal(t, ovwOccupiedUnitCount(t, uid, "seat"), seatUsedGot,
		"overview.seat.used 应与 current_instance_id 非空计数一致（TC-083）")

	// boot_slot：total/in_use。
	boot, ok := data["boot_slot"].(map[string]interface{})
	require.True(t, ok, "boot_slot 应为对象: %v", data["boot_slot"])
	bootTotalGot := int64(boot["total"].(float64))
	bootInUseGot := int64(boot["in_use"].(float64))
	assert.Equal(t, int64(2), bootTotalGot, "seed 2 个 boot_slot 名额")
	assert.Equal(t, int64(2), bootInUseGot,
		"3 台运行中但只有 2 个名额，in_use 应封顶到名额数（TC-054 min(运行数,名额数)）")
	assert.Equal(t, ovwBillingLicenseUnitCount(t, uid, "boot_slot"), bootTotalGot,
		"overview.boot_slot.total 应与未过期 boot_slot 单元池计数一致（TC-083）")

	// runtime_minutes_remaining。
	rtRemaining, ok := data["runtime_minutes_remaining"].(float64)
	require.True(t, ok, "runtime_minutes_remaining 应为数字: %v", data["runtime_minutes_remaining"])
	assert.Equal(t, float64(777), rtRemaining, "seed 777 分钟临时时长")
	assert.Equal(t, ovwRuntimeWalletRemaining(t, uid), int64(rtRemaining),
		"overview.runtime_minutes_remaining 应与钱包表余量一致（TC-083）")

	// 与账本（adjust_grant 流水汇总）一致：本用例的 seat/boot_slot/runtime_minute 均只来自
	// GrantXxxForTest（source=grant → type=adjust_grant），delta 之和应分别等于 total/发放量。
	assert.Equal(t, int64(2), ovwLedgerSum(t, uid, "seat"),
		"seat 科目账本 delta 之和应等于发放的 2 个名额（TC-083）")
	assert.Equal(t, int64(2), ovwLedgerSum(t, uid, "boot_slot"),
		"boot_slot 科目账本 delta 之和应等于发放的 2 个名额（TC-083）")
	assert.Equal(t, int64(777), ovwLedgerSum(t, uid, "runtime_minute"),
		"runtime_minute 科目账本 delta 之和应等于发放的 777 分钟（TC-083）")

	// 与 GET /billing/license-units?kind=seat 交叉核对：items 数与 total 一致，
	// 有占用的条目数与 used 一致（同一份数据的另一投影，不是重复实现）。
	wUnits := doJSON(r, "GET", "/api/v1/billing/license-units?kind=seat", token, nil)
	require.Equal(t, http.StatusOK, wUnits.Code, "license-units 请求失败: %s", wUnits.Body.String())
	unitsData := decode(t, wUnits).Data.(map[string]interface{})
	items, ok := unitsData["items"].([]interface{})
	require.True(t, ok, "items 应为数组: %v", unitsData["items"])
	assert.Equal(t, int(seatTotalGot), len(items), "license-units 列表条数应与 overview.seat.total 一致")
	occupiedInList := 0
	for _, it := range items {
		m := it.(map[string]interface{})
		if cur, _ := m["current_instance_id"].(string); cur != "" {
			occupiedInList++
		}
	}
	assert.Equal(t, int(seatUsedGot), occupiedInList,
		"license-units 列表中占用条目数应与 overview.seat.used 一致")
}

// TestCanBootCapacityGateHTTP 覆盖 TC-10-053（开机准入 billing.CanBoot 的真实契约）：
//   - 两者皆无（无临时时长、无 boot_slot 名额）→ CanBoot=false；
//   - 仅有临时时长(>0) → CanBoot=true；
//   - 仅有 boot_slot 名额(>0) → CanBoot=true。
//
// 【开放项，已记档】CanBoot 只判断「有无临时时长余量 或 有无未过期 boot_slot 名额」，
// 不减去当前在跑实例数——即 boot_slot 名额【不做并发占满准入拒绝】（超额开机的分账放到
// Settle 结算阶段处理）。因此即便名额已被运行中会话占满，CanBoot 仍放行。本用例按此真实契约
// 断言（真绑定：删掉 CanBoot 任一分支都会让某条断言变化）；用例文档 TC-10-053 标题所述
// 「并发上限」是否应做成硬门禁，属产品决策，见 docs/测试/安全回归覆盖审计的 CanBoot 开放项。
//
// billing.CanBoot 是跨模块门面导出的普通 Go 函数（非 HTTP handler），apptest 允许直接调用
// （公开门面，不越 Modulith 边界）。
func TestCanBootCapacityGateHTTP(t *testing.T) {
	r := setupRouter()

	// 本用例只需经 HTTP 注册出一个真实 user_id 造前置态；CanBoot 直接调门面，不经 HTTP 触发开机。
	const phone = "13905000002"
	registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)

	t.Cleanup(func() {
		ovwCleanupRunSessions(uid)
		cleanupUserAndBillingByID(t, uid)
	})

	// 两者皆无 → 拒绝开机。
	can, err := billing.CanBoot(int(uid))
	require.NoError(t, err)
	assert.False(t, can, "无临时时长且无 boot_slot 名额，CanBoot 应为 false")

	// 仅有临时时长（>0）→ 放行。
	require.NoError(t, billing.GrantRuntimeMinutesWalletForTest(int(uid), 10))
	can, err = billing.CanBoot(int(uid))
	require.NoError(t, err)
	assert.True(t, can, "有临时时长(>0) 时 CanBoot 应为 true")

	// 消耗掉临时时长余量，恢复到「两者皆无」基线，专注测 boot_slot 并发上限。
	require.NoError(t, framework.DB.Exec(
		"UPDATE billing_runtime_minute_wallets SET remaining_minutes = 0 WHERE user_id = ?", uid).Error)
	can, err = billing.CanBoot(int(uid))
	require.NoError(t, err)
	require.False(t, can, "回退临时时长余量至 0 后应恢复两者皆无的拒绝基线")

	// 仅有 1 个 boot_slot 名额、尚无任何运行中实例 → 放行（有空闲名额）。
	require.NoError(t, billing.GrantBootSlotLicensesForTest(int(uid), 1))
	can, err = billing.CanBoot(int(uid))
	require.NoError(t, err)
	assert.True(t, can, "有空闲 boot_slot 名额(>0) 时 CanBoot 应为 true")

	// 再插入一台运行中会话：CanBoot 根本不查 run_sessions（只看时长余量与 boot_slot 名额容量），
	// 故插会话对结果是 no-op，仍放行——此断言锁定真实契约「有名额即 true，不做并发占满拒绝」
	// （见函数头「开放项」说明）。若未来把 boot_slot 升级为硬并发门禁并让 CanBoot 减去在跑实例数，
	// 此处应随之改为 False 并同步用例文档。
	ovwInsertRunSession(t, uid, "cp-ovw-canboot-1", "ovw-canboot-log-1")
	can, err = billing.CanBoot(int(uid))
	require.NoError(t, err)
	assert.True(t, can,
		"CanBoot 不查运行态：仅凭有 boot_slot 名额即放行，插入运行中会话不改变结果")
}
