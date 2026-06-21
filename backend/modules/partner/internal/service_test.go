package partner

import (
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptrBool(b bool) *bool    { return &b }
func ptrInt(i int) *int       { return &i }
func ptrStr(s string) *string { return &s }

func sampleCreate(name string, sort int, enabled bool) *PartnerCreate {
	return &PartnerCreate{
		Name:     name,
		PromoURL: "https://partner.example.com/?ref=us",
		Sort:     sort,
		Enabled:  ptrBool(enabled),
	}
}

func TestPartnerCRUDAndDefaults(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("partners") })

	// Enabled 留空默认启用。
	created, err := PartnerService.Create(&PartnerCreate{Name: "p1", PromoURL: "https://x.com"})
	require.NoError(t, err)
	assert.NotZero(t, created.ID)
	assert.True(t, created.Enabled)
	assert.Equal(t, 0, created.ClickCount)
	id := int(created.ID)

	got, err := PartnerService.GetByID(id)
	require.NoError(t, err)
	assert.Equal(t, "p1", got.Name)

	// 更新：改名 + 禁用 + 排序；指针字段可显式置空。
	updated, err := PartnerService.Update(id, &PartnerUpdate{
		Name: "p1b", Enabled: ptrBool(false), Sort: ptrInt(5), Intro: ptrStr("hi"),
	})
	require.NoError(t, err)
	assert.Equal(t, "p1b", updated.Name)
	assert.False(t, updated.Enabled)
	assert.Equal(t, 5, updated.Sort)
	assert.Equal(t, "hi", updated.Intro)

	require.NoError(t, PartnerService.Delete(id))
	_, err = PartnerService.GetByID(id)
	assert.Error(t, err)
}

// ListEnabled 仅返回启用项，且按 sort、id 排序。
func TestListEnabledFiltersAndOrders(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("partners") })

	_, _ = PartnerService.Create(sampleCreate("b", 2, true))
	_, _ = PartnerService.Create(sampleCreate("a", 1, true))
	_, _ = PartnerService.Create(sampleCreate("hidden", 0, false))

	list, err := PartnerService.ListEnabled()
	require.NoError(t, err)
	require.Len(t, list, 2)            // 禁用项被过滤
	assert.Equal(t, "a", list[0].Name) // sort=1 在前
	assert.Equal(t, "b", list[1].Name)
}

// 点击：落明细 + 原子自增 click_count；登录与匿名两种路径。
func TestRecordClick(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("partner_clicks")
		framework.CleanTable("partners")
	})

	p, err := PartnerService.Create(sampleCreate("clickable", 0, true))
	require.NoError(t, err)
	id := int(p.ID)

	// 登录点击
	ok, err := PartnerService.RecordClick(id, &PartnerClick{UserID: 42, Channel: "my", AnonymousID: "anon-1"})
	require.NoError(t, err)
	assert.True(t, ok)
	// 匿名点击（UserID=0）
	ok, err = PartnerService.RecordClick(id, &PartnerClick{UserID: 0, Channel: "www", AnonymousID: "anon-2"})
	require.NoError(t, err)
	assert.True(t, ok)

	got, err := PartnerService.GetByID(id)
	require.NoError(t, err)
	assert.Equal(t, 2, got.ClickCount) // 缓存自增两次

	clicks, total, err := PartnerService.ListClicks(id, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, clicks, 2)
}

// 点击脏数据宽容：合作商不存在或已禁用 → 跳过写入，不报错。
func TestRecordClickToleratesBadPartner(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("partner_clicks")
		framework.CleanTable("partners")
	})

	// 不存在
	ok, err := PartnerService.RecordClick(999999, &PartnerClick{Channel: "my"})
	require.NoError(t, err)
	assert.False(t, ok)

	// 已禁用
	p, _ := PartnerService.Create(sampleCreate("disabled", 0, false))
	ok, err = PartnerService.RecordClick(int(p.ID), &PartnerClick{Channel: "my"})
	require.NoError(t, err)
	assert.False(t, ok)

	_, total, err := PartnerService.ListClicks(int(p.ID), 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
}
