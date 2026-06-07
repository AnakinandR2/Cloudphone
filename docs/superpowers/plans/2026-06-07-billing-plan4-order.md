# 计费系统 Phase 1 · 计划 4：订单 / 收银台 / 支付桩 + 发放 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把目录(计价 `Quote`)、钱包(余额)、权益(`Grant`)串成购买闭环：下单落价格快照、余额支付或后台「标记已付」(网关桩)，支付成功在**单一事务内幂等**地扣款并按品类发放权益。

**Architecture:** 复用 `billing/internal` 单包（`order_*.go` 前缀分子域）。订单 `Order`+`OrderItem`（下单时调 `CatalogService.Quote` 落 `payable/原价/折扣/单价` 快照）。**结算 settle** 是核心：一个 `db.Transaction` 内——①守卫式标记 `pending→paid`（`RowsAffected==0` 即已付/已取消，幂等跳过）②余额支付则守卫式扣余额+写流水 ③按订单项发放权益（`instance_fee→instance_seat`、`boot_pack→boot_seat`、`time_pack→runtime_minute=小时×60`；订阅类到期=now+周期，时长包永久）+写流水。事务收敛于 `order_repository`（符合仓库「事务收敛于 repo」约定，跨 billing 表）；发放复用 tx 绑定的 `gormEntitlementRepository`，避免重复逻辑。金额整数分。

**Tech Stack:** Go 1.26 / Gin / GORM（sqlite 测试）/ testify。模块名 `manager-backend`，分支 `feature/billing-order`。

---

## 领域模型

- **Order** `billing_orders`：`{order_no(uniqueIndex), user_id, status(pending/paid/cancelled), pay_method(balance/wechat/alipay), total_cents, paid_at(*time), 时间戳}`。
- **OrderItem** `billing_order_items`：`{order_id, sku_code, sku_name(快照), category, cycle_months, quantity, unit_price_cents, discount_bps, original_cents, payable_cents, 时间戳}`（全部为下单时 `Quote` 的快照，后续改价不影响历史单）。
- 新增流水类型 `LedgerPurchase = "purchase"`（发放权益时记账）。

## 结算时序

```
PayWithBalance(uid, orderId)  →  settle(order, items, deductBalance=true)
MarkPaid(orderId)[后台桩]      →  settle(order, items, deductBalance=false)

settle (单事务):
  1. UPDATE orders SET status='paid',paid_at=? WHERE id=? AND status='pending'  (RowsAffected==0 → 已结算, 幂等返回)
  2. if deductBalance: UPDATE accounts SET balance=balance-total WHERE user_id=? AND balance>=total (==0 → 余额不足, 回滚) + 写 balance 流水(consume,-total)
  3. for each item: 建权益批次(品类→科目/数量/到期) + 写资源流水(purchase,+qty)
```

## 计划 4 文件结构

| 文件 | 职责 | 动作 |
|---|---|---|
| `internal/order_model.go` | `Order`/`OrderItem`、状态/支付常量、DTO | 创建 |
| `internal/order_repository.go` | `orderRepository`：建单/查/列/`settle`(跨表事务) | 创建 |
| `internal/order_service.go` | `OrderService`：CreateOrder/PayWithBalance/MarkPaid/Get/List | 创建 |
| `internal/order_api.go` | 前台下单/列表/详情/支付；后台列表/标记已付 | 创建 |
| `internal/order_service_test.go` | 服务层测试 | 创建 |
| `internal/model.go` | 加 `LedgerPurchase` 常量 | 修改 |
| `internal/module.go` | Init 装配 `OrderService`；建表加订单两表；路由 | 修改 |
| `internal/main_test.go` | AutoMigrate 加订单两表 | 修改 |

**约定速查**：`framework.OK*/Fail/FailErr`；`apperr.Validation/NotFound/Conflict`；前台 `user.AuthMiddleware()`（`currentUserID`）；后台 `staff.PermissionMiddleware("billing:view"|"billing:manage")`；Go 可用 `time.Now()`；测试 `framework.CleanTable`。已有：`CatalogService.Quote(skuCode,cycleMonths,quantity)→*QuoteResult{Category,CycleMonths,Quantity,UnitPriceCents,DiscountBps,OriginalCents,PayableCents}`；`EntitlementService`/`gormEntitlementRepository`（`createBatch`/`capacity(uid,subject,now)`/`insertLedger`）；资源科目常量 `SubjectInstanceSeat/BootSeat/RuntimeMinute`；品类常量 `CategoryInstanceFee/BootPack/TimePack`；`SourceOrder`。

---

## Task 1：订单模型 + 仓储 + CreateOrder（报价快照）

**Files:** Create `order_model.go`, `order_repository.go`, `order_service.go`, `order_service_test.go`; Modify `model.go`(常量), `module.go`(建表+Init), `main_test.go`(AutoMigrate)。

- [ ] **Step 1：创建 `order_model.go`**

```go
package billing

import "time"

// 订单状态
const (
	OrderPending   = "pending"
	OrderPaid      = "paid"
	OrderCancelled = "cancelled"
)

// 支付方式
const (
	PayBalance = "balance"
	PayWechat  = "wechat"
	PayAlipay  = "alipay"
)

var validPayMethods = map[string]bool{PayBalance: true, PayWechat: true, PayAlipay: true}

// Order 订单（一次购买）。
type Order struct {
	ID         uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	OrderNo    string     `gorm:"type:varchar(40);not null;uniqueIndex:idx_billing_order_no" json:"order_no"`
	UserID     uint       `gorm:"not null;index:idx_billing_order_user" json:"user_id"`
	Status     string     `gorm:"type:varchar(20);not null" json:"status"`
	PayMethod  string     `gorm:"type:varchar(20);not null" json:"pay_method"`
	TotalCents int64      `gorm:"not null" json:"total_cents"`
	PaidAt     *time.Time `json:"paid_at"`
	CreatedAt  time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Order) TableName() string { return "billing_orders" }

// OrderItem 订单项（下单时的价格快照）。
type OrderItem struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	OrderID        uint      `gorm:"not null;index:idx_billing_orderitem_order" json:"order_id"`
	SkuCode        string    `gorm:"type:varchar(64);not null" json:"sku_code"`
	SkuName        string    `gorm:"type:varchar(100)" json:"sku_name"`
	Category       string    `gorm:"type:varchar(20);not null" json:"category"`
	CycleMonths    int       `gorm:"not null;default:0" json:"cycle_months"`
	Quantity       int       `gorm:"not null" json:"quantity"`
	UnitPriceCents int64     `gorm:"not null" json:"unit_price_cents"`
	DiscountBps    int       `gorm:"not null;default:10000" json:"discount_bps"`
	OriginalCents  int64     `gorm:"not null" json:"original_cents"`
	PayableCents   int64     `gorm:"not null" json:"payable_cents"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (OrderItem) TableName() string { return "billing_order_items" }

// OrderItemRequest 下单请求项
type OrderItemRequest struct {
	SkuCode     string `json:"sku_code" binding:"required"`
	CycleMonths int    `json:"cycle_months"`
	Quantity    int    `json:"quantity" binding:"required"`
}

// OrderCreate 下单请求
type OrderCreate struct {
	Items     []OrderItemRequest `json:"items" binding:"required,min=1"`
	PayMethod string             `json:"pay_method" binding:"required"`
}

// OrderDetail 订单 + 项
type OrderDetail struct {
	Order Order       `json:"order"`
	Items []OrderItem `json:"items"`
}
```

- [ ] **Step 2：`model.go` 加 `LedgerPurchase` 常量**

在 `model.go` 的流水类型常量块（`LedgerTopup/.../LedgerAdjustDeduct`）中追加一行：
```go
	LedgerPurchase = "purchase" // 订单支付成功发放权益
```

- [ ] **Step 3：`main_test.go` + `module.go` AutoMigrate 加订单两表**

`main_test.go`：
```go
	if err := framework.DB.AutoMigrate(&Account{}, &LedgerEntry{}, &Sku{}, &DiscountTier{}, &EntitlementBatch{}, &Order{}, &OrderItem{}); err != nil {
```
`module.go` 的 `RegisterSetup` 内 AutoMigrate 同样追加 `&Order{}, &OrderItem{}`。

- [ ] **Step 4：创建 `order_repository.go`（建单/查/列；settle 在 Task 2 加）**

```go
package billing

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type orderRepository interface {
	createOrder(order *Order, items []OrderItem) error
	getByID(id int) (*Order, error)
	getOwned(userID, id int) (*Order, error)
	listItems(orderID int) ([]OrderItem, error)
	listOrders(userID, offset, limit int, status string) ([]Order, int64, error)
	adminListOrders(offset, limit int, userID int, status string) ([]Order, int64, error)
	settle(order *Order, items []OrderItem, deductBalance bool) error
}

type gormOrderRepository struct{ db *gorm.DB }

func newOrderRepository(db *gorm.DB) orderRepository { return &gormOrderRepository{db: db} }

// createOrder 在一个事务内建订单 + 订单项。
func (r *gormOrderRepository) createOrder(order *Order, items []OrderItem) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		for i := range items {
			items[i].OrderID = order.ID
			if err := tx.Create(&items[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *gormOrderRepository) getByID(id int) (*Order, error) {
	var o Order
	if err := r.db.Where("id = ?", id).First(&o).Error; err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *gormOrderRepository) getOwned(userID, id int) (*Order, error) {
	var o Order
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&o).Error; err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *gormOrderRepository) listItems(orderID int) ([]OrderItem, error) {
	var items []OrderItem
	err := r.db.Where("order_id = ?", orderID).Order("id ASC").Find(&items).Error
	return items, err
}

func (r *gormOrderRepository) listOrders(userID, offset, limit int, status string) ([]Order, int64, error) {
	q := r.db.Model(&Order{}).Where("user_id = ?", userID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []Order
	err := q.Order("id DESC").Offset(offset).Limit(limit).Find(&items).Error
	return items, total, err
}

func (r *gormOrderRepository) adminListOrders(offset, limit int, userID int, status string) ([]Order, int64, error) {
	q := r.db.Model(&Order{})
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []Order
	err := q.Order("id DESC").Offset(offset).Limit(limit).Find(&items).Error
	return items, total, err
}

// settle 在 Task 2 实现（占位以满足接口）。
func (r *gormOrderRepository) settle(order *Order, items []OrderItem, deductBalance bool) error {
	return errors.New("not implemented") // TODO(Task 2)
}

// isNotFoundOrder 便于 service 翻译。
func isNotFoundOrder(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }

// genOrderNo 生成订单号（纳秒高分辨率；uniqueIndex 兜底）。
func genOrderNo() string {
	return fmt.Sprintf("BIL%d", time.Now().UnixNano())
}
```

- [ ] **Step 5：创建 `order_service.go`（CreateOrder；Pay/MarkPaid 在 Task 2）**

```go
package billing

import "manager-backend/framework/apperr"

type orderServiceImpl struct {
	repo    orderRepository
	catalog *catalogServiceImpl
}

// OrderService 模块内实例，由 module.Init 注入。
var OrderService *orderServiceImpl

func newOrderService(repo orderRepository, catalog *catalogServiceImpl) *orderServiceImpl {
	return &orderServiceImpl{repo: repo, catalog: catalog}
}

// CreateOrder 逐项 Quote 落快照、求和，建 pending 订单。
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
		sku, err := s.catalog.repo.getSkuByCode(it.SkuCode)
		if err != nil {
			return nil, err
		}
		items = append(items, OrderItem{
			SkuCode: q.SkuCode, SkuName: sku.Name, Category: q.Category,
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

// GetOrder 取本人订单 + 项。
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

// ListOrders 本人订单列表。
func (s *orderServiceImpl) ListOrders(userID, page, size int, status string) ([]Order, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}
	return s.repo.listOrders(userID, (page-1)*size, size, status)
}

// AdminListOrders 全部订单（可按用户/状态过滤）。
func (s *orderServiceImpl) AdminListOrders(page, size, userID int, status string) ([]Order, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}
	return s.repo.adminListOrders((page-1)*size, size, userID, status)
}
```

- [ ] **Step 6：`module.go` Init 装配 OrderService**

```go
func (m *billingModule) Init(db *gorm.DB) error {
	BillingService = newService(newRepository(db))
	CatalogService = newCatalogService(newCatalogRepository(db))
	EntitlementService = newEntitlementService(newEntitlementRepository(db))
	OrderService = newOrderService(newOrderRepository(db), CatalogService)
	return nil
}
```

- [ ] **Step 7：写测试（追加 `order_service_test.go`）**

```go
package billing

import (
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ordUser = 9101

func TestCreateOrderSnapshotsAndTotal(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_orders", "billing_order_items", "billing_skus", "billing_discount_tiers")
	})
	require.NoError(t, SeedCatalog(framework.DB))

	// 实例费 年付 5 台(180000*0.7=126000) + 时长包 1000h(20000*0.8=16000)
	d, err := OrderService.CreateOrder(ordUser, &OrderCreate{
		PayMethod: PayBalance,
		Items: []OrderItemRequest{
			{SkuCode: "instance_fee", CycleMonths: 12, Quantity: 5},
			{SkuCode: "time_pack", CycleMonths: 0, Quantity: 1000},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, OrderPending, d.Order.Status)
	assert.Equal(t, int64(142000), d.Order.TotalCents) // 126000 + 16000
	require.Len(t, d.Items, 2)
	assert.Equal(t, int64(126000), d.Items[0].PayableCents)
	assert.Equal(t, "云手机实例费", d.Items[0].SkuName)
	assert.NotEmpty(t, d.Order.OrderNo)

	// 读取本人订单
	got, err := OrderService.GetOrder(ordUser, int(d.Order.ID))
	require.NoError(t, err)
	assert.Equal(t, d.Order.OrderNo, got.Order.OrderNo)
	// 他人不可见
	_, err = OrderService.GetOrder(ordUser+1, int(d.Order.ID))
	assert.Error(t, err)
}

func TestCreateOrderGuards(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_orders", "billing_order_items", "billing_skus", "billing_discount_tiers")
	})
	require.NoError(t, SeedCatalog(framework.DB))

	_, err := OrderService.CreateOrder(ordUser, &OrderCreate{PayMethod: "btc", Items: []OrderItemRequest{{SkuCode: "instance_fee", CycleMonths: 1, Quantity: 1}}})
	assert.Error(t, err) // 支付方式非法
	_, err = OrderService.CreateOrder(ordUser, &OrderCreate{PayMethod: PayBalance, Items: []OrderItemRequest{{SkuCode: "nope", CycleMonths: 1, Quantity: 1}}})
	assert.Error(t, err) // sku 不存在（Quote 报错）
}
```

- [ ] **Step 8：测试 + 质量门 + 提交**

```bash
cd /home/root/workspace005/gloryphone-code/backend && go test ./modules/billing/internal/ -run 'TestCreateOrder' -v && go test ./modules/billing/internal/ -v && gofmt -l modules/billing/ && go vet ./modules/billing/... && go build ./...
cd /home/root/workspace005/gloryphone-code
git add backend/modules/billing/internal
git commit -m "feat(billing): 订单模型+仓储+CreateOrder(逐项Quote落快照求和)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

> 注：`settle` 此 Task 为占位（返回 not implemented），Task 2 实现。`order_service.go` 引用 `s.catalog.repo.getSkuByCode` —— `catalogServiceImpl` 的 `repo` 字段为小写私有，同包可访问。

---

## Task 2：结算 settle（余额支付 / 标记已付 + 原子发放 + 幂等）

**Files:** Modify `order_repository.go`(`settle` 真实现 + 发放助手), `order_service.go`(PayWithBalance/MarkPaid); Test 追加。

- [ ] **Step 1：写失败测试（追加 `order_service_test.go`）**

```go
func TestPayWithBalanceFulfills(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_orders", "billing_order_items", "billing_skus", "billing_discount_tiers",
			"billing_accounts", "billing_ledger_entries", "billing_entitlement_batches")
	})
	require.NoError(t, SeedCatalog(framework.DB))

	// 充足余额
	_, err := BillingService.Topup(ordUser, 1000000, "充值", "user:9101")
	require.NoError(t, err)

	// 下单：实例费 月付 2 台(6000) + 时长包 1000h(16000) = 22000
	d, err := OrderService.CreateOrder(ordUser, &OrderCreate{
		PayMethod: PayBalance,
		Items: []OrderItemRequest{
			{SkuCode: "instance_fee", CycleMonths: 1, Quantity: 2},
			{SkuCode: "time_pack", CycleMonths: 0, Quantity: 1000},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(22000), d.Order.TotalCents)

	paid, err := OrderService.PayWithBalance(ordUser, int(d.Order.ID))
	require.NoError(t, err)
	assert.Equal(t, OrderPaid, paid.Order.Status)
	require.NotNil(t, paid.Order.PaidAt)

	// 余额扣减
	acc, err := BillingService.GetAccount(ordUser)
	require.NoError(t, err)
	assert.Equal(t, int64(1000000-22000), acc.BalanceCents)

	// 权益已发放：实例席位 2、时长 60000 分钟(1000h*60)
	snap, err := EntitlementService.Capacities(ordUser)
	require.NoError(t, err)
	assert.Equal(t, int64(2), snap.InstanceSeat)
	assert.Equal(t, int64(60000), snap.RuntimeMinute)

	// 幂等：再次支付应报错（已结算），且不二次发放
	_, err = OrderService.PayWithBalance(ordUser, int(d.Order.ID))
	assert.Error(t, err)
	snap, _ = EntitlementService.Capacities(ordUser)
	assert.Equal(t, int64(2), snap.InstanceSeat)
}

func TestPayWithBalanceInsufficient(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_orders", "billing_order_items", "billing_skus", "billing_discount_tiers",
			"billing_accounts", "billing_ledger_entries", "billing_entitlement_batches")
	})
	require.NoError(t, SeedCatalog(framework.DB))

	d, err := OrderService.CreateOrder(ordUser, &OrderCreate{PayMethod: PayBalance,
		Items: []OrderItemRequest{{SkuCode: "instance_fee", CycleMonths: 1, Quantity: 2}}})
	require.NoError(t, err)

	_, err = OrderService.PayWithBalance(ordUser, int(d.Order.ID))
	assert.Error(t, err) // 余额不足

	// 订单仍 pending，无发放
	got, _ := OrderService.GetOrder(ordUser, int(d.Order.ID))
	assert.Equal(t, OrderPending, got.Order.Status)
	snap, _ := EntitlementService.Capacities(ordUser)
	assert.Equal(t, int64(0), snap.InstanceSeat)
}

func TestMarkPaidFulfillsWithoutBalance(t *testing.T) {
	t.Cleanup(func() {
		framework.CleanTable("billing_orders", "billing_order_items", "billing_skus", "billing_discount_tiers",
			"billing_accounts", "billing_ledger_entries", "billing_entitlement_batches")
	})
	require.NoError(t, SeedCatalog(framework.DB))

	// 微信支付下单（不预扣余额），后台标记已付 → 仅发放
	d, err := OrderService.CreateOrder(ordUser, &OrderCreate{PayMethod: PayWechat,
		Items: []OrderItemRequest{{SkuCode: "boot_pack", CycleMonths: 3, Quantity: 4}}})
	require.NoError(t, err)

	paid, err := OrderService.MarkPaid(int(d.Order.ID))
	require.NoError(t, err)
	assert.Equal(t, OrderPaid, paid.Order.Status)

	snap, err := EntitlementService.Capacities(ordUser)
	require.NoError(t, err)
	assert.Equal(t, int64(4), snap.BootSeat)

	// 余额未动（无账户或为0）
	acc, _ := BillingService.GetAccount(ordUser)
	assert.Equal(t, int64(0), acc.BalanceCents)
}
```

- [ ] **Step 2：测试 → FAIL（PayWithBalance/MarkPaid 未定义 + settle 占位）**

`cd /home/root/workspace005/gloryphone-code/backend && go test ./modules/billing/internal/ -run 'TestPayWithBalance|TestMarkPaid' -v`

- [ ] **Step 3：实现 `settle` + 发放助手（替换 `order_repository.go` 占位）**

把 `settle` 占位替换，并加发放助手：

```go
// settle 单事务结算：守卫式标记已付（幂等）→（可选）扣余额+写流水 → 按项发放权益+写流水。
func (r *gormOrderRepository) settle(order *Order, items []OrderItem, deductBalance bool) error {
	now := time.Now()
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 1. 幂等标记已付
		res := tx.Model(&Order{}).
			Where("id = ? AND status = ?", order.ID, OrderPending).
			Updates(map[string]interface{}{"status": OrderPaid, "paid_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return apperr.Conflict("订单状态不可支付（已支付或已取消）")
		}
		// 2. 余额支付：守卫式扣减 + 流水
		if deductBalance {
			ar := tx.Model(&Account{}).
				Where("user_id = ? AND balance_cents >= ?", order.UserID, order.TotalCents).
				Updates(map[string]interface{}{"balance_cents": gorm.Expr("balance_cents - ?", order.TotalCents)})
			if ar.Error != nil {
				return ar.Error
			}
			if ar.RowsAffected == 0 {
				return apperr.Validation("余额不足")
			}
			var acc Account
			if err := tx.Where("user_id = ?", order.UserID).First(&acc).Error; err != nil {
				return err
			}
			if err := tx.Create(&LedgerEntry{
				UserID: order.UserID, Subject: SubjectBalance, Type: LedgerConsume,
				Delta: -order.TotalCents, BalanceAfter: acc.BalanceCents,
				Reason: order.OrderNo, OrderID: order.ID, Operator: "user:" + itoa(int(order.UserID)),
			}).Error; err != nil {
				return err
			}
		}
		// 3. 发放权益
		entRepo := &gormEntitlementRepository{db: tx}
		for _, it := range items {
			if err := fulfillItem(entRepo, int(order.UserID), it, order.OrderNo, now); err != nil {
				return err
			}
		}
		order.Status = OrderPaid
		order.PaidAt = &now
		return nil
	})
}

// fulfillItem 按订单项品类发放对应资源权益 + 写流水。
func fulfillItem(entRepo entitlementRepository, userID int, it OrderItem, orderNo string, now time.Time) error {
	var subject string
	var qty int64
	var expireAt *time.Time
	switch it.Category {
	case CategoryInstanceFee:
		subject, qty = SubjectInstanceSeat, int64(it.Quantity)
		exp := now.AddDate(0, it.CycleMonths, 0)
		expireAt = &exp
	case CategoryBootPack:
		subject, qty = SubjectBootSeat, int64(it.Quantity)
		exp := now.AddDate(0, it.CycleMonths, 0)
		expireAt = &exp
	case CategoryTimePack:
		subject, qty = SubjectRuntimeMinute, int64(it.Quantity)*60 // 小时→分钟
		expireAt = nil                                             // 时长包永久
	default:
		return apperr.Validation("未知商品类别")
	}
	batch := EntitlementBatch{UserID: uint(userID), Subject: subject, Quantity: qty, Source: SourceOrder, SourceRef: orderNo, ExpireAt: expireAt}
	if err := entRepo.createBatch(&batch); err != nil {
		return err
	}
	capacity, err := entRepo.capacity(userID, subject, now)
	if err != nil {
		return err
	}
	return entRepo.insertLedger(&LedgerEntry{
		UserID: uint(userID), Subject: subject, Type: LedgerPurchase,
		Delta: qty, BalanceAfter: capacity, Reason: orderNo, Operator: "user:" + itoa(userID),
	})
}
```

在 `order_repository.go` import 加 `"manager-backend/framework/apperr"` 与 `"strconv"`，并加一个本地 `itoa`：
```go
import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"manager-backend/framework/apperr"
	"gorm.io/gorm"
)

func itoa(n int) string { return strconv.Itoa(n) }
```

- [ ] **Step 4：实现 `PayWithBalance` / `MarkPaid`（`order_service.go` 追加）**

```go
// PayWithBalance 用余额支付本人订单（扣款+发放，单事务幂等）。
func (s *orderServiceImpl) PayWithBalance(userID, id int) (*OrderDetail, error) {
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
	if err := s.repo.settle(o, items, true); err != nil {
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
```

- [ ] **Step 5：测试 → PASS（全部 billing）**

`cd /home/root/workspace005/gloryphone-code/backend && go test ./modules/billing/internal/ -v`

- [ ] **Step 6：质量门 + 提交**

```bash
cd /home/root/workspace005/gloryphone-code/backend && gofmt -l modules/billing/ && go vet ./modules/billing/... && go build ./...
cd /home/root/workspace005/gloryphone-code
git add backend/modules/billing/internal
git commit -m "feat(billing): 订单结算 settle(余额支付/标记已付+原子发放+幂等)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 3：前台 / 后台 订单 API + 路由

> handler 薄转发；build+vet 验证装配。

**Files:** Create `order_api.go`; Modify `module.go`(路由)。

- [ ] **Step 1：创建 `order_api.go`**

```go
package billing

import (
	"net/http"
	"strconv"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// CreateOrder 前台：下单（pending）
func CreateOrder(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req OrderCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	d, err := OrderService.CreateOrder(uid, &req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, d)
}

// ListMyOrders 前台：我的订单
func ListMyOrders(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, total, err := OrderService.ListOrders(uid, page, size, c.Query("status"))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// GetMyOrder 前台：订单详情
func GetMyOrder(c *gin.Context) {
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
	d, err := OrderService.GetOrder(uid, id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, d)
}

// PayMyOrder 前台：余额支付
func PayMyOrder(c *gin.Context) {
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
	d, err := OrderService.PayWithBalance(uid, id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, d)
}

// AdminListOrders 后台：全部订单（?userId=&status=）
func AdminListOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	uid, _ := strconv.Atoi(c.DefaultQuery("userId", "0"))
	list, total, err := OrderService.AdminListOrders(page, size, uid, c.Query("status"))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// AdminMarkOrderPaid 后台：标记已付（网关回调桩）→ 发放
func AdminMarkOrderPaid(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	d, err := OrderService.MarkPaid(id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, d)
}
```

- [ ] **Step 2：`module.go` 加路由**

前台 `/billing` 组内追加：
```go
		g.POST("/orders", CreateOrder)
		g.GET("/orders", ListMyOrders)
		g.GET("/orders/:id", GetMyOrder)
		g.POST("/orders/:id/pay", PayMyOrder)
```
后台 `/admin/billing` 组内追加：
```go
		admin.GET("/orders", staff.PermissionMiddleware("billing:view"), AdminListOrders)
		admin.POST("/orders/:id/mark-paid", staff.PermissionMiddleware("billing:manage"), AdminMarkOrderPaid)
```

> Gin 冲突检查：前台已有 `/skus`、`/quote`、`/entitlements`、`/account`、`/ledger`、`/topup`；新增 `/orders`、`/orders/:id`、`/orders/:id/pay` 同前缀层级一致，无通配冲突。后台 `/orders`、`/orders/:id/mark-paid` 与既有 `/skus/:id`、`/accounts/:userId` 分属不同前缀。**装配后跑全量测试**（apptest 启动会暴露任何 Gin panic）。

- [ ] **Step 3：全量回归 + 质量门（paste 输出）**

```bash
cd /home/root/workspace005/gloryphone-code/backend
gofmt -l modules/billing/
go vet ./...
go test ./... 2>&1 | tail -20
go test ./framework/ -run TestModuleBoundaries -v 2>&1 | tail -5
go build ./...
```
Expect 全 ok、边界 PASS、无 Gin 冲突 panic。

- [ ] **Step 4：提交**

```bash
cd /home/root/workspace005/gloryphone-code
git add backend/modules/billing/internal
git commit -m "feat(billing): 前台下单/列表/详情/支付 + 后台订单/标记已付 API与路由

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## 计划 4 验收对照（PRD §8）

| 需求 | 覆盖 |
|---|---|
| 下单落价格快照（Quote）+ 求和 | Task 1 CreateOrder |
| 订单状态机 pending→paid（无 refunded） | Task 2 settle 守卫 |
| 余额支付（充足扣/不足拒） | Task 2 PayWithBalance（AC-5） |
| 支付成功发放权益（事务、按品类映射、时长 h→min、订阅到期） | Task 2 fulfillItem |
| 幂等（重复支付不二次发放） | Task 2 守卫式 mark-paid（AC-6） |
| 网关支付（Phase1 桩） | Task 2 MarkPaid + Task 3 后台 mark-paid |
| 前台订单、后台订单管理 | Task 3 |

> 真实微信/支付宝网关下单+回调签名验证 = 后续（PRD §14.2，本期桩）。

## Self-Review 记录

- **Spec 覆盖**：覆盖 PRD §8（下单/收银台/支付/发放/幂等）；真实网关桩化、退款用调整（前计划）已说明。
- **占位扫描**：Task 1 `settle` 占位（返回 not implemented）于 Task 2 替换，已标注；其余完整代码+命令。
- **类型一致**：`Order`/`OrderItem`/`OrderCreate`/`OrderItemRequest`/`OrderDetail`、`CreateOrder/GetOrder/ListOrders/AdminListOrders/PayWithBalance/MarkPaid`、`settle(order,items,deductBalance)`、`fulfillItem(entRepo,userID,item,orderNo,now)` 贯穿一致；`LedgerPurchase` 新常量；复用 `CatalogService.Quote`/`gormEntitlementRepository`/`SubjectXxx`/`CategoryXxx`/`SourceOrder`。
- **事务/幂等**：settle 单事务（标记已付守卫 + 余额守卫 + 发放）；重复支付经 `RowsAffected==0` 幂等拒绝、不二次发放；余额不足整体回滚（订单留 pending）。
- **单位**：金额分；时长包 小时→分钟（×60）；订阅到期 `AddDate(0, cycleMonths, 0)`。
- **并发**：sqlite 串行；mysql/pg 行锁硬化沿用 [[Plan3]] 待办（settle 的余额/订单守卫式 UPDATE 已是原子，发放批次受同事务保护）。
