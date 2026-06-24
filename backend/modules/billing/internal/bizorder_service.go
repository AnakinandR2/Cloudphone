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
	q, err := s.pricing.QuoteKindDuration(kind, req.Quantity, req.DurationValue)
	if err != nil {
		return q, err
	}
	// 席位新购/续费：附带赠送的临时开机时长（分钟），前端在订单摘要展示。
	if kind == KindSeat {
		q.GiftRuntimeMinutes = s.seatGiftMinutes(req.Quantity, req.DurationValue)
	}
	return q, nil
}

// seatGiftMinutes 计算席位赠送时长：每席位每月分钟数 × 席位数 × 月数（配置为 0 时返回 0）。
func (s *bizOrderServiceImpl) seatGiftMinutes(seatQty, months int) int {
	if seatQty < 1 || months < 1 {
		return 0
	}
	per, err := s.pricing.GiftMinutesPerSeatMonth()
	if err != nil || per <= 0 {
		return 0
	}
	return per * seatQty * months
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
	// §4.3 支付方式校验：命中配置且 enabled 才放行（前端只展示 enabled 方式，后端兜底拒绝被禁用/已删方式）。
	// 配置缺失（理论上应覆盖白名单）则容错按无手续费处理、不阻断（向后兼容）。
	pm, pmOK := s.pricing.PaymentMethodByCode(req.PayMethod)
	if pmOK && !pm.Enabled {
		return nil, apperr.Validation("该支付方式已停用")
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

	// 在 create 之前固化手续费（create 用 tx.Create 一次性按当前字段入库，之后无 Update 回写）。
	// 基数 = order.TotalCents（购买为折后应付、充值为面额）；配置缺失则 fee=0。
	if pmOK {
		order.FeePercentBps = pm.FeePercentBps
		order.FeeFixedCents = pm.FeeFixedCents
		order.FeeCents = computeFee(order.TotalCents, pm.FeePercentBps, pm.FeeFixedCents)
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

// pay 执行支付：
//   - 余额支付：即时扣款（充值除外）→ 履约 → 置 paid。
//   - 第三方支付：当前为桩网关，无真实对接 → 视为即时到账，直接履约 + 置 paid（不扣余额，
//     钱由外部网关收取）。后续接入真实网关时，这里改回返回 pending + 走 MarkPaid 回调。
func (s *bizOrderServiceImpl) pay(userID int, o *BizOrder, items []BizOrderItem) (PayResult, error) {
	// 余额支付才从钱包扣款；第三方由外部网关收款，不动余额。
	if o.PayMethod == PayBalance && o.BizType != BizRecharge {
		// 实付 = 商品金额 + 手续费（余额方式 fee 恒 0，等价于扣 TotalCents，行为不变）。
		if err := s.wallet.Charge(userID, o.TotalCents+o.FeeCents, LedgerPurchase, "购买:"+o.BizType, o.ID, "user:"+strconv.Itoa(userID)); err != nil {
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
		if err := s.fulfill.FulfillNew(userID, kind, items[0].Quantity, items[0].DurationValue, SourceOrder, ref); err != nil {
			return err
		}
		if kind == KindSeat {
			return s.grantSeatGift(userID, o, items[0].Quantity, items[0].DurationValue, ref)
		}
		return nil
	case BizSeatRenew, BizBootSlotRenew:
		kind, _ := kindOf(o.BizType)
		ids := splitIDs(items[0].RenewUnitIDs)
		if err := s.fulfill.FulfillRenew(userID, kind, ids, items[0].DurationValue); err != nil {
			return err
		}
		if kind == KindSeat {
			return s.grantSeatGift(userID, o, len(ids), items[0].DurationValue, ref)
		}
		return nil
	}
	return apperr.Validation("未知业务类型")
}

// grantSeatGift 席位新购/续费赠送临时开机时长：发放余量 + 记录到订单（gift_runtime_minutes）。
func (s *bizOrderServiceImpl) grantSeatGift(userID int, o *BizOrder, seatQty, months int, ref string) error {
	gift := s.seatGiftMinutes(seatQty, months)
	if gift <= 0 {
		return nil
	}
	if _, err := s.fulfill.GiftRuntime(userID, gift, ref); err != nil {
		return err
	}
	if err := s.repo.setGift(int(o.ID), gift); err != nil {
		return err
	}
	o.GiftRuntimeMinutes = gift
	return nil
}

func (s *bizOrderServiceImpl) durationUnit(kind string) string {
	if kind == KindBootSlot {
		return "day"
	}
	return "month"
}

// ListOrders 订单历史：按状态 + 创建时间区间过滤，随单返回订单项明细。
func (s *bizOrderServiceImpl) ListOrders(userID, page, size int, status string, from, to time.Time) ([]BizOrderWithItems, int64, error) {
	off, lim := pageOffset(page, size)
	orders, total, err := s.repo.listOwned(userID, off, lim, status, from, to)
	if err != nil {
		return nil, 0, err
	}
	ids := make([]uint, 0, len(orders))
	for _, o := range orders {
		ids = append(ids, o.ID)
	}
	itemMap, err := s.repo.itemsByOrders(ids)
	if err != nil {
		return nil, 0, err
	}
	out := make([]BizOrderWithItems, 0, len(orders))
	for _, o := range orders {
		items := itemMap[o.ID]
		if items == nil {
			items = []BizOrderItem{}
		}
		out = append(out, BizOrderWithItems{BizOrder: o, Items: items})
	}
	return out, total, nil
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
