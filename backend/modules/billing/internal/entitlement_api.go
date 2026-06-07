package billing

import (
	"net/http"
	"strconv"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// GetMyEntitlements 前台：我的资源容量 + 批次明细
func GetMyEntitlements(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	caps, err := EntitlementService.Capacities(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	batches, err := EntitlementService.ListBatches(uid, "")
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{"capacities": caps, "batches": batches})
}

// AdjustResourceRequest 后台资源调整请求
type AdjustResourceRequest struct {
	Subject string `json:"subject" binding:"required"`
	Delta   int64  `json:"delta" binding:"required"` // 正=赠送 负=扣减
	Reason  string `json:"reason" binding:"required"`
}

// AdminAdjustResource 运营：手动赠送/扣减资源包（理由必填）
func AdminAdjustResource(c *gin.Context) {
	uid, err := strconv.Atoi(c.Param("userId"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的用户ID")
		return
	}
	staffID, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req AdjustResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	if err := EntitlementService.AdjustResource(uid, req.Subject, req.Delta, req.Reason, "staff:"+strconv.Itoa(staffID)); err != nil {
		framework.FailErr(c, err)
		return
	}
	caps, err := EntitlementService.Capacities(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, caps)
}
