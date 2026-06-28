package library

import (
	"net/http"
	"strconv"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// pathID 解析路由 :id 为 uint。
func pathID(c *gin.Context) (uint, bool) {
	v, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || v == 0 {
		return 0, false
	}
	return uint(v), true
}

// PresignUpload 上传第一段：配额校验 + 建 uploading 行 + 签发 PUT URL（§5.1）。
func PresignUpload(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req PresignUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	res, err := Service.PresignUpload(uid, req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, res)
}

// ConfirmUpload 上传第二段：HeadObject 修正真实大小 + 置 active + 累加用量 + 绑标签（§5.3）。
func ConfirmUpload(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req ConfirmUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	f, err := Service.ConfirmUpload(uid, req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, f)
}

// ListFiles 文件列表（folder_id / file_type / tag_id / keyword / page / size）。
func ListFiles(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	f := fileListFilter{
		FileType: c.Query("file_type"),
		Keyword:  c.Query("keyword"),
		Order:    c.Query("order"),
		Sort:     c.Query("sort"),
		Page:     page,
		Size:     size,
	}
	if v := c.Query("folder_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 64); err == nil {
			fid := uint(id)
			f.FolderID = &fid
			f.FolderIDSet = true
		}
	}
	if v := c.Query("tag_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 64); err == nil {
			f.TagID = uint(id)
		}
	}
	list, total, err := Service.ListFiles(uid, f)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// DownloadFile 返回 presigned GET（做超额锁定校验，§6）。
func DownloadFile(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, ok := pathID(c)
	if !ok {
		framework.Fail(c, http.StatusBadRequest, "非法文件 ID")
		return
	}
	url, err := Service.DownloadFile(uid, id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{"url": url})
}

// UpdateFile 重命名 / 移动文件夹。
func UpdateFile(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, ok := pathID(c)
	if !ok {
		framework.Fail(c, http.StatusBadRequest, "非法文件 ID")
		return
	}
	var req UpdateFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	f, err := Service.UpdateFile(uid, id, req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, f)
}

// DeleteFile 软删文件（cron 异步删 S3）。
func DeleteFile(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, ok := pathID(c)
	if !ok {
		framework.Fail(c, http.StatusBadRequest, "非法文件 ID")
		return
	}
	if err := Service.DeleteFile(uid, id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// GetFileTags 取文件标签 ID 列表。
func GetFileTags(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, ok := pathID(c)
	if !ok {
		framework.Fail(c, http.StatusBadRequest, "非法文件 ID")
		return
	}
	ids, err := Service.GetFileTags(uid, id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{"tag_ids": ids})
}

// SetFileTags 覆盖式给文件打标签。
func SetFileTags(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, ok := pathID(c)
	if !ok {
		framework.Fail(c, http.StatusBadRequest, "非法文件 ID")
		return
	}
	var req struct {
		TagIDs []uint `json:"tag_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	if err := Service.SetFileTags(uid, id, req.TagIDs); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}
