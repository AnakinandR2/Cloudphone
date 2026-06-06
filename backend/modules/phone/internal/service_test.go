package phone

import (
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	userA = 3001
	userB = 3002
)

func TestCloudPhoneCRUDOwnedByUser(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })

	created, err := PhoneService.Create(userA, &CloudPhoneCreate{Name: "甲机", Region: "上海", ProxyID: 5})
	require.NoError(t, err)
	assert.NotZero(t, created.ID)
	assert.Equal(t, uint(userA), created.UserID)
	assert.Equal(t, StatusCreated, created.Status) // 默认状态
	assert.Equal(t, uint(5), created.ProxyID)
	id := int(created.ID)

	got, err := PhoneService.GetByID(userA, id)
	require.NoError(t, err)
	assert.Equal(t, "甲机", got.Name)

	updated, err := PhoneService.Update(userA, id, &CloudPhoneUpdate{Name: "甲机2", Status: StatusStopped})
	require.NoError(t, err)
	assert.Equal(t, "甲机2", updated.Name)
	assert.Equal(t, StatusStopped, updated.Status)

	require.NoError(t, PhoneService.Delete(userA, id)) // STOPPED 可删除
	_, err = PhoneService.GetByID(userA, id)
	assert.Error(t, err)
}

// 核心：用户只能看/改/删自己的云手机，访问他人的一律「不存在」。
func TestCloudPhoneIsolationBetweenUsers(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })

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
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })

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
