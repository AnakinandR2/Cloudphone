package note

import (
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	userA = 1001
	userB = 1002
)

func TestNoteCRUDOwnedByUser(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("notes") })

	created, err := NoteService.Create(userA, &NoteCreate{Title: "甲", Content: "r"})
	require.NoError(t, err)
	assert.NotZero(t, created.ID)
	assert.Equal(t, uint(userA), created.UserID)
	id := int(created.ID)

	got, err := NoteService.GetByID(userA, id)
	require.NoError(t, err)
	assert.Equal(t, "甲", got.Title)

	updated, err := NoteService.Update(userA, id, &NoteUpdate{Title: "乙"})
	require.NoError(t, err)
	assert.Equal(t, "乙", updated.Title)

	require.NoError(t, NoteService.Delete(userA, id))
	_, err = NoteService.GetByID(userA, id)
	assert.Error(t, err)
}

// 核心：用户只能看/改/删自己的笔记，访问他人笔记一律「不存在」。
func TestNoteIsolationBetweenUsers(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("notes") })

	a, err := NoteService.Create(userA, &NoteCreate{Title: "A的笔记"})
	require.NoError(t, err)
	_, err = NoteService.Create(userB, &NoteCreate{Title: "B的笔记"})
	require.NoError(t, err)

	// 各自只看到自己的
	listA, totalA, err := NoteService.GetList(userA, 1, 10, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), totalA)
	require.Len(t, listA, 1)
	assert.Equal(t, "A的笔记", listA[0].Title)

	// B 拿不到 A 的笔记详情
	_, err = NoteService.GetByID(userB, int(a.ID))
	assert.Error(t, err)

	// B 改不动 A 的笔记
	_, err = NoteService.Update(userB, int(a.ID), &NoteUpdate{Title: "篡改"})
	assert.Error(t, err)

	// B 删不掉 A 的笔记
	assert.Error(t, NoteService.Delete(userB, int(a.ID)))

	// A 的笔记仍在且未被改
	stillA, err := NoteService.GetByID(userA, int(a.ID))
	require.NoError(t, err)
	assert.Equal(t, "A的笔记", stillA.Title)
}

func TestNoteListFilterOwnOnly(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("notes") })

	for _, n := range []string{"苹果", "香蕉", "苹果汁"} {
		_, err := NoteService.Create(userA, &NoteCreate{Title: n})
		require.NoError(t, err)
	}
	_, err := NoteService.Create(userB, &NoteCreate{Title: "苹果派"}) // 他人数据不应混入
	require.NoError(t, err)

	list, total, err := NoteService.GetList(userA, 1, 10, "苹果", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, list, 2)
}
