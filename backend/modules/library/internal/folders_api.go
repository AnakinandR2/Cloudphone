package library

import (
	"net/http"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// ListFolders 文件夹列表。
func ListFolders(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	list, err := Service.ListFolders(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

// CreateFolder 新建文件夹。
func CreateFolder(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req CreateFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	fo, err := Service.CreateFolder(uid, req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, fo)
}

// UpdateFolder 重命名文件夹。
func UpdateFolder(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, ok := pathID(c)
	if !ok {
		framework.Fail(c, http.StatusBadRequest, "非法文件夹 ID")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	fo, err := Service.RenameFolder(uid, id, req.Name)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, fo)
}

// DeleteFolder 删除文件夹（拒绝删非空）。
func DeleteFolder(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, ok := pathID(c)
	if !ok {
		framework.Fail(c, http.StatusBadRequest, "非法文件夹 ID")
		return
	}
	if err := Service.DeleteFolder(uid, id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// MoveFolder 移动文件夹到新父级（parent_id=0 为根）。
func MoveFolder(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, ok := pathID(c)
	if !ok {
		framework.Fail(c, http.StatusBadRequest, "非法文件夹 ID")
		return
	}
	var req struct {
		ParentID uint `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	fo, err := Service.MoveFolder(uid, id, req.ParentID)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, fo)
}
