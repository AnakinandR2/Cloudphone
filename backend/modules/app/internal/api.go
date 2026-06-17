package app

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"manager-backend/framework"
	"manager-backend/framework/midplat"

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

// GetAppList 我的应用列表（本地绑定 + 中台状态刷新）
// @Summary 我的应用列表
// @Tags 应用管理
// @Produce json
// @Security Bearer
// @Router /app/list [get]
func GetAppList(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	list, err := AppService.List(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

// GetMarket 应用市场（应用商店：由 admin 上传、面向全部用户，区别于 /app/list 仅本人上传的）
// @Summary 应用市场
// @Tags 应用管理
// @Produce json
// @Security Bearer
// @Router /app/market [get]
func GetMarket(c *gin.Context) {
	if _, ok := currentUserID(c); !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	list, err := AppService.StoreList()
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

// UploadApp 上传 APK 到应用库（multipart：file 必填；appName / appDesc 可选）
// @Summary 上传应用
// @Tags 应用管理
// @Accept multipart/form-data
// @Produce json
// @Security Bearer
// @Param file formData file true "APK / XAPK 文件"
// @Param appName formData string false "应用名，留空则用中台解析出的真名"
// @Param appDesc formData string false "应用描述"
// @Router /app/upload [post]
func UploadApp(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "未选择文件")
		return
	}
	tmp, err := os.CreateTemp("", "appupload-*"+filepath.Ext(fh.Filename))
	if err != nil {
		framework.Fail(c, http.StatusInternalServerError, "创建临时文件失败")
		return
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath)
	if err := c.SaveUploadedFile(fh, tmpPath); err != nil {
		framework.Fail(c, http.StatusInternalServerError, "保存上传文件失败")
		return
	}
	// appName 留空 → 由中台解析出的应用真名决定（见 framework UploadAppFromFile）。
	rec, err := AppService.Upload(uid, tmpPath, midplat.UploadAppOptions{
		AppName: c.PostForm("appName"),
		AppDesc: c.PostForm("appDesc"),
	})
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, rec)
}

// ── 浏览器驱动的分片上传流水线（前台 my 用） ─────────────────────────────────
// 浏览器切片并逐片驱动：initiate → part → complete → parse →（用户确认）→ create →（轮询）status。
// uploadId 一律以字符串透传，避免 JS number 精度问题；后端转 int64 调中台。

// parseUploadID 把字符串 uploadId 解析成 int64；空串/非法 → 0（秒传命中时为 0，合法）。
func parseUploadID(s string) int64 {
	id, _ := strconv.ParseInt(s, 10, 64)
	return id
}

// uploadAppInfoDTO 是秒传命中 / 解析返回的应用元信息（前端确认面板用）。
type uploadAppInfoDTO struct {
	AppName     string `json:"appName"`
	PackageName string `json:"packageName"`
	Version     string `json:"version"`
	IconPath    string `json:"iconPath"`
	FileSize    string `json:"fileSize"`
	MD5         string `json:"md5"`
}

// initiateRespDTO 是 /app/upload/initiate 的响应（uploadId 转字符串）。
type initiateRespDTO struct {
	UploadID      string            `json:"uploadId"`
	PartSize      int64             `json:"partSize"`
	TotalParts    int               `json:"totalParts"`
	UploadSuccess bool              `json:"uploadSuccess"`
	AppInfo       *uploadAppInfoDTO `json:"appInfo,omitempty"`
}

// InitiateUpload 启动分片上传会话（§1.3）。命中秒传时返回 uploadSuccess=true + appInfo。
// @Summary 启动分片上传
// @Tags 应用管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body object true "{fileName, fileSize, contentMd5}"
// @Router /app/upload/initiate [post]
func InitiateUpload(c *gin.Context) {
	if _, ok := currentUserID(c); !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req struct {
		FileName   string `json:"fileName"`
		FileSize   int64  `json:"fileSize"`
		ContentMD5 string `json:"contentMd5"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.FileName == "" {
		framework.Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	resp, err := AppService.InitiateUpload(midplat.InitiateUploadRequest{
		FileName: req.FileName, FileSize: req.FileSize, ContentMD5: req.ContentMD5,
	})
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	out := initiateRespDTO{
		PartSize:      resp.PartSize,
		TotalParts:    resp.TotalParts,
		UploadSuccess: resp.UploadSuccess,
	}
	if resp.UploadID != nil {
		out.UploadID = strconv.FormatInt(*resp.UploadID, 10)
	}
	if resp.UploadSuccess && resp.AppInfo != nil {
		ai := resp.AppInfo
		out.AppInfo = &uploadAppInfoDTO{
			AppName:     ai.AppName,
			PackageName: ai.PackageName,
			Version:     ai.Version,
			IconPath:    ai.IconPath,
			FileSize:    ai.FileSize,
			MD5:         ai.AppMD5,
		}
	}
	framework.OKWithData(c, out)
}

// UploadPart 上传单个分片（§1.4）。uploadId/partNumber/contentMd5 走 query，二进制走 form 字段 file。
// @Summary 上传分片
// @Tags 应用管理
// @Accept multipart/form-data
// @Produce json
// @Security Bearer
// @Router /app/upload/part [post]
func UploadPart(c *gin.Context) {
	if _, ok := currentUserID(c); !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	uploadID := parseUploadID(c.Query("uploadId"))
	partNumber, _ := strconv.Atoi(c.Query("partNumber"))
	if uploadID == 0 || partNumber <= 0 {
		framework.Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "未收到分片数据")
		return
	}
	f, err := fh.Open()
	if err != nil {
		framework.Fail(c, http.StatusInternalServerError, "读取分片失败")
		return
	}
	defer f.Close()
	resp, err := AppService.UploadPart(uploadID, partNumber, c.Query("contentMd5"), f, fh.Filename)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, resp)
}

// CompleteUpload 完成分片合并（§1.5）。
// @Summary 完成分片合并
// @Tags 应用管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body object true "{uploadId}"
// @Router /app/upload/complete [post]
func CompleteUpload(c *gin.Context) {
	if _, ok := currentUserID(c); !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req struct {
		UploadID string `json:"uploadId"`
	}
	_ = c.ShouldBindJSON(&req)
	uploadID := parseUploadID(req.UploadID)
	if uploadID == 0 {
		framework.Fail(c, http.StatusBadRequest, "缺少 uploadId")
		return
	}
	url, err := AppService.CompleteUpload(uploadID)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{"downloadUrl": url})
}

// ParseApp 解析已上传 APK 元信息（§1.6）。
// @Summary 解析已上传应用
// @Tags 应用管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body object true "{uploadId}"
// @Router /app/upload/parse [post]
func ParseApp(c *gin.Context) {
	if _, ok := currentUserID(c); !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req struct {
		UploadID string `json:"uploadId"`
	}
	_ = c.ShouldBindJSON(&req)
	uploadID := parseUploadID(req.UploadID)
	if uploadID == 0 {
		framework.Fail(c, http.StatusBadRequest, "缺少 uploadId")
		return
	}
	parsed, err := AppService.ParseUpload(uploadID)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{
		"uploadId":    req.UploadID,
		"appName":     parsed.AppName,
		"packageName": parsed.PackageName,
		"version":     parsed.Version,
		"iconPath":    parsed.IconPath,
		"fileSize":    parsed.FileSize,
		"md5":         parsed.MD5,
	})
}

// CreateAppFromUpload 由已上传文件创建应用并落本地绑定（§1.8）。秒传时 uploadId 为空。
// @Summary 创建已上传应用
// @Tags 应用管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body object true "{uploadId, appName, packageName, version, iconPath, fileSize, md5, appDesc}"
// @Router /app/upload/create [post]
func CreateAppFromUpload(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req struct {
		UploadID    string `json:"uploadId"`
		AppName     string `json:"appName"`
		PackageName string `json:"packageName"`
		Version     string `json:"version"`
		IconPath    string `json:"iconPath"`
		FileSize    string `json:"fileSize"`
		MD5         string `json:"md5"`
		AppDesc     string `json:"appDesc"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	rec, err := AppService.CreateFromUpload(uid, midplat.CreateFromUploadedFileRequest{
		UploadID:    parseUploadID(req.UploadID),
		AppName:     req.AppName,
		PackageName: req.PackageName,
		Version:     req.Version,
		IconPath:    req.IconPath,
		FileSize:    req.FileSize,
		MD5:         req.MD5,
		AppDesc:     req.AppDesc,
	})
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, rec)
}

// UploadStatus 查询上传任务状态（§1.9）。
// @Summary 查询上传状态
// @Tags 应用管理
// @Produce json
// @Security Bearer
// @Param uploadId query string true "上传会话 ID"
// @Router /app/upload/status [get]
func UploadStatus(c *gin.Context) {
	if _, ok := currentUserID(c); !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	uploadID := parseUploadID(c.Query("uploadId"))
	if uploadID == 0 {
		framework.Fail(c, http.StatusBadRequest, "缺少 uploadId")
		return
	}
	status, err := AppService.QueryUploadStatus(uploadID)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{"status": status})
}

// BatchDeleteApps 批量删除（仅本人上传的应用，按本地绑定 id）
// @Summary 批量删除应用
// @Tags 应用管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body object true "{ids: number[]} 本地绑定 id"
// @Router /app/batch-delete [post]
func BatchDeleteApps(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req struct {
		IDs []int `json:"ids"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := AppService.BatchDelete(uid, req.IDs); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// AdminListApps 运营：列出全部用户上传的应用（跨用户，含所属用户信息）
// @Summary 运营查看用户应用
// @Tags 应用管理(运营)
// @Produce json
// @Security Bearer
// @Router /admin/apps [get]
func AdminListApps(c *gin.Context) {
	list, err := AppService.AdminList()
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

// AdminBatchDeleteApps 运营：删除任意用户的应用（按本地绑定 id）
// @Summary 运营删除用户应用
// @Tags 应用管理(运营)
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body object true "{ids: number[]} 本地绑定 id"
// @Router /admin/apps/batch-delete [post]
func AdminBatchDeleteApps(c *gin.Context) {
	var req struct {
		IDs []int `json:"ids"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := AppService.AdminBatchDelete(req.IDs); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// AdminStoreList 应用商店：列出 admin 上传、面向全部用户的应用
// @Summary 应用商店列表
// @Tags 应用商店(运营)
// @Produce json
// @Security Bearer
// @Router /admin/apps/store [get]
func AdminStoreList(c *gin.Context) {
	list, err := AppService.StoreList()
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

// AdminStoreUpload 应用商店：上传 APK（multipart：file 必填；appName / appDesc 可选）
// @Summary 上传应用商店应用
// @Tags 应用商店(运营)
// @Accept multipart/form-data
// @Produce json
// @Security Bearer
// @Param file formData file true "APK / XAPK 文件"
// @Param appName formData string false "应用名，留空则用中台解析出的真名"
// @Param appDesc formData string false "应用描述"
// @Router /admin/apps/store/upload [post]
func AdminStoreUpload(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "未选择文件")
		return
	}
	tmp, err := os.CreateTemp("", "storeupload-*"+filepath.Ext(fh.Filename))
	if err != nil {
		framework.Fail(c, http.StatusInternalServerError, "创建临时文件失败")
		return
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath)
	if err := c.SaveUploadedFile(fh, tmpPath); err != nil {
		framework.Fail(c, http.StatusInternalServerError, "保存上传文件失败")
		return
	}
	rec, err := AppService.StoreUpload(tmpPath, midplat.UploadAppOptions{
		AppName: c.PostForm("appName"),
		AppDesc: c.PostForm("appDesc"),
	})
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, rec)
}

// AdminStoreDelete 应用商店：批量删除（按本地绑定 id，仅限商店应用）
// @Summary 删除应用商店应用
// @Tags 应用商店(运营)
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body object true "{ids: number[]} 本地绑定 id"
// @Router /admin/apps/store/batch-delete [post]
func AdminStoreDelete(c *gin.Context) {
	var req struct {
		IDs []int `json:"ids"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := AppService.StoreDelete(req.IDs); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}
