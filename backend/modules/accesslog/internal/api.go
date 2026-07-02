package accesslog

import (
	"net/http"
	"strconv"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// GetAccessLogList 获取访问日志列表（需超管）
// @Summary 获取访问日志列表
// @Description 分页查询接口访问日志，支持多条件筛选（需要超级管理员权限）
// @Tags 访问日志
// @Produce json
// @Security Bearer
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(20)
// @Param username query string false "用户名（模糊搜索）"
// @Param scope query string false "身份域（staff/user/anonymous）"
// @Param method query string false "HTTP方法（GET/POST/PUT/DELETE）"
// @Param path query string false "请求路径（模糊搜索）"
// @Param status_group query string false "状态码区间（2xx/4xx/5xx）"
// @Param start_time query string false "开始时间（ISO 8601）"
// @Param end_time query string false "结束时间（ISO 8601）"
// @Param order query string false "排序字段（id/created_at/status_code/latency_ms）"
// @Param sort query string false "排序方向（ascending/descending）"
// @Success 200 {object} framework.Response{data=framework.PageResponse}
// @Failure 401 {object} framework.Response
// @Failure 403 {object} framework.Response
// @Router /access-log/list [get]
func GetAccessLogList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	// 钳制分页参数，避免 size=-1 触发 Limit(-1) 拉全表、page=0 产生负 offset（S4）。
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}

	list, total, err := AccessLogService.GetList(ListQuery{
		Page:        page,
		Size:        size,
		Username:    c.Query("username"),
		Scope:       c.Query("scope"),
		Method:      c.Query("method"),
		Path:        c.Query("path"),
		StatusGroup: c.Query("status_group"),
		StartTime:   c.Query("start_time"),
		EndTime:     c.Query("end_time"),
		Order:       c.Query("order"),
		Sort:        c.Query("sort"),
	})
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// GetAccessLog 获取访问日志详情（需超管）
// @Summary 获取访问日志详情
// @Description 根据 ID 获取访问日志完整信息，包含请求/响应头和体（需要超级管理员权限）
// @Tags 访问日志
// @Produce json
// @Security Bearer
// @Param id path int true "日志ID"
// @Success 200 {object} framework.Response{data=AccessLog}
// @Failure 400 {object} framework.Response
// @Failure 401 {object} framework.Response
// @Failure 403 {object} framework.Response
// @Failure 404 {object} framework.Response
// @Router /access-log/{id} [get]
func GetAccessLog(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}

	item, err := AccessLogService.GetByID(id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}
