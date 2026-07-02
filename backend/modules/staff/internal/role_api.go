package staff

import (
	"net/http"
	"strconv"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// GetRoleList 获取角色列表
// @Summary 获取角色列表
// @Tags 角色管理
// @Produce json
// @Security Bearer
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(20)
// @Param name query string false "角色名称模糊搜索"
// @Param order query string false "排序字段（id/name/created_at）"
// @Param sort query string false "排序方向（ascending/descending）"
// @Success 200 {object} framework.Response{data=framework.PageResponse}
// @Router /role/list [get]
func GetRoleList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	name := c.Query("name")
	order := c.Query("order")
	sort := c.Query("sort")

	list, total, err := RoleService.GetList(page, size, name, order, sort)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// GetRole 获取角色详情
// @Summary 获取角色详情
// @Tags 角色管理
// @Produce json
// @Security Bearer
// @Param id path int true "角色ID"
// @Success 200 {object} framework.Response{data=Role}
// @Router /role/{id} [get]
func GetRole(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的角色ID")
		return
	}

	role, err := RoleService.GetByID(id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, role)
}

// CreateRole 创建角色
// @Summary 创建角色
// @Tags 角色管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body RoleCreate true "角色信息"
// @Success 200 {object} framework.Response{data=Role}
// @Router /role/create [post]
func CreateRole(c *gin.Context) {
	var req RoleCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	role, err := RoleService.CreateChecked(&req, c.GetInt("userID"), c.GetBool("is_superuser"))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, role)
}

// UpdateRole 更新角色
// @Summary 更新角色
// @Tags 角色管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "角色ID"
// @Param body body RoleUpdate true "角色信息"
// @Success 200 {object} framework.Response{data=Role}
// @Router /role/update/{id} [put]
func UpdateRole(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的角色ID")
		return
	}

	var req RoleUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	role, err := RoleService.UpdateChecked(id, &req, c.GetInt("userID"), c.GetBool("is_superuser"))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, role)
}

// DeleteRole 删除角色
// @Summary 删除角色
// @Tags 角色管理
// @Produce json
// @Security Bearer
// @Param id path int true "角色ID"
// @Success 200 {object} framework.Response
// @Router /role/delete/{id} [delete]
func DeleteRole(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的角色ID")
		return
	}

	if err := RoleService.Delete(id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// GetPermissions 获取系统所有预定义权限
// @Summary 获取系统权限列表
// @Tags 角色管理
// @Produce json
// @Security Bearer
// @Success 200 {object} framework.Response{data=[]PermissionGroup}
// @Router /role/permissions [get]
func GetPermissions(c *gin.Context) {
	framework.OKWithData(c, PermissionGroups)
}
