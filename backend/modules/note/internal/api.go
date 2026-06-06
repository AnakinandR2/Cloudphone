package note

import (
	"net/http"
	"strconv"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// currentUserID 取当前登录前台用户 ID（由 user 鉴权中间件写入 context）。
func currentUserID(c *gin.Context) (int, bool) {
	v, ok := c.Get("userID")
	if !ok {
		return 0, false
	}
	id, ok := v.(int)
	return id, ok
}

// GetNoteList 我的笔记列表（分页，仅本人）
// @Summary 我的笔记列表
// @Tags 我的笔记
// @Produce json
// @Security Bearer
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(10)
// @Param title query string false "标题（模糊搜索）"
// @Param order query string false "排序字段"
// @Param sort query string false "排序方式（ascending/descending）"
// @Success 200 {object} framework.Response{data=framework.PageResponse}
// @Router /note/list [get]
func GetNoteList(c *gin.Context) {
	cid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
	list, total, err := NoteService.GetList(cid, page, size, c.Query("title"), c.Query("order"), c.Query("sort"))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// GetNote 笔记详情（仅本人）
// @Summary 笔记详情
// @Tags 我的笔记
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Success 200 {object} framework.Response{data=Note}
// @Failure 404 {object} framework.Response
// @Router /note/{id} [get]
func GetNote(c *gin.Context) {
	cid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	item, err := NoteService.GetByID(cid, id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// CreateNote 新建笔记
// @Summary 新建笔记
// @Tags 我的笔记
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body NoteCreate true "数据"
// @Success 200 {object} framework.Response{data=Note}
// @Router /note/create [post]
func CreateNote(c *gin.Context) {
	cid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req NoteCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := NoteService.Create(cid, &req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// UpdateNote 更新笔记（仅本人）
// @Summary 更新笔记
// @Tags 我的笔记
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Param body body NoteUpdate true "数据"
// @Success 200 {object} framework.Response{data=Note}
// @Router /note/update/{id} [put]
func UpdateNote(c *gin.Context) {
	cid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	var req NoteUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := NoteService.Update(cid, id, &req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// DeleteNote 删除笔记（仅本人）
// @Summary 删除笔记
// @Tags 我的笔记
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Success 200 {object} framework.Response
// @Router /note/delete/{id} [delete]
func DeleteNote(c *gin.Context) {
	cid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	if err := NoteService.Delete(cid, id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}
