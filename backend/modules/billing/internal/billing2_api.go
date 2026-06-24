package billing

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// 新购买/费用模型 HTTP handler（契约 §1 用户端 + §2 后台）。新路径，不破坏旧端点。

// ---- 用户端 ----

// GetBillingOverview GET /billing/overview —— KPI 概览。
func GetBillingOverview(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	balance, err := WalletService.BalanceCents(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	seatTotal, err := LicenseService.Capacity(uid, KindSeat)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	bootTotal, err := LicenseService.Capacity(uid, KindBootSlot)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	mins, err := RuntimeWalletService.Remaining(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	// 已用席位由占用物化推导（current_instance_id 非空数）；包月名额不持久绑定实例，
	// 「在用」= 当前运行中的台数（向 phone 取），封顶到持有名额数。
	seatUsed := countOccupied(uid, KindSeat)
	bootInUse := bootSlotInUse(uid, bootTotal)
	framework.OKWithData(c, gin.H{
		"balance_cents":             balance,
		"seat":                      gin.H{"total": seatTotal, "used": seatUsed},
		"boot_slot":                 gin.H{"total": bootTotal, "in_use": bootInUse},
		"runtime_minutes_remaining": mins,
	})
}

// bootSlotInUse 包月名额「在用」数 = min(当前运行中的台数, 持有名额数)；下限 0。
func bootSlotInUse(userID, bootTotal int) int {
	n := runningInstanceCount(userID)
	if n > bootTotal {
		n = bootTotal
	}
	if n < 0 {
		n = 0
	}
	return n
}

func countOccupied(userID int, kind string) int {
	units, err := LicenseService.repo.activeUnits(userID, kind, time.Now())
	if err != nil {
		return 0
	}
	n := 0
	for _, u := range units {
		if u.CurrentInstanceID != "" {
			n++
		}
	}
	return n
}

// GetPurchaseConfig GET /billing/purchase-config —— 前端渲染拉条/按钮组/须知/支付方式。
func GetPurchaseConfig(c *gin.Context) {
	cfg, err := PricingConfigService.Get()
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	// 仅暴露启用的支付方式（按 sort 升序由前端排）。
	framework.OKWithData(c, gin.H{
		"payment_methods":        cfg.PaymentMethods,
		"recharge_presets_cents": cfg.RechargePresets,
		"kinds":                  cfg.Kinds,
		"runtime_pack":           cfg.Runtime,
	})
}

// BizQuote POST /billing/quote —— 服务端权威报价。
func BizQuote(c *gin.Context) {
	var req BizQuoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	q, err := BizOrderService.Quote(&req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{
		"quantity":              q.Quantity,
		"duration_value":        q.DurationValue,
		"unit_price_cents":      q.UnitPriceCents,
		"billing_units":         q.BillingUnits,
		"original_cents":        q.OriginalCents,
		"qty_discount_bps":      q.QtyDiscountBps,
		"duration_discount_bps": q.DurationDiscountBps,
		"payable_cents":         q.PayableCents,
		"gift_runtime_minutes":  q.GiftRuntimeMinutes,
	})
}

// ListLicenseUnits GET /billing/license-units?kind=&expiring_before=&keyword= —— 续费 tab 列表。
func ListLicenseUnits(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	kind := c.DefaultQuery("kind", KindSeat)
	var before time.Time
	if v := c.Query("expiring_before"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			before = t
		}
	}
	items, err := LicenseService.ListActiveUnits(uid, kind, before)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		items = filterUnitsByKeyword(items, kw)
	}
	framework.OKWithData(c, gin.H{"items": items})
}

// CreateBizOrder POST /billing/orders —— 新模型下单。
func CreateBizOrder(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req BizOrderCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	res, err := BizOrderService.CreateOrder(uid, &req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, res)
}

// ListBizOrders GET /billing/orders?status=&page=&size=
func ListBizOrders(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	var from, to time.Time
	if p := parseTimeQuery(c.Query("from")); p != nil {
		from = *p
	}
	if p := parseTimeQuery(c.Query("to")); p != nil {
		to = *p
	}
	list, total, err := BizOrderService.ListOrders(uid, page, size, c.Query("status"), from, to)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// GetBizOrder GET /billing/orders/:id
func GetBizOrder(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	o, items, err := BizOrderService.GetOrder(uid, id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{"id": o.ID, "biz_type": o.BizType, "status": o.Status,
		"total_cents": o.TotalCents, "pay_method": o.PayMethod, "created_at": o.CreatedAt,
		"paid_at": o.PaidAt, "expired_at": o.ExpiredAt,
		"fee_cents": o.FeeCents, "fee_percent_bps": o.FeePercentBps, "fee_fixed_cents": o.FeeFixedCents,
		"items": items})
}

// PayBizOrder POST /billing/orders/:id/pay —— 继续支付。
func PayBizOrder(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	res, err := BizOrderService.PayOrder(uid, id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, res)
}

// GetRuntimeLog GET /billing/runtime/log?page=&size= —— 费用日志（聚合到开机会话）。
func GetRuntimeLog(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	from := parseTimeQuery(c.Query("from"))
	to := parseTimeQuery(c.Query("to"))
	res, err := RuntimeEngineService.RuntimeLog(uid, page, size, nil, from, to)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, res)
}

// parseTimeQuery 解析时间段筛选参数：优先 RFC3339（前端 toISOString），兼容 datetime-local
// 的 "2006-01-02T15:04" 与日期 "2006-01-02"；空或无法解析返回 nil（不过滤）。
func parseTimeQuery(s string) *time.Time {
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}

// ---- 后台 ----

// AdminGetPricing GET /admin/billing/pricing
func AdminGetPricing(c *gin.Context) {
	cfg, err := PricingConfigService.Get()
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{"kinds": cfg.Kinds})
}

// AdminSavePricing PUT /admin/billing/pricing
func AdminSavePricing(c *gin.Context) {
	var body struct {
		Kinds map[string]KindPricing `json:"kinds"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	cfg, err := PricingConfigService.SavePartial(func(d *PricingConfigData) {
		if body.Kinds != nil {
			for k, v := range body.Kinds {
				d.Kinds[k] = v
			}
		}
	})
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{"kinds": cfg.Kinds})
}

// AdminGetRuntimePricing GET /admin/billing/runtime-config（新模型时长配置）
func AdminGetRuntimePricing(c *gin.Context) {
	cfg, err := PricingConfigService.Get()
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, cfg.Runtime)
}

// AdminSaveRuntimePricing PUT /admin/billing/runtime-config（新模型时长配置）
func AdminSaveRuntimePricing(c *gin.Context) {
	var body RuntimePackCfg
	if err := c.ShouldBindJSON(&body); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	cfg, err := PricingConfigService.SavePartial(func(d *PricingConfigData) { d.Runtime = body })
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, cfg.Runtime)
}

// AdminGetPaymentMethods GET /admin/billing/payment-methods
func AdminGetPaymentMethods(c *gin.Context) {
	cfg, err := PricingConfigService.Get()
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{"payment_methods": cfg.PaymentMethods})
}

// AdminSavePaymentMethods PUT /admin/billing/payment-methods
func AdminSavePaymentMethods(c *gin.Context) {
	var body struct {
		PaymentMethods []PaymentMethod `json:"payment_methods"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	// 手续费配置校验：比例 ∈ [0,10000]、固定 ≥ 0；余额方式（balance）两项必须都为 0。
	for _, pm := range body.PaymentMethods {
		if pm.FeePercentBps < 0 || pm.FeePercentBps > 10000 {
			framework.Fail(c, http.StatusBadRequest, "比例手续费需在 0-10000 基点（0-100%）之间")
			return
		}
		if pm.FeeFixedCents < 0 {
			framework.Fail(c, http.StatusBadRequest, "固定手续费不能为负")
			return
		}
		if pm.Code == PayBalance && (pm.FeePercentBps != 0 || pm.FeeFixedCents != 0) {
			framework.Fail(c, http.StatusBadRequest, "余额支付不可配置手续费")
			return
		}
	}
	cfg, err := PricingConfigService.SavePartial(func(d *PricingConfigData) { d.PaymentMethods = body.PaymentMethods })
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{"payment_methods": cfg.PaymentMethods})
}

// AdminGetRechargePresets GET /admin/billing/recharge-presets
func AdminGetRechargePresets(c *gin.Context) {
	cfg, err := PricingConfigService.Get()
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{"presets_cents": cfg.RechargePresets})
}

// AdminSaveRechargePresets PUT /admin/billing/recharge-presets
func AdminSaveRechargePresets(c *gin.Context) {
	var body struct {
		PresetsCents []int64 `json:"presets_cents"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	cfg, err := PricingConfigService.SavePartial(func(d *PricingConfigData) { d.RechargePresets = body.PresetsCents })
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{"presets_cents": cfg.RechargePresets})
}

// AdminGetNotices GET /admin/billing/notices
func AdminGetNotices(c *gin.Context) {
	cfg, err := PricingConfigService.Get()
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	notices := gin.H{}
	for k, v := range cfg.Kinds {
		notices[k] = gin.H{"notice": v.Notice, "billing_note": v.BillingNote}
	}
	notices["runtime_pack"] = gin.H{"notice": cfg.Runtime.Notice}
	framework.OKWithData(c, notices)
}

// AdminSaveNotices PUT /admin/billing/notices
func AdminSaveNotices(c *gin.Context) {
	var body map[string]struct {
		Notice      string `json:"notice"`
		BillingNote string `json:"billing_note"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	cfg, err := PricingConfigService.SavePartial(func(d *PricingConfigData) {
		for k, v := range body {
			if k == "runtime_pack" {
				d.Runtime.Notice = v.Notice
				continue
			}
			if kp, ok := d.Kinds[k]; ok {
				kp.Notice = v.Notice
				kp.BillingNote = v.BillingNote
				d.Kinds[k] = kp
			}
		}
	})
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, cfg.Kinds)
}

// AdminListBizOrders GET /admin/billing/biz-orders?userId=&status=&page=&size=
func AdminListBizOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	uid, _ := strconv.Atoi(c.DefaultQuery("userId", "0"))
	list, total, err := BizOrderService.AdminListOrders(page, size, uid, c.Query("status"))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// AdminMarkBizOrderPaid POST /admin/billing/biz-orders/:id/mark-paid
func AdminMarkBizOrderPaid(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	o, err := BizOrderService.MarkPaid(id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, o)
}

// AdminGetBizOrder GET /admin/billing/biz-orders/:id —— 后台订单详情（含订单项）。
func AdminGetBizOrder(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	o, items, err := BizOrderService.AdminGetOrder(id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{"id": o.ID, "user_id": o.UserID, "biz_type": o.BizType, "status": o.Status,
		"total_cents": o.TotalCents, "pay_method": o.PayMethod, "created_at": o.CreatedAt,
		"paid_at": o.PaidAt, "expired_at": o.ExpiredAt,
		"fee_cents": o.FeeCents, "fee_percent_bps": o.FeePercentBps, "fee_fixed_cents": o.FeeFixedCents,
		"items": items})
}

// AdminAdjustResourceV2 POST /admin/billing/accounts/:userId/adjust-resource —— 走统一履约（source=grant）。
// 支持 subject ∈ {seat, boot_slot, runtime_minute}；seat/boot_slot 需带 duration_value。
func AdminAdjustResourceV2(c *gin.Context) {
	uid, err := strconv.Atoi(c.Param("userId"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的用户ID")
		return
	}
	var body struct {
		Subject       string `json:"subject" binding:"required"`
		Quantity      int    `json:"quantity"`
		DurationValue int    `json:"duration_value"`
		Minutes       int    `json:"minutes"`
		Reason        string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	staffID, _ := currentUserID(c)
	ref := "staff:" + strconv.Itoa(staffID)
	switch body.Subject {
	case KindSeat, KindBootSlot:
		err = FulfillService.FulfillNew(uid, body.Subject, body.Quantity, body.DurationValue, SourceGrant, ref)
	case SubjectRuntimeMinute:
		err = FulfillService.FulfillRuntimePack(uid, body.Minutes, SourceGrant, ref)
	default:
		framework.Fail(c, http.StatusBadRequest, "非法的资源科目")
		return
	}
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}
