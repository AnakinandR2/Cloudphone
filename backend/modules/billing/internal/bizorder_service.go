package billing

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"manager-backend/framework/apperr"

	"gorm.io/gorm"
)

// errOrderAlreadyPaid 事务内 CAS 置已付返回 0 行的哨兵：订单已被并发赢家处理，
// 用于中止事务但对调用方视为幂等成功（非真实错误，外层用 errors.Is 吞掉）。
var errOrderAlreadyPaid = errors.New("order already paid")

// bizOrderServiceImpl 新购买模型订单服务：报价 → 下单 → 支付 → 统一履约。
type bizOrderServiceImpl struct {
	db      *gorm.DB
	repo    bizOrderRepository
	pricing *pricingConfigServiceImpl
	wallet  *walletServiceImpl
	fulfill *fulfillServiceImpl
}

// BizOrderService 模块内单例。
var BizOrderService *bizOrderServiceImpl

func newBizOrderService(db *gorm.DB, repo bizOrderRepository, pricing *pricingConfigServiceImpl, wallet *walletServiceImpl, fulfill *fulfillServiceImpl) *bizOrderServiceImpl {
	return &bizOrderServiceImpl{db: db, repo: repo, pricing: pricing, wallet: wallet, fulfill: fulfill}
}

// withTx 返回一个所有 repo/service 都绑定到同一 tx 的克隆，
// 使 pay / MarkPaid 的「扣款 + 履约 + 置已付」在单个数据库事务内原子完成
// （任一步失败整体回滚，杜绝"扣了钱没发货 / 重复扣款"）。pricing 为只读配置，直接复用。
func (s *bizOrderServiceImpl) withTx(tx *gorm.DB) *bizOrderServiceImpl {
	return &bizOrderServiceImpl{
		db:      tx,
		repo:    newBizOrderRepository(tx),
		pricing: s.pricing,
		wallet:  newWalletService(newRepository(tx)),
		fulfill: newFulfillService(newLicenseRepository(tx), newRuntimeWalletRepository(tx), newRepository(tx)),
	}
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

// Quote 服务端权威报价。userID 供已注册业务类型计算依赖当前用户状态的价格（如套餐补差价）。
func (s *bizOrderServiceImpl) Quote(userID int, req *BizQuoteRequest) (PriceQuote, error) {
	if !isValidBizType(req.BizType) {
		return PriceQuote{}, apperr.Validation("非法的业务类型")
	}
	// 已注册业务类型：委托其 quote 算权威价，包成 PriceQuote 供前端预览（meta_json 在下单时再算）。
	if rb, ok := lookupBizType(req.BizType); ok {
		res, err := rb.quote(userID, req.Params)
		if err != nil {
			return PriceQuote{}, err
		}
		return PriceQuote{
			Quantity:            1,
			PayableCents:        res.TotalCents,
			OriginalCents:       res.TotalCents,
			UnitPriceCents:      res.TotalCents,
			QtyDiscountBps:      DiscountBpsFull,
			DurationDiscountBps: DiscountBpsFull,
		}, nil
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
	if !isValidBizType(req.BizType) {
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
	default:
		// 已注册业务类型：调其 quote 得权威价 + meta_json，组装单行订单项（quantity=1）。
		rb, ok := lookupBizType(req.BizType)
		if !ok {
			return nil, apperr.Validation("非法的业务类型")
		}
		res, err := rb.quote(userID, req.Params)
		if err != nil {
			return nil, err
		}
		order.TotalCents = res.TotalCents
		item = BizOrderItem{
			TargetKind: req.BizType, Quantity: 1,
			UnitPriceCents: res.TotalCents, AmountCents: res.TotalCents,
			QtyDiscountBps: DiscountBpsFull, DurationDiscountBps: DiscountBpsFull,
			MetaJSON: string(res.MetaJSON),
		}
	}

	// 在 create 之前固化手续费（create 用 tx.Create 一次性按当前字段入库，之后无 Update 回写）。
	// 基数 = order.TotalCents（购买为折后应付、充值为面额）；配置缺失则 fee=0。
	if pmOK {
		order.FeePercentBps = pm.FeePercentBps
		order.FeeFixedCents = pm.FeeFixedCents
		order.FeeFreeThresholdCents = pm.FeeFreeThresholdCents
		order.FeeCents = computeFee(order.TotalCents, pm.FeePercentBps, pm.FeeFixedCents, pm.FeeFreeThresholdCents)
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
//
// chargeIfBalance 余额支付时扣款并写流水；第三方桩网关不动余额；TotalCents==0 跳过（如套餐降级）。
func (s *bizOrderServiceImpl) chargeIfBalance(userID int, o *BizOrder) error {
	if o.PayMethod == PayBalance && o.BizType != BizRecharge && o.TotalCents > 0 {
		// 实付 = 商品金额 + 手续费（余额方式 fee 恒 0，等价于扣 TotalCents）。
		return s.wallet.Charge(userID, o.TotalCents+o.FeeCents, LedgerPurchase, "购买:"+o.BizType, o.ID, "user:"+strconv.Itoa(userID))
	}
	return nil
}

func (s *bizOrderServiceImpl) pay(userID int, o *BizOrder, items []BizOrderItem) (PayResult, error) {
	// 【CAS 置已付 + 扣款 + 履约】收进单事务：CAS 先行——并发下仅一个赢家(rows=1)继续，其余读到
	// rows=0 幂等中止；任一步失败整体回滚，订单回到未支付且未扣款，可重试。扣款、履约（含注册型
	// 外部业务——单库 monolith 下经 tx 写自身表）与「置已付」恒原子，杜绝重复扣款、"已付却未扣款"、
	// 以及"已扣款却未履约"（丢钱）。
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		stx := s.withTx(tx)
		rows, err := stx.repo.markPaid(int(o.ID))
		if err != nil {
			return err
		}
		if rows == 0 {
			return errOrderAlreadyPaid
		}
		if err := stx.chargeIfBalance(userID, o); err != nil {
			return err
		}
		return stx.fulfillOrder(userID, o, items)
	})
	if txErr != nil {
		if errors.Is(txErr, errOrderAlreadyPaid) {
			now := time.Now()
			o.Status, o.PaidAt = BizOrderPaid, &now
			return PayResult{Status: BizOrderPaid, PayMethod: o.PayMethod}, nil
		}
		return PayResult{}, txErr
	}
	now := time.Now()
	o.Status, o.PaidAt = BizOrderPaid, &now
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
	// CAS 置已付 + 履约收进单事务：CAS 先行使并发/重复回调幂等(仅赢家履约)；履约（含注册型外部业务）
	// 一并纳入事务，失败整体回滚。
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		stx := s.withTx(tx)
		rows, err := stx.repo.markPaid(id)
		if err != nil {
			return err
		}
		if rows == 0 {
			return errOrderAlreadyPaid
		}
		return stx.fulfillOrder(int(o.UserID), o, items)
	})
	if txErr != nil {
		if errors.Is(txErr, errOrderAlreadyPaid) {
			o.Status = BizOrderPaid
			return o, nil
		}
		return nil, txErr
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
	// 已注册业务类型：调其 fulfill，回传订单项 meta_json 落地业务变更。
	if rb, ok := lookupBizType(o.BizType); ok {
		var meta []byte
		if len(items) > 0 {
			meta = []byte(items[0].MetaJSON)
		}
		// s.db 在支付事务内为绑定的 tx（见 withTx），外部履约随之纳入同一事务。
		return rb.fulfill(s.db, userID, o.ID, meta)
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
func (s *bizOrderServiceImpl) ListOrders(userID, page, size int, status, bizType string, from, to time.Time) ([]BizOrderWithItems, int64, error) {
	off, lim := pageOffset(page, size)
	orders, total, err := s.repo.listOwned(userID, off, lim, status, bizType, from, to)
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
