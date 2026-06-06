package user

import (
	"net/http"
	"strconv"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// AdminListUsers 管理侧：前台用户列表
// @Summary 前台用户列表（管理侧）
// @Tags 前台用户管理
// @Produce json
// @Security Bearer
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(20)
// @Param phone query string false "手机号（模糊搜索）"
// @Param is_active query bool false "启用状态过滤"
// @Success 200 {object} framework.Response{data=framework.PageResponse}
// @Router /admin/users/list [get]
func AdminListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	var active *bool
	if v := c.Query("is_active"); v != "" {
		b := v == "true" || v == "1"
		active = &b
	}

	list, total, err := Service.AdminList(page, size, c.Query("phone"), active)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// AdminGetUser 管理侧：前台用户详情
// @Summary 前台用户详情（管理侧）
// @Tags 前台用户管理
// @Produce json
// @Security Bearer
// @Param id path int true "用户ID"
// @Success 200 {object} framework.Response{data=User}
// @Failure 404 {object} framework.Response
// @Router /admin/users/{id} [get]
func AdminGetUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	cust, err := Service.GetByID(id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, cust)
}

// AdminSetUserStatus 管理侧：启用/禁用前台用户
// @Summary 启用/禁用前台用户（管理侧）
// @Description 禁用会同时令该用户已签发令牌立即失效（强制下线）。
// @Tags 前台用户管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "用户ID"
// @Param body body AdminStatusRequest true "启用状态"
// @Success 200 {object} framework.Response{data=User}
// @Router /admin/users/{id}/status [put]
func AdminSetUserStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	var req AdminStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	cust, err := Service.SetActive(id, *req.IsActive)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, cust)
}
