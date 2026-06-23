package note

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCtx 构造一个带 httptest recorder 的 gin.Context，可选预置 userID。
// setUser=false 模拟「未授权」（鉴权中间件没写入 userID，或写入了非 int）。
func newCtx(method, target string, body []byte, setUser bool, uid int) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, target, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	c.Request = r
	if setUser {
		c.Set("userID", uid)
	}
	return c, w
}

// decode 解出统一响应体。
func decode(t *testing.T, w *httptest.ResponseRecorder) framework.Response {
	t.Helper()
	var resp framework.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

func setParam(c *gin.Context, key, val string) {
	c.Params = append(c.Params, gin.Param{Key: key, Value: val})
}

// ---------- 未授权分支（所有 handler 共有）----------

func TestHandlersUnauthorized(t *testing.T) {
	cases := []struct {
		name    string
		method  string
		target  string
		handler gin.HandlerFunc
	}{
		{"list", http.MethodGet, "/note/list", GetNoteList},
		{"get", http.MethodGet, "/note/1", GetNote},
		{"create", http.MethodPost, "/note/create", CreateNote},
		{"update", http.MethodPut, "/note/update/1", UpdateNote},
		{"delete", http.MethodDelete, "/note/delete/1", DeleteNote},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := newCtx(tc.method, tc.target, nil, false, 0)
			tc.handler(c)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

// userID 写入了非 int 类型时，currentUserID 也应判未授权。
func TestHandlerUnauthorizedWrongType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/note/list", nil)
	c.Set("userID", "not-an-int")
	GetNoteList(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ---------- GetNoteList ----------

func TestGetNoteListHandlerOK(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("notes") })
	const uid = 7001
	_, err := NoteService.Create(uid, &NoteCreate{Title: "h1"})
	require.NoError(t, err)
	_, err = NoteService.Create(uid, &NoteCreate{Title: "h2"})
	require.NoError(t, err)

	c, w := newCtx(http.MethodGet, "/note/list?page=1&size=10&title=h&order=title&sort=ascending", nil, true, uid)
	GetNoteList(c)
	require.Equal(t, http.StatusOK, w.Code)
	resp := decode(t, w)
	assert.Equal(t, 0, resp.Code)
	// data 是 PageResponse，total 应为 2
	data, _ := resp.Data.(map[string]interface{})
	assert.Equal(t, float64(2), data["total"])
}

// 默认分页参数（page/size 缺省 + 非数字）也走 DefaultQuery 兜底。
func TestGetNoteListHandlerDefaults(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("notes") })
	const uid = 7002
	_, err := NoteService.Create(uid, &NoteCreate{Title: "x"})
	require.NoError(t, err)

	c, w := newCtx(http.MethodGet, "/note/list", nil, true, uid)
	GetNoteList(c)
	require.Equal(t, http.StatusOK, w.Code)
	data, _ := decode(t, w).Data.(map[string]interface{})
	assert.Equal(t, float64(1), data["total"])
}

// ---------- GetNote ----------

func TestGetNoteHandlerOK(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("notes") })
	const uid = 7003
	created, err := NoteService.Create(uid, &NoteCreate{Title: "详情", Content: "正文"})
	require.NoError(t, err)

	c, w := newCtx(http.MethodGet, "/note/"+strconv.Itoa(int(created.ID)), nil, true, uid)
	setParam(c, "id", strconv.Itoa(int(created.ID)))
	GetNote(c)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0, decode(t, w).Code)
}

func TestGetNoteHandlerBadID(t *testing.T) {
	c, w := newCtx(http.MethodGet, "/note/abc", nil, true, 7004)
	setParam(c, "id", "abc")
	GetNote(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetNoteHandlerNotFound(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("notes") })
	c, w := newCtx(http.MethodGet, "/note/999999", nil, true, 7005)
	setParam(c, "id", "999999")
	GetNote(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---------- CreateNote ----------

func TestCreateNoteHandlerOK(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("notes") })
	const uid = 7006
	body, _ := json.Marshal(NoteCreate{Title: "新建", Content: "c"})
	c, w := newCtx(http.MethodPost, "/note/create", body, true, uid)
	CreateNote(c)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0, decode(t, w).Code)

	// 落库验证
	list, total, err := NoteService.GetList(uid, 1, 10, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	assert.Equal(t, "新建", list[0].Title)
}

// 缺 required 的 title → 绑定失败 → 400。
func TestCreateNoteHandlerBindError(t *testing.T) {
	c, w := newCtx(http.MethodPost, "/note/create", []byte(`{"content":"无标题"}`), true, 7007)
	CreateNote(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// 非法 JSON → 绑定失败 → 400。
func TestCreateNoteHandlerInvalidJSON(t *testing.T) {
	c, w := newCtx(http.MethodPost, "/note/create", []byte(`{bad`), true, 7008)
	CreateNote(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------- UpdateNote ----------

func TestUpdateNoteHandlerOK(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("notes") })
	const uid = 7009
	created, err := NoteService.Create(uid, &NoteCreate{Title: "旧"})
	require.NoError(t, err)

	body, _ := json.Marshal(NoteUpdate{Title: "新", Content: "新正文"})
	c, w := newCtx(http.MethodPut, "/note/update/"+strconv.Itoa(int(created.ID)), body, true, uid)
	setParam(c, "id", strconv.Itoa(int(created.ID)))
	UpdateNote(c)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0, decode(t, w).Code)

	got, err := NoteService.GetByID(uid, int(created.ID))
	require.NoError(t, err)
	assert.Equal(t, "新", got.Title)
	assert.Equal(t, "新正文", got.Content)
}

func TestUpdateNoteHandlerBadID(t *testing.T) {
	body, _ := json.Marshal(NoteUpdate{Title: "x"})
	c, w := newCtx(http.MethodPut, "/note/update/abc", body, true, 7010)
	setParam(c, "id", "abc")
	UpdateNote(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateNoteHandlerInvalidJSON(t *testing.T) {
	c, w := newCtx(http.MethodPut, "/note/update/1", []byte(`{bad`), true, 7011)
	setParam(c, "id", "1")
	UpdateNote(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateNoteHandlerNotFound(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("notes") })
	body, _ := json.Marshal(NoteUpdate{Title: "x"})
	c, w := newCtx(http.MethodPut, "/note/update/999999", body, true, 7012)
	setParam(c, "id", "999999")
	UpdateNote(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---------- DeleteNote ----------

func TestDeleteNoteHandlerOK(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("notes") })
	const uid = 7013
	created, err := NoteService.Create(uid, &NoteCreate{Title: "待删"})
	require.NoError(t, err)

	c, w := newCtx(http.MethodDelete, "/note/delete/"+strconv.Itoa(int(created.ID)), nil, true, uid)
	setParam(c, "id", strconv.Itoa(int(created.ID)))
	DeleteNote(c)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0, decode(t, w).Code)

	_, err = NoteService.GetByID(uid, int(created.ID))
	assert.Error(t, err)
}

func TestDeleteNoteHandlerBadID(t *testing.T) {
	c, w := newCtx(http.MethodDelete, "/note/delete/abc", nil, true, 7014)
	setParam(c, "id", "abc")
	DeleteNote(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteNoteHandlerNotFound(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("notes") })
	c, w := newCtx(http.MethodDelete, "/note/delete/999999", nil, true, 7015)
	setParam(c, "id", "999999")
	DeleteNote(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---------- service: Update 部分更新 / 空更新分支 ----------

// Update 只传 Content 时仅更新 content，title 不变；空 NoteUpdate 时不写库但仍返回当前值。
func TestUpdatePartialAndEmpty(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("notes") })
	const uid = 7016
	created, err := NoteService.Create(uid, &NoteCreate{Title: "原标题", Content: "原内容"})
	require.NoError(t, err)
	id := int(created.ID)

	// 只更新 content
	upd, err := NoteService.Update(uid, id, &NoteUpdate{Content: "改内容"})
	require.NoError(t, err)
	assert.Equal(t, "原标题", upd.Title)
	assert.Equal(t, "改内容", upd.Content)

	// 空更新：fields 为空，不写库，返回现值
	upd2, err := NoteService.Update(uid, id, &NoteUpdate{})
	require.NoError(t, err)
	assert.Equal(t, "原标题", upd2.Title)
	assert.Equal(t, "改内容", upd2.Content)
}

// 分页 offset 生效：第二页应返回剩余项。
func TestGetListPagination(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("notes") })
	const uid = 7017
	for i := 0; i < 3; i++ {
		_, err := NoteService.Create(uid, &NoteCreate{Title: "p" + strconv.Itoa(i)})
		require.NoError(t, err)
	}
	page1, total, err := NoteService.GetList(uid, 1, 2, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, page1, 2)

	page2, _, err := NoteService.GetList(uid, 2, 2, "", "", "")
	require.NoError(t, err)
	assert.Len(t, page2, 1)
}
