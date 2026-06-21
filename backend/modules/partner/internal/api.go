package partner

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"manager-backend/framework"
	"manager-backend/framework/auth"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func parseID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return 0, false
	}
	return id, true
}

// optionalUserID 软取登录前台用户 ID：公开接口可匿名访问，带有效 user 令牌时识别出 user_id，
// 否则返回 0（匿名）。仅做归因，不做版本校验（best-effort）。
func optionalUserID(c *gin.Context) int {
	token := strings.TrimSpace(c.GetHeader("Authorization"))
	if after, ok := strings.CutPrefix(token, "Bearer "); ok {
		token = strings.TrimSpace(after)
	}
	if token == "" {
		if name := framework.AppConfig.UserJWTCookieName; name != "" {
			token, _ = c.Cookie(name)
		}
	}
	if token == "" {
		return 0
	}
	secret := framework.AppConfig.UserJWTSecret
	if secret == "" {
		secret = framework.AppConfig.JWTSecret
	}
	claims, err := auth.Parse(token, secret)
	if err != nil || claims.Scope != auth.ScopeUser {
		return 0
	}
	return claims.UserID
}

// --- admin：合作商管理 ---

// AdminListPartners 合作商列表（全量，含禁用）
// @Summary 合作商列表（管理侧）
// @Tags 合作商管理
// @Produce json
// @Security Bearer
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(20)
// @Param kw query string false "名称/介绍 模糊搜索"
// @Success 200 {object} framework.Response{data=framework.PageResponse}
// @Router /admin/partners [get]
func AdminListPartners(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, total, err := PartnerService.AdminList(page, size, c.Query("kw"), c.Query("order"), c.Query("sort"))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// CreatePartner 新增合作商
// @Summary 新增合作商
// @Tags 合作商管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body PartnerCreate true "数据"
// @Success 200 {object} framework.Response{data=Partner}
// @Router /admin/partners [post]
func CreatePartner(c *gin.Context) {
	var req PartnerCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := PartnerService.Create(&req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// UpdatePartner 更新合作商
// @Summary 更新合作商
// @Tags 合作商管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Param body body PartnerUpdate true "数据"
// @Success 200 {object} framework.Response{data=Partner}
// @Router /admin/partners/{id} [put]
func UpdatePartner(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req PartnerUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := PartnerService.Update(id, &req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// DeletePartner 删除合作商
// @Summary 删除合作商
// @Tags 合作商管理
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Success 200 {object} framework.Response
// @Router /admin/partners/{id} [delete]
func DeletePartner(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := PartnerService.Delete(id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// allowedImageExt 允许上传的图片扩展名。
var allowedImageExt = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true, ".svg": true}

const maxImageSize = 5 << 20 // 5MB

// UploadPartnerImage 上传合作商图片（logo/配图）到 S3，返回公开 URL
// @Summary 上传合作商图片
// @Tags 合作商管理
// @Accept multipart/form-data
// @Produce json
// @Security Bearer
// @Param file formData file true "图片文件"
// @Success 200 {object} framework.Response{data=object}
// @Router /admin/partners/upload [post]
func UploadPartnerImage(c *gin.Context) {
	if framework.S3 == nil {
		framework.Fail(c, http.StatusServiceUnavailable, "对象存储未配置，无法上传图片")
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "请选择图片文件")
		return
	}
	if fh.Size > maxImageSize {
		framework.Fail(c, http.StatusBadRequest, "图片不能超过 5MB")
		return
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if !allowedImageExt[ext] {
		framework.Fail(c, http.StatusBadRequest, "仅支持 png/jpg/jpeg/webp/gif/svg 图片")
		return
	}
	f, err := fh.Open()
	if err != nil {
		framework.Fail(c, http.StatusInternalServerError, "读取文件失败")
		return
	}
	defer f.Close()

	contentType := fh.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	key := fmt.Sprintf("partners/%s%s", uuid.NewString(), ext)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := framework.S3.PutObject(ctx, key, f, contentType); err != nil {
		framework.Fail(c, http.StatusInternalServerError, "上传失败："+err.Error())
		return
	}
	framework.OKWithData(c, gin.H{"url": framework.S3.PublicURL(key)})
}

// AdminListClicks 点击明细分页（运营查看趋势/明细）
// @Summary 合作商点击明细
// @Tags 合作商管理
// @Produce json
// @Security Bearer
// @Param id path int true "合作商ID"
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(20)
// @Success 200 {object} framework.Response{data=framework.PageResponse}
// @Router /admin/partners/{id}/clicks [get]
func AdminListClicks(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, total, err := PartnerService.ListClicks(id, page, size)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// --- 公开：my / www 展示与点击上报 ---

// ListPartners 启用合作商列表（公开，按 sort 排序）
// @Summary 代理IP推荐列表（公开）
// @Tags 代理IP推荐
// @Produce json
// @Success 200 {object} framework.Response{data=[]PublicPartner}
// @Router /partner/list [get]
func ListPartners(c *gin.Context) {
	items, err := PartnerService.ListEnabled()
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, items)
}

// ClickPartner 上报一次推广链接点击（公开，支持匿名）
// @Summary 上报推广链接点击
// @Tags 代理IP推荐
// @Accept json
// @Produce json
// @Param id path int true "合作商ID"
// @Param body body ClickRequest false "软标识"
// @Success 200 {object} framework.Response
// @Router /partner/{id}/click [post]
func ClickPartner(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req ClickRequest
	_ = c.ShouldBindJSON(&req) // 软标识可缺省；解析失败按空处理，不阻断

	click := &PartnerClick{
		UserID:      uint(optionalUserID(c)),
		IP:          c.ClientIP(),
		UserAgent:   c.GetHeader("User-Agent"),
		Referer:     c.GetHeader("Referer"),
		AnonymousID: req.AnonymousID,
		SessionID:   req.SessionID,
		UTMSource:   req.UTMSource,
		UTMMedium:   req.UTMMedium,
		UTMCampaign: req.UTMCampaign,
		UTMTerm:     req.UTMTerm,
		UTMContent:  req.UTMContent,
		Channel:     req.Channel,
		Extra:       req.Extra,
	}
	if _, err := PartnerService.RecordClick(id, click); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c) // 即便合作商不存在/禁用也返回成功，前端据此放心跳转
}
