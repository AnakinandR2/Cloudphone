package example

import (
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExampleCreate(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("example_items") })

	item, err := ExampleService.Create(&ExampleItemCreate{
		Title:   "测试标题",
		Content: "测试内容",
	})
	require.NoError(t, err)
	assert.NotZero(t, item.ID)
	assert.Equal(t, "测试标题", item.Title)
	assert.Equal(t, "测试内容", item.Content)
	assert.False(t, item.CreatedAt.IsZero())
}

func TestExampleGetByID(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("example_items") })

	created, _ := ExampleService.Create(&ExampleItemCreate{Title: "查找", Content: "按ID查找"})

	found, err := ExampleService.GetByID(int(created.ID))
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "查找", found.Title)
}

func TestExampleGetByIDNotFound(t *testing.T) {
	_, err := ExampleService.GetByID(999999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestExampleUpdate(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("example_items") })

	item, _ := ExampleService.Create(&ExampleItemCreate{Title: "旧标题", Content: "旧内容"})

	updated, err := ExampleService.Update(int(item.ID), &ExampleItemUpdate{
		Title:   "新标题",
		Content: "新内容",
	})
	require.NoError(t, err)
	assert.Equal(t, "新标题", updated.Title)
	assert.Equal(t, "新内容", updated.Content)
}

func TestExampleUpdatePartial(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("example_items") })

	item, _ := ExampleService.Create(&ExampleItemCreate{Title: "标题", Content: "内容"})

	updated, err := ExampleService.Update(int(item.ID), &ExampleItemUpdate{Title: "改了标题"})
	require.NoError(t, err)
	assert.Equal(t, "改了标题", updated.Title)
	assert.Equal(t, "内容", updated.Content)
}

func TestExampleUpdateNotFound(t *testing.T) {
	_, err := ExampleService.Update(999999, &ExampleItemUpdate{Title: "x"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestExampleDelete(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("example_items") })

	item, _ := ExampleService.Create(&ExampleItemCreate{Title: "删除", Content: "待删除"})

	err := ExampleService.Delete(int(item.ID))
	require.NoError(t, err)

	_, err = ExampleService.GetByID(int(item.ID))
	assert.Error(t, err)
}

func TestExampleDeleteNotFound(t *testing.T) {
	err := ExampleService.Delete(999999)
	assert.Error(t, err)
}

func TestExampleGetList(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("example_items") })

	for i := 0; i < 15; i++ {
		ExampleService.Create(&ExampleItemCreate{
			Title:   "item" + string(rune('a'+i)),
			Content: "content",
		})
	}

	list, total, err := ExampleService.GetList(1, 10, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(15), total)
	assert.Len(t, list, 10)

	list2, _, err := ExampleService.GetList(2, 10, "", "", "")
	require.NoError(t, err)
	assert.Len(t, list2, 5)
}

func TestExampleGetListFilter(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("example_items") })

	ExampleService.Create(&ExampleItemCreate{Title: "苹果", Content: "水果"})
	ExampleService.Create(&ExampleItemCreate{Title: "香蕉", Content: "水果"})
	ExampleService.Create(&ExampleItemCreate{Title: "苹果汁", Content: "饮料"})

	list, total, err := ExampleService.GetList(1, 10, "苹果", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, list, 2)
}

func TestExampleGetListEmpty(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("example_items") })

	list, total, err := ExampleService.GetList(1, 10, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, list)
}

func TestExampleGetListSort(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("example_items") })

	ExampleService.Create(&ExampleItemCreate{Title: "B", Content: ""})
	ExampleService.Create(&ExampleItemCreate{Title: "A", Content: ""})
	ExampleService.Create(&ExampleItemCreate{Title: "C", Content: ""})

	list, _, err := ExampleService.GetList(1, 10, "", "title", "ascending")
	require.NoError(t, err)
	assert.Equal(t, "A", list[0].Title)
	assert.Equal(t, "C", list[2].Title)
}
