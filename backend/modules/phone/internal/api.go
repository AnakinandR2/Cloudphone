package phone

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

func parseID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return 0, false
	}
	return id, true
}

// --- 前台：我的云手机 ---

// GetCloudPhoneList 我的云手机列表（分页，仅本人）
// @Summary 我的云手机列表
// @Tags 我的云手机
// @Produce json
// @Security Bearer
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(10)
// @Param kw query string false "名称/cpId 模糊搜索"
// @Param status query string false "状态过滤"
// @Success 200 {object} framework.Response{data=framework.PageResponse}
// @Router /phone/list [get]
func GetCloudPhoneList(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
	list, total, err := PhoneService.GetList(uid, page, size, c.Query("kw"), c.Query("status"), c.Query("tag"), c.Query("order"), c.Query("sort"))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// GetCloudPhone 云手机详情（仅本人）
// @Summary 云手机详情
// @Tags 我的云手机
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Success 200 {object} framework.Response{data=CloudPhone}
// @Router /phone/{id} [get]
func GetCloudPhone(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	item, err := PhoneService.GetByID(uid, id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// CreateCloudPhone 新建云手机档案
// @Summary 新建云手机
// @Tags 我的云手机
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body CloudPhoneCreate true "数据"
// @Success 200 {object} framework.Response{data=CloudPhone}
// @Router /phone/create [post]
func CreateCloudPhone(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req CloudPhoneCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := PhoneService.Create(uid, &req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// UpdateCloudPhone 更新云手机（仅本人）
// @Summary 更新云手机
// @Tags 我的云手机
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Param body body CloudPhoneUpdate true "数据"
// @Success 200 {object} framework.Response{data=CloudPhone}
// @Router /phone/update/{id} [put]
func UpdateCloudPhone(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req CloudPhoneUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := PhoneService.Update(uid, id, &req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// DeleteCloudPhone 删除云手机（仅本人）
// @Summary 删除云手机
// @Tags 我的云手机
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Success 200 {object} framework.Response
// @Router /phone/delete/{id} [delete]
func DeleteCloudPhone(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := PhoneService.Delete(uid, id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// --- 管理侧：实例管理（全量）---

// AdminListCloudPhones 云手机实例列表（全量，管理侧）
// @Summary 云手机实例列表（管理侧）
// @Tags 实例管理
// @Produce json
// @Security Bearer
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(20)
// @Param kw query string false "名称/cpId 模糊搜索"
// @Param status query string false "状态过滤"
// @Param userId query int false "按属主用户过滤"
// @Success 200 {object} framework.Response{data=framework.PageResponse}
// @Router /admin/phones/list [get]
func AdminListCloudPhones(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	userID, _ := strconv.Atoi(c.Query("userId"))
	list, total, err := PhoneService.AdminList(page, size, c.Query("kw"), c.Query("status"), c.Query("tag"), userID, c.Query("order"), c.Query("sort"))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// AdminGetCloudPhone 云手机详情（管理侧）
// @Summary 云手机详情（管理侧）
// @Tags 实例管理
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Success 200 {object} framework.Response{data=CloudPhone}
// @Router /admin/phones/{id} [get]
func AdminGetCloudPhone(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	item, err := PhoneService.AdminGetByID(id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// AdminDeleteCloudPhone 强制删除云手机（管理侧，运维）
// @Summary 强制删除云手机（管理侧）
// @Tags 实例管理
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Success 200 {object} framework.Response
// @Router /admin/phones/delete/{id} [delete]
func AdminDeleteCloudPhone(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := PhoneService.AdminDelete(id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// AdminListPhoneTags 运营侧全用户标签去重列表（标签筛选下拉用）
// @Summary 全部标签
// @Tags 实例管理
// @Produce json
// @Security Bearer
// @Router /admin/phones/tags [get]
func AdminListPhoneTags(c *gin.Context) {
	tags, err := PhoneService.AdminListTags()
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, tags)
}
