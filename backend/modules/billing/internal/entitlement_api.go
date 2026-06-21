package billing

import (
	"net/http"

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
