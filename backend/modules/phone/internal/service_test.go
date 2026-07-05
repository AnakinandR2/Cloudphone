package phone

import (
	"testing"

	"manager-backend/framework"
	"manager-backend/modules/billing"
	"manager-backend/modules/proxy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	userA = 3001
	userB = 3002
)

func TestCloudPhoneCRUDOwnedByUser(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("cloud_phones", "billing_seat_usages", "billing_entitlement_batches", "billing_ledger_entries", "billing_license_units", "proxies")
	})
	require.NoError(t, billing.GrantSeatLicensesForTest(userA, 5))
	// I2 修复后 Create 绑代理需属主校验：先给 userA 建一条真实代理。
	p, err := proxy.Create(userA, proxy.ProxyInput{Name: "px", Host: "203.0.113.9", Port: 1080})
	require.NoError(t, err)

	created, err := PhoneService.Create(userA, &CloudPhoneCreate{Name: "甲机", ProxyID: uint(p.ID)})
	require.NoError(t, err)
	assert.NotZero(t, created.ID)
	assert.Equal(t, uint(userA), created.UserID)
	assert.Equal(t, StatusCreated, created.Status) // 默认状态
	assert.Equal(t, uint(p.ID), created.ProxyID)
	id := int(created.ID)

	got, err := PhoneService.GetByID(userA, id)
	require.NoError(t, err)
	assert.Equal(t, "甲机", got.Name)

	// status 是状态机字段，前台通用 update 不得改写（I1）：请求带 Status 也应被忽略。
	updated, err := PhoneService.Update(userA, id, &CloudPhoneUpdate{Name: "甲机2", Status: StatusStopped})
	require.NoError(t, err)
	assert.Equal(t, "甲机2", updated.Name)
	assert.Equal(t, StatusCreated, updated.Status, "status 不应被前台 update 改写")

	require.NoError(t, PhoneService.Delete(userA, id)) // 无中台直删（CREATED 可删）
	_, err = PhoneService.GetByID(userA, id)
	assert.Error(t, err)
}

// 核心：用户只能看/改/删自己的云手机，访问他人的一律「不存在」。
func TestCloudPhoneIsolationBetweenUsers(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("cloud_phones", "billing_seat_usages", "billing_entitlement_batches", "billing_ledger_entries", "billing_license_units")
	})
	require.NoError(t, billing.GrantSeatLicensesForTest(userA, 5))
	require.NoError(t, billing.GrantSeatLicensesForTest(userB, 5))

	a, err := PhoneService.Create(userA, &CloudPhoneCreate{Name: "A机"})
	require.NoError(t, err)
	_, err = PhoneService.Create(userB, &CloudPhoneCreate{Name: "B机"})
	require.NoError(t, err)

	listA, totalA, err := PhoneService.GetList(userA, 1, 10, "", "", "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), totalA)
	require.Len(t, listA, 1)
	assert.Equal(t, "A机", listA[0].Name)

	_, err = PhoneService.GetByID(userB, int(a.ID))
	assert.Error(t, err)
	_, err = PhoneService.Update(userB, int(a.ID), &CloudPhoneUpdate{Name: "篡改"})
	assert.Error(t, err)
	assert.Error(t, PhoneService.Delete(userB, int(a.ID)))

	still, err := PhoneService.GetByID(userA, int(a.ID))
	require.NoError(t, err)
	assert.Equal(t, "A机", still.Name)
}

// 管理侧：看到全量实例（跨用户），支持按 userId/status 过滤，可强制删除。
func TestCloudPhoneAdminListAllFilterDelete(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("cloud_phones", "billing_seat_usages", "billing_entitlement_batches", "billing_ledger_entries", "billing_license_units")
	})
	require.NoError(t, billing.GrantSeatLicensesForTest(userA, 5))
	require.NoError(t, billing.GrantSeatLicensesForTest(userB, 5))

	a, err := PhoneService.Create(userA, &CloudPhoneCreate{Name: "A机"})
	require.NoError(t, err)
	_, err = PhoneService.Create(userB, &CloudPhoneCreate{Name: "B机"})
	require.NoError(t, err)

	all, total, err := PhoneService.AdminList(1, 10, "", "", "", 0, "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(2), total, "管理侧应看到全部用户的云手机")
	assert.Len(t, all, 2)

	// 按属主过滤
	onlyA, ta, err := PhoneService.AdminList(1, 10, "", "", "", userA, "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), ta)
	require.Len(t, onlyA, 1)
	assert.Equal(t, "A机", onlyA[0].Name)

	// 运维强制删除
	require.NoError(t, PhoneService.AdminDelete(int(a.ID)))
	_, total2, err := PhoneService.AdminList(1, 10, "", "", "", 0, "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), total2)
}

func TestCreateGatedByInstanceSeat(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("cloud_phones", "billing_seat_usages", "billing_entitlement_batches", "billing_ledger_entries", "billing_license_units")
	})
	const u = 9501
	_, err := PhoneService.Create(u, &CloudPhoneCreate{Name: "x"})
	assert.Error(t, err) // 无席位 → 拒

	require.NoError(t, billing.GrantSeatLicensesForTest(u, 1))
	_, err = PhoneService.Create(u, &CloudPhoneCreate{Name: "a"})
	require.NoError(t, err) // 1 席位 → 可创建
	_, err = PhoneService.Create(u, &CloudPhoneCreate{Name: "b"})
	assert.Error(t, err) // 超席位 → 拒
}
