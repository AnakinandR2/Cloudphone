package app

import (
	"net/http"
	"os"
	"path/filepath"

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
