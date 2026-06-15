package billing

import (
	"net/http"
	"strconv"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// GetMyRuntimeUsage 前台：当前用户的时长费用量切片（分页，倒序）。
func GetMyRuntimeUsage(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, total, err := RuntimeService.ListSlices(uid, page, size)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// AdminGetRuntimeConfig 后台：读时长费配置。
func AdminGetRuntimeConfig(c *gin.Context) {
	cfg, err := RuntimeService.GetConfig()
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, cfg)
}

type runtimeConfigBody struct {
	UnitPriceCentsPerMinute int64 `json:"unit_price_cents_per_minute"`
	LowBalanceAlertCents    int64 `json:"low_balance_alert_cents"`
}

// AdminSaveRuntimeConfig 后台：保存时长费单价 + 低余额阈值。
func AdminSaveRuntimeConfig(c *gin.Context) {
	var req runtimeConfigBody
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	if req.UnitPriceCentsPerMinute < 0 || req.LowBalanceAlertCents < 0 {
		framework.Fail(c, http.StatusBadRequest, "数值不能为负")
		return
	}
	cfg, err := RuntimeService.SaveConfig(req.UnitPriceCentsPerMinute, req.LowBalanceAlertCents)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, cfg)
}
