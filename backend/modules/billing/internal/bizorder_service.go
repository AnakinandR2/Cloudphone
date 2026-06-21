package billing

import (
	"strconv"
	"strings"
	"time"

	"manager-backend/framework/apperr"
)

// bizOrderServiceImpl 新购买模型订单服务：报价 → 下单 → 支付 → 统一履约。
type bizOrderServiceImpl struct {
	repo    bizOrderRepository
	pricing *pricingConfigServiceImpl
	wallet  *walletServiceImpl
	fulfill *fulfillServiceImpl
}

// BizOrderService 模块内单例。
var BizOrderService *bizOrderServiceImpl

func newBizOrderService(repo bizOrderRepository, pricing *pricingConfigServiceImpl, wallet *walletServiceImpl, fulfill *fulfillServiceImpl) *bizOrderServiceImpl {
	return &bizOrderServiceImpl{repo: repo, pricing: pricing, wallet: wallet, fulfill: fulfill}
}

// kindOf 把 biz_type 映射到授权单元 kind。
func kindOf(bizType string) (string, bool) {
	switch bizType {
	case BizSeatNew, BizSeatRenew:
		return KindSeat, true
	case BizBootSlotNew, BizBootSlotRenew:
		return KindBootSlot, true
	}
	return "", false
}

func isRenew(bizType string) bool {
	return bizType == BizSeatRenew || bizType == BizBootSlotRenew
}

// Quote 服务端权威报价。
func (s *bizOrderServiceImpl) Quote(req *BizQuoteRequest) (PriceQuote, error) {
	if !validBizTypes[req.BizType] {
		return PriceQuote{}, apperr.Validation("非法的业务类型")
	}
	if req.BizType == BizRuntimePack {
		return s.pricing.QuoteRuntimePack(req.Minutes)
	}
	if req.BizType == BizRecharge {
		return PriceQuote{}, apperr.Validation("充值无需报价")
	}
	kind, _ := kindOf(req.BizType)
	return s.pricing.QuoteKindDuration(kind, req.Quantity, req.DurationValue)
}

// CreateOrder 下单：组装订单项 + 计价 → 入库；余额支付即时扣款并履约，第三方返回待支付。
func (s *bizOrderServiceImpl) CreateOrder(userID int, req *BizOrderCreate) (*BizOrderResult, error) {
	if !validBizTypes[req.BizType] {
		return nil, apperr.Validation("非法的业务类型")
	}
	if !validPayMethods[req.PayMethod] {
		return nil, apperr.Validation("不支持的支付方式")
	}
	// 充值不能用余额支付。
	if req.BizType == BizRecharge && req.PayMethod == PayBalance {
		return nil, apperr.Validation("充值不支持余额支付")
	}

	order := BizOrder{UserID: uint(userID), BizType: req.BizType, Status: BizOrderUnpaid, PayMethod: req.PayMethod}
	var item BizOrderItem

	switch req.BizType {
	case BizRecharge:
		if req.AmountCents <= 0 {
			return nil, apperr.Validation("充值金额必须大于0")
		}
		order.TotalCents = req.AmountCents
		item = BizOrderItem{TargetKind: SubjectBalance, Quantity: 1, AmountCents: req.AmountCents, UnitPriceCents: req.AmountCents, QtyDiscountBps: DiscountBpsFull, DurationDiscountBps: DiscountBpsFull}
	case BizRuntimePack:
		q, err := s.pricing.QuoteRuntimePack(req.Minutes)
		if err != nil {
			return nil, err
		}
		order.TotalCents = q.PayableCents
		item = bizItemFromQuote(SubjectRuntimeMinute, q, "")
		item.Quantity = req.Minutes
	case BizSeatNew, BizBootSlotNew:
		kind, _ := kindOf(req.BizType)
		q, err := s.pricing.QuoteKindDuration(kind, req.Quantity, req.DurationValue)
		if err != nil {
			return nil, err
		}
		order.TotalCents = q.PayableCents
		item = bizItemFromQuote(kind, q, "")
		item.DurationUnit = s.durationUnit(kind)
	case BizSeatRenew, BizBootSlotRenew:
		kind, _ := kindOf(req.BizType)
		if len(req.UnitIDs) == 0 {
			return nil, apperr.Validation("请选择要续费的授权单元")
		}
		q, err := s.pricing.QuoteKindDuration(kind, len(req.UnitIDs), req.DurationValue)
		if err != nil {
			return nil, err
		}
		order.TotalCents = q.PayableCents
		item = bizItemFromQuote(kind, q, joinIDs(req.UnitIDs))
		item.DurationUnit = s.durationUnit(kind)
	}

	if err := s.repo.create(&order, []BizOrderItem{item}); err != nil {
		return nil, err
	}
	pay, err := s.pay(userID, &order, []BizOrderItem{item})
	if err != nil {
		return nil, err
	}
	return &BizOrderResult{Order: order, Pay: pay}, nil
}

// PayOrder 继续支付未支付订单。
func (s *bizOrderServiceImpl) PayOrder(userID, id int) (*BizOrderResult, error) {
	o, items, err := s.repo.getOwned(userID, id)
	if err != nil {
		if isNotFoundBizOrder(err) {
			return nil, apperr.NotFound("订单不存在")
		}
		return nil, err
	}
	if o.Status == BizOrderPaid {
		return &BizOrderResult{Order: *o, Pay: PayResult{Status: BizOrderPaid, PayMethod: o.PayMethod}}, nil
	}
	if o.Status == BizOrderExpired {
		return nil, apperr.Validation("订单已过期")
	}
	pay, err := s.pay(userID, o, items)
	if err != nil {
		return nil, err
	}
	return &BizOrderResult{Order: *o, Pay: pay}, nil
}

// pay 执行支付：余额支付即时扣款 + 履约 + 置 paid；第三方返回待支付 + stub 参数。
func (s *bizOrderServiceImpl) pay(userID int, o *BizOrder, items []BizOrderItem) (PayResult, error) {
	if o.PayMethod != PayBalance {
		// 第三方支付：返回待支付 + 桩支付参数（不扣余额、不履约）。
		return PayResult{
			Status:    "pending",
			PayMethod: o.PayMethod,
			PayParams: map[string]interface{}{
				"order_id":  o.ID,
				"amount":    o.TotalCents,
				"qr_stub":   "stub://pay/" + o.PayMethod + "/" + strconv.Itoa(int(o.ID)),
				"expire_in": 900,
			},
		}, nil
	}
	// 余额支付：扣款（充值除外）→ 履约 → 置 paid。
	if o.BizType != BizRecharge {
		if err := s.wallet.Charge(userID, o.TotalCents, LedgerPurchase, "购买:"+o.BizType, o.ID, "user:"+strconv.Itoa(userID)); err != nil {
			return PayResult{}, err
		}
	}
	if err := s.fulfillOrder(userID, o, items); err != nil {
		return PayResult{}, err
	}
	if err := s.repo.markPaid(int(o.ID)); err != nil {
		return PayResult{}, err
	}
	now := time.Now()
	o.Status = BizOrderPaid
	o.PaidAt = &now
	return PayResult{Status: BizOrderPaid, PayMethod: o.PayMethod}, nil
}

// MarkPaid 后台/网关回调桩：标记已付并履约（不扣余额）。
func (s *bizOrderServiceImpl) MarkPaid(id int) (*BizOrder, error) {
	o, items, err := s.repo.get(id)
	if err != nil {
		if isNotFoundBizOrder(err) {
			return nil, apperr.NotFound("订单不存在")
		}
		return nil, err
	}
	if o.Status == BizOrderPaid {
		return o, nil
	}
	if err := s.fulfillOrder(int(o.UserID), o, items); err != nil {
		return nil, err
	}
	if err := s.repo.markPaid(id); err != nil {
		return nil, err
	}
	o.Status = BizOrderPaid
	return o, nil
}

// fulfillOrder 根据订单类型走统一履约。
func (s *bizOrderServiceImpl) fulfillOrder(userID int, o *BizOrder, items []BizOrderItem) error {
	ref := "biz_order:" + strconv.Itoa(int(o.ID))
	switch o.BizType {
	case BizRecharge:
		return s.wallet.TopUp(userID, o.TotalCents, "充值", "user:"+strconv.Itoa(userID))
	case BizRuntimePack:
		return s.fulfill.FulfillRuntimePack(userID, items[0].Quantity, SourceOrder, ref)
	case BizSeatNew, BizBootSlotNew:
		kind, _ := kindOf(o.BizType)
		return s.fulfill.FulfillNew(userID, kind, items[0].Quantity, items[0].DurationValue, SourceOrder, ref)
	case BizSeatRenew, BizBootSlotRenew:
		kind, _ := kindOf(o.BizType)
		ids := splitIDs(items[0].RenewUnitIDs)
		return s.fulfill.FulfillRenew(userID, kind, ids, items[0].DurationValue)
	}
	return apperr.Validation("未知业务类型")
}

func (s *bizOrderServiceImpl) durationUnit(kind string) string {
	if kind == KindBootSlot {
		return "day"
	}
	return "month"
}

func (s *bizOrderServiceImpl) ListOrders(userID, page, size int, status string) ([]BizOrder, int64, error) {
	off, lim := pageOffset(page, size)
	return s.repo.listOwned(userID, off, lim, status)
}

func (s *bizOrderServiceImpl) GetOrder(userID, id int) (*BizOrder, []BizOrderItem, error) {
	o, items, err := s.repo.getOwned(userID, id)
	if err != nil {
		if isNotFoundBizOrder(err) {
			return nil, nil, apperr.NotFound("订单不存在")
		}
		return nil, nil, err
	}
	return o, items, nil
}

func (s *bizOrderServiceImpl) AdminListOrders(page, size, userID int, status string) ([]BizOrder, int64, error) {
	off, lim := pageOffset(page, size)
	return s.repo.listAll(off, lim, userID, status)
}

// AdminGetOrder 后台查任意订单详情（含订单项），不限属主。
func (s *bizOrderServiceImpl) AdminGetOrder(id int) (*BizOrder, []BizOrderItem, error) {
	o, items, err := s.repo.get(id)
	if err != nil {
		if isNotFoundBizOrder(err) {
			return nil, nil, apperr.NotFound("订单不存在")
		}
		return nil, nil, err
	}
	return o, items, nil
}

func bizItemFromQuote(kind string, q PriceQuote, renewIDs string) BizOrderItem {
	return BizOrderItem{
		TargetKind: kind, Quantity: q.Quantity, DurationValue: q.DurationValue,
		UnitPriceCents: q.UnitPriceCents, QtyDiscountBps: q.QtyDiscountBps,
		DurationDiscountBps: q.DurationDiscountBps, AmountCents: q.PayableCents,
		RenewUnitIDs: renewIDs,
	}
}

func joinIDs(ids []uint) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.Itoa(int(id)))
	}
	return strings.Join(parts, ",")
}

func splitIDs(s string) []uint {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]uint, 0, len(parts))
	for _, p := range parts {
		if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
			out = append(out, uint(n))
		}
	}
	return out
}

func pageOffset(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}
	return (page - 1) * size, size
}
