package billing

import (
	"net/http"
	"strconv"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// currentUserID 取当前登录用户 ID（前台 user / 后台 staff 中间件均写入 "userID"）。
func currentUserID(c *gin.Context) (int, bool) {
	v, ok := c.Get("userID")
	if !ok {
		return 0, false
	}
	id, ok := v.(int)
	return id, ok
}

// GetMyAccount 我的计费账户（余额等）
func GetMyAccount(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	acc, err := BillingService.GetAccount(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, acc)
}

// GetMyLedger 我的费用日志（流水，分页 + subject/type 筛选）
func GetMyLedger(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, total, err := BillingService.ListLedger(uid, page, size, c.Query("subject"), c.Query("type"))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// Topup 充值（计划1 为桩：直接入账；计划4 改由支付网关回调驱动）
func Topup(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req TopupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	acc, err := BillingService.Topup(uid, req.AmountCents, "充值", "user:"+strconv.Itoa(uid))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, acc)
}
