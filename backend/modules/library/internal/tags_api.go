package library

import (
	"net/http"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// ListTags 标签列表。
func ListTags(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	list, err := Service.ListTags(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

// CreateTag 新建标签。
func CreateTag(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req CreateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	t, err := Service.CreateTag(uid, req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, t)
}

// UpdateTag 重命名 / 改色标签。
func UpdateTag(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, ok := pathID(c)
	if !ok {
		framework.Fail(c, http.StatusBadRequest, "非法标签 ID")
		return
	}
	var req UpdateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	t, err := Service.UpdateTag(uid, id, req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, t)
}

// DeleteTag 删除标签。
func DeleteTag(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, ok := pathID(c)
	if !ok {
		framework.Fail(c, http.StatusBadRequest, "非法标签 ID")
		return
	}
	if err := Service.DeleteTag(uid, id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}
