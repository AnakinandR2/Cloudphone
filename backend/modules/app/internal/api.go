package app

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

// atoiDefault 解析整数，失败回退默认值。
func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// ── 用户应用（user auth） ────────────────────────────────────────────────

// UserList 当前用户应用列表（library app 文件 LEFT JOIN app_user_meta）
// @Summary 我的应用列表
// @Tags 应用管理
// @Produce json
// @Security Bearer
// @Router /app/user [get]
func UserList(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	page := atoiDefault(c.Query("page"), 1)
	size := atoiDefault(c.Query("size"), 50)
	list, err := AppService.ListUserApps(uid, page, size)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

// UserFinalize 回读对象解析 → 写 app_user_meta，返回该应用 DTO
// @Summary 解析并落定我的应用
// @Tags 应用管理
// @Produce json
// @Security Bearer
// @Param fileId path int true "素材库文件 ID"
// @Router /app/user/{fileId}/finalize [post]
func UserFinalize(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	fid, err := strconv.ParseUint(c.Param("fileId"), 10, 64)
	if err != nil || fid == 0 {
		framework.Fail(c, http.StatusBadRequest, "无效文件 ID")
		return
	}
	dto, err := AppService.FinalizeUserApp(uid, uint(fid))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, dto)
}

// UserBatchDelete 删除我的应用（素材库删文件释放配额 + 删 meta）
// @Summary 批量删除我的应用
// @Tags 应用管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body object true "{file_ids: number[]}"
// @Router /app/user/batch-delete [post]
func UserBatchDelete(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req struct {
		FileIDs []uint `json:"file_ids"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := AppService.BatchDeleteUserApps(uid, req.FileIDs); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// MarketList 浏览应用市场（仅 parse_status=ready）
// @Summary 应用市场
// @Tags 应用管理
// @Produce json
// @Security Bearer
// @Router /app/market [get]
func MarketList(c *gin.Context) {
	if _, ok := currentUserID(c); !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	list, err := AppService.ListMarket(true)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

// ── 运营治理（staff auth + 权限） ─────────────────────────────────────────

// AdminUserAppList 跨用户：用户应用文件 + 上传者 + meta
// @Summary 运营查看用户应用
// @Tags 应用管理(运营)
// @Produce json
// @Security Bearer
// @Router /admin/apps [get]
func AdminUserAppList(c *gin.Context) {
	list, err := AppService.AdminListUserApps()
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

// AdminUserAppBatchDelete 跨用户删除（按 meta.user_id 定属主后 library 删）
// @Summary 运营删除用户应用
// @Tags 应用管理(运营)
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body object true "{file_ids: number[]}"
// @Router /admin/apps/batch-delete [post]
func AdminUserAppBatchDelete(c *gin.Context) {
	var req struct {
		FileIDs []uint `json:"file_ids"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := AppService.AdminBatchDeleteUserApps(req.FileIDs); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// AdminMarketList 市场列表（admin）
// @Summary 应用市场列表(运营)
// @Tags 应用市场(运营)
// @Produce json
// @Security Bearer
// @Router /admin/apps/market [get]
func AdminMarketList(c *gin.Context) {
	list, err := AppService.ListMarket(false)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

// AdminMarketUpload 上传市场应用（multipart 'file'）
// @Summary 上传应用市场应用
// @Tags 应用市场(运营)
// @Accept multipart/form-data
// @Produce json
// @Security Bearer
// @Param file formData file true "APK / XAPK 文件"
// @Router /admin/apps/market/upload [post]
func AdminMarketUpload(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "未选择文件")
		return
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(fh.Filename), "."))
	tmp, err := os.CreateTemp("", "marketupload-*."+ext)
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
	dto, err := AppService.MarketUpload(tmpPath, ext)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, dto)
}

// AdminMarketBatchDelete 删除市场应用（删 S3 对象 + 行）
// @Summary 删除应用市场应用
// @Tags 应用市场(运营)
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body object true "{ids: number[]}"
// @Router /admin/apps/market/batch-delete [post]
func AdminMarketBatchDelete(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := AppService.MarketBatchDelete(req.IDs); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}
