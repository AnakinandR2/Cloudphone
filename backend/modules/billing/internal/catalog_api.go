package billing

import (
	"net/http"
	"strconv"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// ListSkus 前台：上架 SKU + 折扣阶梯（收银台渲染用）
func ListSkus(c *gin.Context) {
	list, err := CatalogService.ListListedSkus()
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

// QuoteRequest 计价请求
type QuoteRequest struct {
	SkuCode     string `json:"sku_code" binding:"required"`
	CycleMonths int    `json:"cycle_months"`
	Quantity    int    `json:"quantity" binding:"required"`
}

// Quote 前台：服务端权威计价
func Quote(c *gin.Context) {
	var req QuoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	res, err := CatalogService.Quote(req.SkuCode, req.CycleMonths, req.Quantity)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, res)
}

// adminSkuIDParam 解析 :id 路径参数（后台 SKU 用）
func adminSkuIDParam(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return 0, false
	}
	return id, true
}

// AdminListSkus 运营：列出全部 SKU（含下架）
func AdminListSkus(c *gin.Context) {
	list, err := CatalogService.ListSkus(true)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

// AdminCreateSku 运营：新建 SKU
func AdminCreateSku(c *gin.Context) {
	var req SkuCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	sku, err := CatalogService.CreateSku(&req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, sku)
}

// AdminUpdateSku 运营：更新 SKU
func AdminUpdateSku(c *gin.Context) {
	id, ok := adminSkuIDParam(c)
	if !ok {
		return
	}
	var req SkuUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	sku, err := CatalogService.UpdateSku(id, &req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, sku)
}

// AdminDeleteSku 运营：删除 SKU（连带折扣阶梯）
func AdminDeleteSku(c *gin.Context) {
	id, ok := adminSkuIDParam(c)
	if !ok {
		return
	}
	if err := CatalogService.DeleteSku(id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// AdminListTiers 运营：列出某 SKU 的折扣阶梯
func AdminListTiers(c *gin.Context) {
	id, ok := adminSkuIDParam(c)
	if !ok {
		return
	}
	list, err := CatalogService.ListTiers(id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

// AdminCreateTier 运营：给 SKU 加折扣阶梯
func AdminCreateTier(c *gin.Context) {
	id, ok := adminSkuIDParam(c)
	if !ok {
		return
	}
	var req TierCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	tier, err := CatalogService.CreateTier(id, &req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, tier)
}

// AdminUpdateTier 运营：更新折扣阶梯（:tierId）
func AdminUpdateTier(c *gin.Context) {
	tid, err := strconv.Atoi(c.Param("tierId"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	var req TierUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	tier, err := CatalogService.UpdateTier(tid, &req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, tier)
}

// AdminDeleteTier 运营：删除折扣阶梯（:tierId）
func AdminDeleteTier(c *gin.Context) {
	tid, err := strconv.Atoi(c.Param("tierId"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	if err := CatalogService.DeleteTier(tid); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}
