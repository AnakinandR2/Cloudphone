package proxy

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

// --- 前台：我的代理 ---

// GetProxyList 我的代理列表（分页，仅本人）
// @Summary 我的 SOCKS5 代理列表
// @Tags SOCKS5代理
// @Produce json
// @Security Bearer
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(10)
// @Param kw query string false "名称/host 模糊搜索"
// @Success 200 {object} framework.Response{data=framework.PageResponse}
// @Router /proxy/list [get]
func GetProxyList(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
	list, total, err := ProxyService.GetList(uid, page, size, c.Query("kw"), c.Query("order"), c.Query("sort"))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// GetProxyOptions 我的可用代理（不分页，供「绑定代理」下拉）
// @Summary 我的可用代理（不分页，下拉用）
// @Tags SOCKS5代理
// @Produce json
// @Security Bearer
// @Success 200 {object} framework.Response{data=[]Proxy}
// @Router /proxy/options [get]
func GetProxyOptions(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	items, err := ProxyService.ListOptions(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, items)
}

// TestProxy 测试代理（经 SOCKS5 实测连通性/延迟/出口IP + 自动识别归属）
// @Summary 测试代理
// @Tags SOCKS5代理
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Success 200 {object} framework.Response{data=Proxy}
// @Router /proxy/{id}/test [post]
func TestProxy(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	p, err := ProxyService.TestProxy(uid, id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, p)
}

// GetProxy 代理详情（仅本人）
// @Summary 代理详情
// @Tags SOCKS5代理
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Success 200 {object} framework.Response{data=Proxy}
// @Router /proxy/{id} [get]
func GetProxy(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	item, err := ProxyService.GetByID(uid, id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// CreateProxy 新增代理
// @Summary 新增代理
// @Tags SOCKS5代理
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body ProxyCreate true "数据"
// @Success 200 {object} framework.Response{data=Proxy}
// @Router /proxy/create [post]
func CreateProxy(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req ProxyCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := ProxyService.Create(uid, &req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// ProbeProxy 即时探测代理（按参数测试，不落库）
// @Summary 即时探测代理（表单测试用）
// @Tags SOCKS5代理
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body ProxyProbeRequest true "代理参数"
// @Success 200 {object} framework.Response{data=ProbeOutcome}
// @Router /proxy/probe [post]
func ProbeProxy(c *gin.Context) {
	if _, ok := currentUserID(c); !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req ProxyProbeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请填写主机与端口")
		return
	}
	out, err := ProxyService.Probe(&req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, out)
}

// BatchImportProxies 批量导入代理
// @Summary 批量导入代理
// @Tags SOCKS5代理
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body ProxyBatch true "代理条目数组"
// @Success 200 {object} framework.Response
// @Router /proxy/batch [post]
func BatchImportProxies(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req ProxyBatch
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	if len(req.Proxies) == 0 {
		framework.Fail(c, http.StatusBadRequest, "没有可导入的代理")
		return
	}
	created, err := ProxyService.BatchCreate(uid, req.Proxies)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{"created": created, "total": len(req.Proxies)})
}

// UpdateProxy 更新代理（仅本人）
// @Summary 更新代理
// @Tags SOCKS5代理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Param body body ProxyUpdate true "数据"
// @Success 200 {object} framework.Response{data=Proxy}
// @Router /proxy/update/{id} [put]
func UpdateProxy(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req ProxyUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := ProxyService.Update(uid, id, &req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// DeleteProxy 删除代理（仅本人）
// @Summary 删除代理
// @Tags SOCKS5代理
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Success 200 {object} framework.Response
// @Router /proxy/delete/{id} [delete]
func DeleteProxy(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := ProxyService.Delete(uid, id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// --- 管理侧：代理池（全量）---

// AdminListProxies 代理池列表（全量，管理侧）
// @Summary 代理池列表（管理侧）
// @Tags 代理池管理
// @Produce json
// @Security Bearer
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(20)
// @Param kw query string false "名称/host 模糊搜索"
// @Param status query string false "健康状态过滤（unknown/ok/fail）"
// @Success 200 {object} framework.Response{data=framework.PageResponse}
// @Router /admin/proxies/list [get]
func AdminListProxies(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, total, err := ProxyService.AdminList(page, size, c.Query("kw"), c.Query("status"), c.Query("order"), c.Query("sort"))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// AdminGetProxy 代理详情（管理侧）
// @Summary 代理详情（管理侧）
// @Tags 代理池管理
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Success 200 {object} framework.Response{data=Proxy}
// @Router /admin/proxies/{id} [get]
func AdminGetProxy(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	item, err := ProxyService.AdminGetByID(id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// AdminDeleteProxy 删除代理（管理侧，运维）
// @Summary 删除代理（管理侧）
// @Tags 代理池管理
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Success 200 {object} framework.Response
// @Router /admin/proxies/delete/{id} [delete]
func AdminDeleteProxy(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := ProxyService.AdminDelete(id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}
