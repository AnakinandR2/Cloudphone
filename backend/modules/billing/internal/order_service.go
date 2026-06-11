package billing

import "manager-backend/framework/apperr"

type orderServiceImpl struct {
	repo    orderRepository
	catalog *catalogServiceImpl
}

var OrderService *orderServiceImpl

func newOrderService(repo orderRepository, catalog *catalogServiceImpl) *orderServiceImpl {
	return &orderServiceImpl{repo: repo, catalog: catalog}
}

func (s *orderServiceImpl) CreateOrder(userID int, req *OrderCreate) (*OrderDetail, error) {
	if !validPayMethods[req.PayMethod] {
		return nil, apperr.Validation("不支持的支付方式")
	}
	if len(req.Items) == 0 {
		return nil, apperr.Validation("订单项不能为空")
	}
	var total int64
	items := make([]OrderItem, 0, len(req.Items))
	for _, it := range req.Items {
		q, err := s.catalog.Quote(it.SkuCode, it.CycleMonths, it.Quantity)
		if err != nil {
			return nil, err
		}
		items = append(items, OrderItem{
			SkuCode: q.SkuCode, SkuName: q.SkuName, Category: q.Category,
			CycleMonths: q.CycleMonths, Quantity: q.Quantity,
			UnitPriceCents: q.UnitPriceCents, DiscountBps: q.DiscountBps,
			OriginalCents: q.OriginalCents, PayableCents: q.PayableCents,
		})
		total += q.PayableCents
	}
	order := Order{OrderNo: genOrderNo(), UserID: uint(userID), Status: OrderPending, PayMethod: req.PayMethod, TotalCents: total}
	if err := s.repo.createOrder(&order, items); err != nil {
		return nil, err
	}
	return &OrderDetail{Order: order, Items: items}, nil
}

func (s *orderServiceImpl) GetOrder(userID, id int) (*OrderDetail, error) {
	o, err := s.repo.getOwned(userID, id)
	if err != nil {
		if isNotFoundOrder(err) {
			return nil, apperr.NotFound("订单不存在")
		}
		return nil, err
	}
	items, err := s.repo.listItems(int(o.ID))
	if err != nil {
		return nil, err
	}
	return &OrderDetail{Order: *o, Items: items}, nil
}

func (s *orderServiceImpl) ListOrders(userID, page, size int, status string) ([]Order, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}
	return s.repo.listOrders(userID, (page-1)*size, size, status)
}

func (s *orderServiceImpl) AdminListOrders(page, size, userID int, status string) ([]Order, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}
	return s.repo.adminListOrders((page-1)*size, size, userID, status)
}

// PayOrder 前台支付本人订单（单事务幂等）。
// 余额支付：扣余额 + 发放；微信/支付宝：即时到账桩（不扣余额，直接结算发放，
// 模拟第三方支付秒回执），上线真实网关后改为创建预支付单并由回调结算。
func (s *orderServiceImpl) PayOrder(userID, id int) (*OrderDetail, error) {
	o, err := s.repo.getOwned(userID, id)
	if err != nil {
		if isNotFoundOrder(err) {
			return nil, apperr.NotFound("订单不存在")
		}
		return nil, err
	}
	items, err := s.repo.listItems(int(o.ID))
	if err != nil {
		return nil, err
	}
	if err := s.repo.settle(o, items, o.PayMethod == PayBalance); err != nil {
		return nil, err
	}
	return &OrderDetail{Order: *o, Items: items}, nil
}

// MarkPaid 后台/网关回调桩：标记已付并发放（不扣余额）。
func (s *orderServiceImpl) MarkPaid(id int) (*OrderDetail, error) {
	o, err := s.repo.getByID(id)
	if err != nil {
		if isNotFoundOrder(err) {
			return nil, apperr.NotFound("订单不存在")
		}
		return nil, err
	}
	items, err := s.repo.listItems(int(o.ID))
	if err != nil {
		return nil, err
	}
	if err := s.repo.settle(o, items, false); err != nil {
		return nil, err
	}
	return &OrderDetail{Order: *o, Items: items}, nil
}
