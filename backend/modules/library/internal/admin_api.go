package library

import (
	"net/http"
	"strconv"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// pathUserID 解析路由 :userId 为 int（>0）。
func pathUserID(c *gin.Context) (int, bool) {
	v, err := strconv.Atoi(c.Param("userId"))
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}

// AdminGetPricing 读取素材库定价配置（staff library:view）。
func AdminGetPricing(c *gin.Context) {
	cfg, err := PricingConfigService.Get()
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, cfg)
}

// AdminSavePricing 覆盖保存素材库定价配置（staff library:manage）。
func AdminSavePricing(c *gin.Context) {
	var data LibraryPricingConfigData
	if err := c.ShouldBindJSON(&data); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	if err := PricingConfigService.Save(&data); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, &data)
}

// AdminGetUser 查某用户用量 + 订阅（staff library:view，§8 gap-7）。
func AdminGetUser(c *gin.Context) {
	uid, ok := pathUserID(c)
	if !ok {
		framework.Fail(c, http.StatusBadRequest, "非法用户 ID")
		return
	}
	view, err := Service.AdminGetUser(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, view)
}

// AdminGrantUser 后台赠送/调整某用户套餐（staff library:manage，§8 gap-7）。
func AdminGrantUser(c *gin.Context) {
	uid, ok := pathUserID(c)
	if !ok {
		framework.Fail(c, http.StatusBadRequest, "非法用户 ID")
		return
	}
	var req AdminGrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	view, err := Service.AdminGrant(uid, req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, view)
}
