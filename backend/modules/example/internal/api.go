package example

import (
	"net/http"
	"strconv"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// GetExampleList 获取示例列表（分页，需后台登录 + example:view）
// @Summary 获取示例列表
// @Description 分页获取示例数据列表（仅后台，需 example:view 权限）
// @Tags 示例
// @Produce json
// @Security Bearer
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(10)
// @Param title query string false "标题（模糊搜索）"
// @Param order query string false "排序字段"
// @Param sort query string false "排序方式（ascending/descending）"
// @Success 200 {object} framework.Response{data=framework.PageResponse}
// @Failure 401 {object} framework.Response
// @Failure 403 {object} framework.Response
// @Router /example/list [get]
func GetExampleList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
	title := c.Query("title")
	order := c.Query("order")
	sort := c.Query("sort")

	list, total, err := ExampleService.GetList(page, size, title, order, sort)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// GetExample 获取单个示例（需后台登录 + example:view）
// @Summary 获取示例详情
// @Description 根据 ID 获取示例数据（仅后台，需 example:view 权限）
// @Tags 示例
// @Produce json
// @Security Bearer
// @Param id path int true "示例ID"
// @Success 200 {object} framework.Response{data=ExampleItem}
// @Failure 401 {object} framework.Response
// @Failure 403 {object} framework.Response
// @Failure 404 {object} framework.Response
// @Router /example/{id} [get]
func GetExample(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}

	item, err := ExampleService.GetByID(id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// CreateExample 创建示例（需要登录）
// @Summary 创建示例
// @Description 创建新的示例数据（需要登录）
// @Tags 示例
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body ExampleItemCreate true "示例数据"
// @Success 200 {object} framework.Response{data=ExampleItem}
// @Failure 400 {object} framework.Response
// @Router /example/create [post]
func CreateExample(c *gin.Context) {
	var req ExampleItemCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	item, err := ExampleService.Create(&req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// UpdateExample 更新示例（需要登录）
// @Summary 更新示例
// @Description 更新示例数据（需要登录）
// @Tags 示例
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "示例ID"
// @Param body body ExampleItemUpdate true "更新数据"
// @Success 200 {object} framework.Response{data=ExampleItem}
// @Failure 400 {object} framework.Response
// @Router /example/update/{id} [put]
func UpdateExample(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}

	var req ExampleItemUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	item, err := ExampleService.Update(id, &req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// DeleteExample 删除示例（需要登录）
// @Summary 删除示例
// @Description 删除示例数据（需要登录）
// @Tags 示例
// @Produce json
// @Security Bearer
// @Param id path int true "示例ID"
// @Success 200 {object} framework.Response
// @Failure 400 {object} framework.Response
// @Router /example/delete/{id} [delete]
func DeleteExample(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}

	if err := ExampleService.Delete(id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}
