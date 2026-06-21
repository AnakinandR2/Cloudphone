package billing

import (
	"net/http"
	"strconv"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// ListMyTrials 前台：可领试用列表
func ListMyTrials(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	list, err := TrialService.ListClaimable(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

// ClaimTrial 前台：领取试用（:code）
func ClaimTrial(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	code := c.Param("code")
	var req ClaimRequest
	_ = c.ShouldBindJSON(&req)
	if err := TrialService.ClaimTrial(uid, code, req.InviteCode); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// AdminListTrialPolicies 后台：全部试用策略
func AdminListTrialPolicies(c *gin.Context) {
	list, err := TrialService.ListPolicies()
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

// AdminCreateTrialPolicy 后台：新建试用策略
func AdminCreateTrialPolicy(c *gin.Context) {
	var req TrialPolicyCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	p, err := TrialService.CreatePolicy(&req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, p)
}

// AdminUpdateTrialPolicy 后台：更新试用策略
func AdminUpdateTrialPolicy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	var req TrialPolicyUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	p, err := TrialService.UpdatePolicy(id, &req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, p)
}

// AdminDeleteTrialPolicy 后台：删除试用策略
func AdminDeleteTrialPolicy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	if err := TrialService.DeletePolicy(id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// AdminGrantTrialEligibility 后台：给用户授予领取资格（:id=policyID）
func AdminGrantTrialEligibility(c *gin.Context) {
	pid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	staffID, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req EligibilityGrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	if err := TrialService.GrantEligibility(pid, req.UserID, "staff:"+strconv.Itoa(staffID)); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// AdminFeatureTrialPolicy 后台：单选标记某策略为营销站展示（:id=policyID）
func AdminFeatureTrialPolicy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	var req TrialFeatureRequest
	_ = c.ShouldBindJSON(&req)
	if err := TrialService.SetMarketingFeatured(id, req.Featured); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// AdminListTrialGrants 后台：某策略发放记录（:id=policyID）
func AdminListTrialGrants(c *gin.Context) {
	pid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	list, err := TrialService.ListGrants(pid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}
