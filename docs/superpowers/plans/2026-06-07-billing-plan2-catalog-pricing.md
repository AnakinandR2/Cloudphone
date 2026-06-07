# 计费系统 Phase 1 · 计划 2：商品目录与定价 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在已落地的 `billing` 模块中加入「商品目录 + 阶梯折扣 + 服务端权威计价」：SKU（实例费/包月开机包/时长包三类）、折扣阶梯（按 周期×数量 / 时长档），以及一个 `Quote` 计价引擎；配套前台只读列表 + 计价接口、后台 SKU/折扣 CRUD。

**Architecture:** 复用 Plan 1 的约定与 `billing/internal` 单包，按 staff 模块 `role_*.go` 的先例用文件前缀 `catalog_` 划分子域（与账户/钱包文件并存，同一 Go 包）。金额一律整数分（int64）。折扣用**基点 bps**（10000=全价无折扣，8500=8.5折=付85%）。计价在服务端计算（前端不可信），下单（计划4）将复用 `Quote`。新增 `CatalogService`，与 `BillingService` 一起在 `module.Init` 装配。

**Tech Stack:** Go 1.26 / Gin / GORM（sqlite 测试）/ testify。模块名 `manager-backend`，分支 `feature/billing-catalog`。

---

## 领域模型

- **SKU** `billing_skus`：可购买产品，三类 `category ∈ {instance_fee, boot_pack, time_pack}`。`UnitPriceCents` 语义：instance_fee/boot_pack = 分/台/月；time_pack = 分/小时。
- **DiscountTier** `billing_discount_tiers`：`(sku_id, cycle_months, min_quantity, discount_bps)`。
  - 订阅类（instance_fee/boot_pack）：`cycle_months ∈ {1,3,12}`，`min_quantity` = 台数门槛。
  - 时长包（time_pack）：`cycle_months = 0`，`min_quantity` = 小时门槛。
  - 一条阶梯同时编码「周期折扣 + 数量折扣」：解析时取「同周期、`min_quantity ≤ 请求量` 中 `min_quantity` 最大」的那条；无匹配则 `bps=10000`。
- **计价 Quote**：`原价 = UnitPriceCents × 计费单位`（订阅=`cycle_months×台数`；时长包=`小时数`）；`应付 = round_half_up(原价 × bps / 10000)`。

## 计划 2 文件结构

| 文件 | 职责 | 动作 |
|---|---|---|
| `backend/modules/billing/internal/catalog_model.go` | `Sku`/`DiscountTier`、常量、请求 DTO、`QuoteResult` | 创建 |
| `backend/modules/billing/internal/catalog_repository.go` | `catalogRepository` 接口 + GORM 实现（SKU/阶梯 CRUD + 查询） | 创建 |
| `backend/modules/billing/internal/catalog_service.go` | `CatalogService`：CRUD、`ListListedSkus`、`Quote`、`SeedCatalog` | 创建 |
| `backend/modules/billing/internal/catalog_api.go` | 前台 + 后台 handler | 创建 |
| `backend/modules/billing/internal/catalog_service_test.go` | 服务层测试 | 创建 |
| `backend/modules/billing/internal/module.go` | 装配 `CatalogService`、建表+seed、路由 | 修改 |
| `backend/modules/billing/internal/main_test.go` | AutoMigrate 增加 catalog 表 | 修改 |

**约定速查**：`framework.OK/OKWithData/OKWithPage/Fail/FailErr`；`apperr.Validation/NotFound`；前台 `user.AuthMiddleware()`（`c.Get("userID")`→int）；后台 `admin.Use(middlewareFuncs...)` + `staff.PermissionMiddleware("billing:view"|"billing:manage")`；测试 `framework.SetupTestDB`+`framework.CleanTable`。`currentUserID` helper 已存在于 `api.go`（勿重复定义）。

---

## Task 1：Catalog 模型 + 建表 + 幂等 seed

**Files:** Create `catalog_model.go`; Modify `module.go`, `main_test.go`; Test `catalog_service_test.go`（占位，Task 1 仅验证建表+seed 经 service）。注：`SeedCatalog` 服务方法在本 Task 创建（见 Step 4 临时最小实现），Task 2/3 继续补 service。

- [ ] **Step 1：创建 `catalog_model.go`**

```go
package billing

import "time"

// SKU 类别
const (
	CategoryInstanceFee = "instance_fee" // 实例费：分/台/月
	CategoryBootPack    = "boot_pack"    // 包月开机包：分/台/月
	CategoryTimePack    = "time_pack"    // 时长包：分/小时
)

// DiscountBpsFull 全价（无折扣）基点。8500=8.5折=付85%。
const DiscountBpsFull = 10000

// Sku 可购买商品（运营维护）。
type Sku struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Code           string    `gorm:"type:varchar(64);not null;uniqueIndex:idx_billing_sku_code" json:"code"`
	Category       string    `gorm:"type:varchar(20);not null" json:"category"` // instance_fee/boot_pack/time_pack
	Name           string    `gorm:"type:varchar(100);not null" json:"name"`
	Description    string    `gorm:"type:varchar(255)" json:"description"`
	UnitPriceCents int64     `gorm:"not null" json:"unit_price_cents"` // 订阅类=分/台/月；时长包=分/小时
	Unit           string    `gorm:"type:varchar(20)" json:"unit"`     // 展示单位，如 "台/月"、"小时"
	Listed         bool      `gorm:"not null;default:true" json:"listed"`
	Sort           int       `gorm:"not null;default:0" json:"sort"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Sku) TableName() string { return "billing_skus" }

// DiscountTier 折扣阶梯：同周期下，达到 MinQuantity 起按 DiscountBps 计价。
type DiscountTier struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	SkuID       uint      `gorm:"not null;index:idx_billing_tier_sku" json:"sku_id"`
	CycleMonths int       `gorm:"not null;default:0" json:"cycle_months"` // 订阅=1/3/12；时长包=0
	MinQuantity int       `gorm:"not null;default:1" json:"min_quantity"` // 台数 或 小时 门槛
	DiscountBps int       `gorm:"not null;default:10000" json:"discount_bps"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (DiscountTier) TableName() string { return "billing_discount_tiers" }

// SkuCreate / SkuUpdate / TierCreate / TierUpdate 请求 DTO
type SkuCreate struct {
	Code           string `json:"code" binding:"required"`
	Category       string `json:"category" binding:"required"`
	Name           string `json:"name" binding:"required"`
	Description    string `json:"description"`
	UnitPriceCents int64  `json:"unit_price_cents" binding:"required"`
	Unit           string `json:"unit"`
	Listed         *bool  `json:"listed"` // 指针：缺省视为 true
	Sort           int    `json:"sort"`
}

type SkuUpdate struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	UnitPriceCents *int64 `json:"unit_price_cents"`
	Unit           string `json:"unit"`
	Listed         *bool  `json:"listed"`
	Sort           *int   `json:"sort"`
}

type TierCreate struct {
	CycleMonths int `json:"cycle_months"`
	MinQuantity int `json:"min_quantity" binding:"required"`
	DiscountBps int `json:"discount_bps" binding:"required"`
}

type TierUpdate struct {
	CycleMonths *int `json:"cycle_months"`
	MinQuantity *int `json:"min_quantity"`
	DiscountBps *int `json:"discount_bps"`
}

// SkuWithTiers 前台列表项：SKU + 其折扣阶梯。
type SkuWithTiers struct {
	Sku   Sku            `json:"sku"`
	Tiers []DiscountTier `json:"tiers"`
}

// QuoteResult 计价结果（服务端权威）。
type QuoteResult struct {
	SkuCode        string `json:"sku_code"`
	Category       string `json:"category"`
	CycleMonths    int    `json:"cycle_months"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	BillingUnits   int    `json:"billing_units"`
	OriginalCents  int64  `json:"original_cents"`
	DiscountBps    int    `json:"discount_bps"`
	PayableCents   int64  `json:"payable_cents"`
}
```

- [ ] **Step 2：`main_test.go` 增加 catalog 表的 AutoMigrate**

把 `main_test.go` 里的 AutoMigrate 行改为：
```go
	if err := framework.DB.AutoMigrate(&Account{}, &LedgerEntry{}, &Sku{}, &DiscountTier{}); err != nil {
		panic(err)
	}
```

- [ ] **Step 3：`module.go` 建表 + seed**

把 `init()` 里的 `RegisterSetup` 改为同时建 catalog 表并 seed（幂等）：
```go
	// 建表（幂等）：计费账户 + 统一流水 + 商品目录 + 折扣阶梯。
	framework.RegisterSetup(func(db *gorm.DB) error {
		if err := db.AutoMigrate(&Account{}, &LedgerEntry{}, &Sku{}, &DiscountTier{}); err != nil {
			return err
		}
		return SeedCatalog(db)
	})
```

- [ ] **Step 4：创建 `catalog_service.go`，先实现 `SeedCatalog`（幂等 seed）**

```go
package billing

import "gorm.io/gorm"

// SeedCatalog 幂等写入三类默认 SKU 及其折扣阶梯（按 code 查重，存在即跳过该 SKU）。
// 默认价仅为开箱即用值，运营可在后台改价/调折扣。
func SeedCatalog(db *gorm.DB) error {
	type seedSku struct {
		sku   Sku
		tiers []DiscountTier
	}
	seeds := []seedSku{
		{
			sku: Sku{Code: "instance_fee", Category: CategoryInstanceFee, Name: "云手机实例费", Unit: "台/月", UnitPriceCents: 3000, Listed: true, Sort: 1},
			tiers: []DiscountTier{
				{CycleMonths: 1, MinQuantity: 1, DiscountBps: 10000},
				{CycleMonths: 3, MinQuantity: 1, DiscountBps: 8500},
				{CycleMonths: 12, MinQuantity: 1, DiscountBps: 7000},
			},
		},
		{
			sku: Sku{Code: "boot_pack", Category: CategoryBootPack, Name: "包月开机包", Unit: "台/月", UnitPriceCents: 2000, Listed: true, Sort: 2},
			tiers: []DiscountTier{
				{CycleMonths: 1, MinQuantity: 1, DiscountBps: 10000},
				{CycleMonths: 3, MinQuantity: 1, DiscountBps: 8500},
				{CycleMonths: 12, MinQuantity: 1, DiscountBps: 7000},
			},
		},
		{
			sku: Sku{Code: "time_pack", Category: CategoryTimePack, Name: "时长包", Unit: "小时", UnitPriceCents: 20, Listed: true, Sort: 3},
			tiers: []DiscountTier{
				{CycleMonths: 0, MinQuantity: 1, DiscountBps: 10000},
				{CycleMonths: 0, MinQuantity: 500, DiscountBps: 9000},
				{CycleMonths: 0, MinQuantity: 1000, DiscountBps: 8000},
			},
		},
	}
	for _, s := range seeds {
		var existing Sku
		err := db.Where("code = ?", s.sku.Code).First(&existing).Error
		if err == nil {
			continue // 已存在，跳过（幂等）
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		created := s.sku
		if err := db.Create(&created).Error; err != nil {
			return err
		}
		for i := range s.tiers {
			s.tiers[i].SkuID = created.ID
			if err := db.Create(&s.tiers[i]).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
```

- [ ] **Step 5：写测试（seed 幂等）— 创建 `catalog_service_test.go`**

```go
package billing

import (
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedCatalogIdempotent(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_skus", "billing_discount_tiers") })

	require.NoError(t, SeedCatalog(framework.DB))
	var skuCount, tierCount int64
	framework.DB.Model(&Sku{}).Count(&skuCount)
	framework.DB.Model(&DiscountTier{}).Count(&tierCount)
	assert.Equal(t, int64(3), skuCount)
	assert.Equal(t, int64(9), tierCount)

	// 再次 seed 不应重复插入
	require.NoError(t, SeedCatalog(framework.DB))
	framework.DB.Model(&Sku{}).Count(&skuCount)
	framework.DB.Model(&DiscountTier{}).Count(&tierCount)
	assert.Equal(t, int64(3), skuCount)
	assert.Equal(t, int64(9), tierCount)
}
```

- [ ] **Step 6：跑测试 → PASS**

`cd /home/root/workspace005/gloryphone-code/backend && go test ./modules/billing/internal/ -run TestSeedCatalogIdempotent -v`

- [ ] **Step 7：质量门 + 提交**

```bash
cd /home/root/workspace005/gloryphone-code/backend && gofmt -l modules/billing/ && go vet ./modules/billing/... && go build ./... && go test ./modules/billing/internal/ -v
cd /home/root/workspace005/gloryphone-code
git add backend/modules/billing/internal
git commit -m "feat(billing): 商品目录模型(SKU/折扣阶梯)+建表+幂等seed

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 2：Catalog 仓储 + SKU/阶梯 CRUD 服务

**Files:** Create `catalog_repository.go`; Modify `catalog_service.go`, `module.go`(Init 装配 CatalogService); Test 追加 `catalog_service_test.go`。

- [ ] **Step 1：写失败测试（追加）**

```go
func TestSkuAndTierCRUD(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_skus", "billing_discount_tiers") })

	listed := true
	sku, err := CatalogService.CreateSku(&SkuCreate{Code: "x_fee", Category: CategoryInstanceFee, Name: "测试费", UnitPriceCents: 5000, Unit: "台/月", Listed: &listed})
	require.NoError(t, err)
	assert.NotZero(t, sku.ID)
	assert.True(t, sku.Listed)

	// code 重复 → 冲突
	_, err = CatalogService.CreateSku(&SkuCreate{Code: "x_fee", Category: CategoryInstanceFee, Name: "重复", UnitPriceCents: 1})
	assert.Error(t, err)

	// 非法 category → 校验失败
	_, err = CatalogService.CreateSku(&SkuCreate{Code: "bad", Category: "nope", Name: "x", UnitPriceCents: 1})
	assert.Error(t, err)

	// 更新
	newPrice := int64(6000)
	upd, err := CatalogService.UpdateSku(int(sku.ID), &SkuUpdate{UnitPriceCents: &newPrice})
	require.NoError(t, err)
	assert.Equal(t, int64(6000), upd.UnitPriceCents)

	// 阶梯
	tier, err := CatalogService.CreateTier(int(sku.ID), &TierCreate{CycleMonths: 12, MinQuantity: 1, DiscountBps: 7000})
	require.NoError(t, err)
	assert.Equal(t, uint(sku.ID), tier.SkuID)

	// bps 越界 → 校验失败
	_, err = CatalogService.CreateTier(int(sku.ID), &TierCreate{CycleMonths: 1, MinQuantity: 1, DiscountBps: 12000})
	assert.Error(t, err)

	tiers, err := CatalogService.ListTiers(int(sku.ID))
	require.NoError(t, err)
	require.Len(t, tiers, 1)

	require.NoError(t, CatalogService.DeleteTier(int(tier.ID)))
	require.NoError(t, CatalogService.DeleteSku(int(sku.ID)))
	_, err = CatalogService.GetSku(int(sku.ID))
	assert.Error(t, err)
}

func TestListListedSkus(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_skus", "billing_discount_tiers") })
	require.NoError(t, SeedCatalog(framework.DB))

	// 下架一个
	all, err := CatalogService.ListSkus(true)
	require.NoError(t, err)
	require.NotEmpty(t, all)
	notListed := false
	_, err = CatalogService.UpdateSku(int(all[0].ID), &SkuUpdate{Listed: &notListed})
	require.NoError(t, err)

	listed, err := CatalogService.ListListedSkus()
	require.NoError(t, err)
	assert.Len(t, listed, 2) // 3 个 seed，下架 1 个
	for _, s := range listed {
		assert.True(t, s.Sku.Listed)
	}
}
```

- [ ] **Step 2：跑测试 → FAIL（CatalogService 未定义）**

`cd /home/root/workspace005/gloryphone-code/backend && go test ./modules/billing/internal/ -run 'TestSkuAndTierCRUD|TestListListedSkus' -v`

- [ ] **Step 3：创建 `catalog_repository.go`**

```go
package billing

import (
	"errors"

	"gorm.io/gorm"
)

type catalogRepository interface {
	createSku(s *Sku) error
	updateSku(id int, fields map[string]interface{}) error
	deleteSku(id int) error
	getSku(id int) (*Sku, error)
	getSkuByCode(code string) (*Sku, error)
	listSkus(listedOnly bool) ([]Sku, error)

	createTier(t *DiscountTier) error
	updateTier(id int, fields map[string]interface{}) error
	deleteTier(id int) error
	getTier(id int) (*DiscountTier, error)
	listTiersBySku(skuID int) ([]DiscountTier, error)
	listTiersBySkuCycle(skuID, cycleMonths int) ([]DiscountTier, error)
}

type gormCatalogRepository struct{ db *gorm.DB }

func newCatalogRepository(db *gorm.DB) catalogRepository { return &gormCatalogRepository{db: db} }

func (r *gormCatalogRepository) createSku(s *Sku) error { return r.db.Create(s).Error }

func (r *gormCatalogRepository) updateSku(id int, fields map[string]interface{}) error {
	return r.db.Model(&Sku{}).Where("id = ?", id).Updates(fields).Error
}

func (r *gormCatalogRepository) deleteSku(id int) error {
	return r.db.Where("id = ?", id).Delete(&Sku{}).Error
}

func (r *gormCatalogRepository) getSku(id int) (*Sku, error) {
	var s Sku
	if err := r.db.Where("id = ?", id).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *gormCatalogRepository) getSkuByCode(code string) (*Sku, error) {
	var s Sku
	if err := r.db.Where("code = ?", code).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *gormCatalogRepository) listSkus(listedOnly bool) ([]Sku, error) {
	q := r.db.Model(&Sku{})
	if listedOnly {
		q = q.Where("listed = ?", true)
	}
	var items []Sku
	err := q.Order("sort ASC, id ASC").Find(&items).Error
	return items, err
}

func (r *gormCatalogRepository) createTier(t *DiscountTier) error { return r.db.Create(t).Error }

func (r *gormCatalogRepository) updateTier(id int, fields map[string]interface{}) error {
	return r.db.Model(&DiscountTier{}).Where("id = ?", id).Updates(fields).Error
}

func (r *gormCatalogRepository) deleteTier(id int) error {
	return r.db.Where("id = ?", id).Delete(&DiscountTier{}).Error
}

func (r *gormCatalogRepository) getTier(id int) (*DiscountTier, error) {
	var t DiscountTier
	if err := r.db.Where("id = ?", id).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *gormCatalogRepository) listTiersBySku(skuID int) ([]DiscountTier, error) {
	var items []DiscountTier
	err := r.db.Where("sku_id = ?", skuID).Order("cycle_months ASC, min_quantity ASC").Find(&items).Error
	return items, err
}

func (r *gormCatalogRepository) listTiersBySkuCycle(skuID, cycleMonths int) ([]DiscountTier, error) {
	var items []DiscountTier
	err := r.db.Where("sku_id = ? AND cycle_months = ?", skuID, cycleMonths).
		Order("min_quantity ASC").Find(&items).Error
	return items, err
}

// isNotFound 便于 service 把 GORM not-found 翻译成领域错误。
func isNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
```

- [ ] **Step 4：在 `catalog_service.go` 加 `CatalogService` 服务体 + CRUD**

在 `catalog_service.go` 顶部（`import` 之后）追加服务定义与方法（保留已有的 `SeedCatalog`）：

```go
import (
	"manager-backend/framework/apperr"

	"gorm.io/gorm"
)

// catalogServiceImpl 商品目录服务。
type catalogServiceImpl struct{ repo catalogRepository }

// CatalogService 模块内实例，由 module.Init 注入 DB 后装配。
var CatalogService *catalogServiceImpl

func newCatalogService(repo catalogRepository) *catalogServiceImpl {
	return &catalogServiceImpl{repo: repo}
}

var validCategory = map[string]bool{
	CategoryInstanceFee: true, CategoryBootPack: true, CategoryTimePack: true,
}

func validBps(bps int) bool { return bps >= 0 && bps <= DiscountBpsFull }

// CreateSku 新建 SKU（运营）。
func (s *catalogServiceImpl) CreateSku(req *SkuCreate) (*Sku, error) {
	if !validCategory[req.Category] {
		return nil, apperr.Validation("非法的商品类别")
	}
	if req.UnitPriceCents < 0 {
		return nil, apperr.Validation("单价不能为负")
	}
	if _, err := s.repo.getSkuByCode(req.Code); err == nil {
		return nil, apperr.Conflict("商品编码已存在")
	} else if !isNotFound(err) {
		return nil, err
	}
	listed := true
	if req.Listed != nil {
		listed = *req.Listed
	}
	sku := Sku{
		Code: req.Code, Category: req.Category, Name: req.Name, Description: req.Description,
		UnitPriceCents: req.UnitPriceCents, Unit: req.Unit, Listed: listed, Sort: req.Sort,
	}
	if err := s.repo.createSku(&sku); err != nil {
		return nil, err
	}
	return &sku, nil
}

// UpdateSku 更新 SKU 可变字段（category/code 不可改）。
func (s *catalogServiceImpl) UpdateSku(id int, req *SkuUpdate) (*Sku, error) {
	if _, err := s.GetSku(id); err != nil {
		return nil, err
	}
	fields := map[string]interface{}{}
	if req.Name != "" {
		fields["name"] = req.Name
	}
	if req.Description != "" {
		fields["description"] = req.Description
	}
	if req.UnitPriceCents != nil {
		if *req.UnitPriceCents < 0 {
			return nil, apperr.Validation("单价不能为负")
		}
		fields["unit_price_cents"] = *req.UnitPriceCents
	}
	if req.Unit != "" {
		fields["unit"] = req.Unit
	}
	if req.Listed != nil {
		fields["listed"] = *req.Listed
	}
	if req.Sort != nil {
		fields["sort"] = *req.Sort
	}
	if len(fields) > 0 {
		if err := s.repo.updateSku(id, fields); err != nil {
			return nil, err
		}
	}
	return s.GetSku(id)
}

// DeleteSku 删除 SKU 及其折扣阶梯。
func (s *catalogServiceImpl) DeleteSku(id int) error {
	if _, err := s.GetSku(id); err != nil {
		return err
	}
	tiers, err := s.repo.listTiersBySku(id)
	if err != nil {
		return err
	}
	for _, t := range tiers {
		if err := s.repo.deleteTier(int(t.ID)); err != nil {
			return err
		}
	}
	return s.repo.deleteSku(id)
}

// GetSku 取 SKU。
func (s *catalogServiceImpl) GetSku(id int) (*Sku, error) {
	sku, err := s.repo.getSku(id)
	if err != nil {
		if isNotFound(err) {
			return nil, apperr.NotFound("商品不存在")
		}
		return nil, err
	}
	return sku, nil
}

// ListSkus 列出 SKU（includeUnlisted=true 含下架，供运营）。
func (s *catalogServiceImpl) ListSkus(includeUnlisted bool) ([]Sku, error) {
	return s.repo.listSkus(!includeUnlisted)
}

// ListListedSkus 前台：上架 SKU + 其折扣阶梯。
func (s *catalogServiceImpl) ListListedSkus() ([]SkuWithTiers, error) {
	skus, err := s.repo.listSkus(true)
	if err != nil {
		return nil, err
	}
	out := make([]SkuWithTiers, 0, len(skus))
	for _, sku := range skus {
		tiers, err := s.repo.listTiersBySku(int(sku.ID))
		if err != nil {
			return nil, err
		}
		out = append(out, SkuWithTiers{Sku: sku, Tiers: tiers})
	}
	return out, nil
}

// CreateTier 给 SKU 加折扣阶梯。
func (s *catalogServiceImpl) CreateTier(skuID int, req *TierCreate) (*DiscountTier, error) {
	if _, err := s.GetSku(skuID); err != nil {
		return nil, err
	}
	if req.MinQuantity < 1 {
		return nil, apperr.Validation("数量门槛必须≥1")
	}
	if !validBps(req.DiscountBps) {
		return nil, apperr.Validation("折扣基点须在 0~10000")
	}
	if req.CycleMonths < 0 {
		return nil, apperr.Validation("周期月数不能为负")
	}
	tier := DiscountTier{SkuID: uint(skuID), CycleMonths: req.CycleMonths, MinQuantity: req.MinQuantity, DiscountBps: req.DiscountBps}
	if err := s.repo.createTier(&tier); err != nil {
		return nil, err
	}
	return &tier, nil
}

// UpdateTier 更新折扣阶梯。
func (s *catalogServiceImpl) UpdateTier(id int, req *TierUpdate) (*DiscountTier, error) {
	tier, err := s.repo.getTier(id)
	if err != nil {
		if isNotFound(err) {
			return nil, apperr.NotFound("折扣阶梯不存在")
		}
		return nil, err
	}
	fields := map[string]interface{}{}
	if req.CycleMonths != nil {
		if *req.CycleMonths < 0 {
			return nil, apperr.Validation("周期月数不能为负")
		}
		fields["cycle_months"] = *req.CycleMonths
	}
	if req.MinQuantity != nil {
		if *req.MinQuantity < 1 {
			return nil, apperr.Validation("数量门槛必须≥1")
		}
		fields["min_quantity"] = *req.MinQuantity
	}
	if req.DiscountBps != nil {
		if !validBps(*req.DiscountBps) {
			return nil, apperr.Validation("折扣基点须在 0~10000")
		}
		fields["discount_bps"] = *req.DiscountBps
	}
	if len(fields) > 0 {
		if err := s.repo.updateTier(id, fields); err != nil {
			return nil, err
		}
	}
	t, err := s.repo.getTier(id)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// DeleteTier 删除折扣阶梯。
func (s *catalogServiceImpl) DeleteTier(id int) error {
	if _, err := s.repo.getTier(id); err != nil {
		if isNotFound(err) {
			return apperr.NotFound("折扣阶梯不存在")
		}
		return err
	}
	return s.repo.deleteTier(id)
}

// ListTiers 列出某 SKU 的折扣阶梯。
func (s *catalogServiceImpl) ListTiers(skuID int) ([]DiscountTier, error) {
	if _, err := s.GetSku(skuID); err != nil {
		return nil, err
	}
	return s.repo.listTiersBySku(skuID)
}
```

> 注：`SeedCatalog` 已在 Task 1 的 `catalog_service.go` 中，import 行需合并（`gorm.io/gorm` 已用；新增 `manager-backend/framework/apperr`）。`createSku` 的 code 唯一性由服务层先查重保证；DB 唯一索引是兜底。

- [ ] **Step 5：在 `module.go` 的 `Init` 装配 `CatalogService`**

```go
func (m *billingModule) Init(db *gorm.DB) error {
	BillingService = newService(newRepository(db))
	CatalogService = newCatalogService(newCatalogRepository(db))
	return nil
}
```

- [ ] **Step 6：跑测试 → PASS（全部 billing 测试）**

`cd /home/root/workspace005/gloryphone-code/backend && go test ./modules/billing/internal/ -v`

- [ ] **Step 7：质量门 + 提交**

```bash
cd /home/root/workspace005/gloryphone-code/backend && gofmt -l modules/billing/ && go vet ./modules/billing/... && go build ./...
cd /home/root/workspace005/gloryphone-code
git add backend/modules/billing/internal
git commit -m "feat(billing): 商品目录仓储 + SKU/折扣阶梯 CRUD 服务

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 3：计价引擎 Quote

**Files:** Modify `catalog_service.go`(加 `Quote`); Test 追加。

- [ ] **Step 1：写失败测试（追加）**

```go
func TestQuotePricing(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_skus", "billing_discount_tiers") })
	require.NoError(t, SeedCatalog(framework.DB))

	// 实例费：3000 分/台/月，年付(12月)7折，5 台 → 3000*12*5=180000，*0.7=126000
	q, err := CatalogService.Quote("instance_fee", 12, 5)
	require.NoError(t, err)
	assert.Equal(t, int64(180000), q.OriginalCents)
	assert.Equal(t, 7000, q.DiscountBps)
	assert.Equal(t, int64(126000), q.PayableCents)
	assert.Equal(t, 60, q.BillingUnits) // 12*5

	// 月付无折扣：3000*1*2=6000
	q, err = CatalogService.Quote("instance_fee", 1, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(6000), q.OriginalCents)
	assert.Equal(t, 10000, q.DiscountBps)
	assert.Equal(t, int64(6000), q.PayableCents)

	// 时长包：20 分/小时，1000 小时 8 折 → 20*1000=20000，*0.8=16000
	q, err = CatalogService.Quote("time_pack", 0, 1000)
	require.NoError(t, err)
	assert.Equal(t, int64(20000), q.OriginalCents)
	assert.Equal(t, 8000, q.DiscountBps)
	assert.Equal(t, int64(16000), q.PayableCents)

	// 时长包 300 小时 → 命中 ≥1 档（无折扣）：20*300=6000
	q, err = CatalogService.Quote("time_pack", 0, 300)
	require.NoError(t, err)
	assert.Equal(t, 10000, q.DiscountBps)
	assert.Equal(t, int64(6000), q.PayableCents)
}

func TestQuoteGuards(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("billing_skus", "billing_discount_tiers") })
	require.NoError(t, SeedCatalog(framework.DB))

	_, err := CatalogService.Quote("nope", 1, 1)
	assert.Error(t, err) // 不存在
	_, err = CatalogService.Quote("instance_fee", 1, 0)
	assert.Error(t, err) // 数量<1
	_, err = CatalogService.Quote("instance_fee", 0, 1)
	assert.Error(t, err) // 订阅类必须 cycleMonths>0
	_, err = CatalogService.Quote("time_pack", 1, 100)
	assert.Error(t, err) // 时长包必须 cycleMonths==0
}
```

- [ ] **Step 2：跑测试 → FAIL（Quote 未定义）**

`cd /home/root/workspace005/gloryphone-code/backend && go test ./modules/billing/internal/ -run 'TestQuotePricing|TestQuoteGuards' -v`

- [ ] **Step 3：在 `catalog_service.go` 实现 `Quote`**

```go
// Quote 服务端权威计价。订阅类(instance_fee/boot_pack)：cycleMonths∈{>0}，quantity=台数；
// 时长包(time_pack)：cycleMonths==0，quantity=小时数。
func (s *catalogServiceImpl) Quote(skuCode string, cycleMonths, quantity int) (*QuoteResult, error) {
	sku, err := s.repo.getSkuByCode(skuCode)
	if err != nil {
		if isNotFound(err) {
			return nil, apperr.NotFound("商品不存在")
		}
		return nil, err
	}
	if quantity < 1 {
		return nil, apperr.Validation("数量必须≥1")
	}
	if sku.Category == CategoryTimePack {
		if cycleMonths != 0 {
			return nil, apperr.Validation("时长包的周期月数必须为0")
		}
	} else if cycleMonths < 1 {
		return nil, apperr.Validation("订阅类商品周期月数必须≥1")
	}

	var billingUnits int
	if sku.Category == CategoryTimePack {
		billingUnits = quantity
	} else {
		billingUnits = cycleMonths * quantity
	}
	originalCents := sku.UnitPriceCents * int64(billingUnits)

	// 折扣：同周期、min_quantity ≤ quantity 中取 min_quantity 最大者；无则全价。
	bps := DiscountBpsFull
	tiers, err := s.repo.listTiersBySkuCycle(int(sku.ID), cycleMonths)
	if err != nil {
		return nil, err
	}
	bestMin := -1
	for _, t := range tiers {
		if t.MinQuantity <= quantity && t.MinQuantity > bestMin {
			bestMin = t.MinQuantity
			bps = t.DiscountBps
		}
	}

	payableCents := (originalCents*int64(bps) + int64(DiscountBpsFull)/2) / int64(DiscountBpsFull) // 四舍五入

	return &QuoteResult{
		SkuCode: sku.Code, Category: sku.Category, CycleMonths: cycleMonths, Quantity: quantity,
		UnitPriceCents: sku.UnitPriceCents, BillingUnits: billingUnits,
		OriginalCents: originalCents, DiscountBps: bps, PayableCents: payableCents,
	}, nil
}
```

- [ ] **Step 4：跑测试 → PASS（全部 billing 测试）**

`cd /home/root/workspace005/gloryphone-code/backend && go test ./modules/billing/internal/ -v`

- [ ] **Step 5：质量门 + 提交**

```bash
cd /home/root/workspace005/gloryphone-code/backend && gofmt -l modules/billing/ && go vet ./modules/billing/... && go build ./...
cd /home/root/workspace005/gloryphone-code
git add backend/modules/billing/internal
git commit -m "feat(billing): 计价引擎 Quote(周期×数量/时长档 折扣阶梯解析)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 4：前台 API（SKU 列表 / 计价）+ 路由

> handler 为薄转发；以 build+vet 验证装配（沿用 Plan 1 约定）。

**Files:** Create `catalog_api.go`; Modify `module.go`(前台路由)。

- [ ] **Step 1：创建 `catalog_api.go`**

```go
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

// adminSkuIDParam 解析 :id 路径参数（后台 SKU 用，Task 5 复用）
func adminSkuIDParam(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return 0, false
	}
	return id, true
}
```

- [ ] **Step 2：在 `module.go` 前台 `/billing` 组内加路由**

在前台组 `{ ... }` 内（topup 之后）追加：
```go
		g.GET("/skus", ListSkus)
		g.POST("/quote", Quote)
```

- [ ] **Step 3：验证（paste 输出）**

`cd /home/root/workspace005/gloryphone-code/backend && gofmt -l modules/billing/ && go vet ./modules/billing/... && go build ./... && go test ./modules/billing/internal/ -v`

- [ ] **Step 4：提交**

```bash
cd /home/root/workspace005/gloryphone-code
git add backend/modules/billing/internal
git commit -m "feat(billing): 前台 API 商品列表/计价 + 路由

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 5：后台 API（SKU/折扣阶梯 CRUD）+ 路由

**Files:** Modify `catalog_api.go`(后台 handler), `module.go`(后台路由)。

- [ ] **Step 1：在 `catalog_api.go` 追加后台 handler**

```go
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
```

- [ ] **Step 2：在 `module.go` 后台 `/admin/billing` 组内加路由**

在后台组 `{ ... }`（accounts 路由之后）追加：
```go
		admin.GET("/skus", staff.PermissionMiddleware("billing:view"), AdminListSkus)
		admin.POST("/skus", staff.PermissionMiddleware("billing:manage"), AdminCreateSku)
		admin.PUT("/skus/:id", staff.PermissionMiddleware("billing:manage"), AdminUpdateSku)
		admin.DELETE("/skus/:id", staff.PermissionMiddleware("billing:manage"), AdminDeleteSku)
		admin.GET("/skus/:id/tiers", staff.PermissionMiddleware("billing:view"), AdminListTiers)
		admin.POST("/skus/:id/tiers", staff.PermissionMiddleware("billing:manage"), AdminCreateTier)
		admin.PUT("/tiers/:tierId", staff.PermissionMiddleware("billing:manage"), AdminUpdateTier)
		admin.DELETE("/tiers/:tierId", staff.PermissionMiddleware("billing:manage"), AdminDeleteTier)
```

> 注意：Gin 同一路由组内，`/accounts/:userId` 与 `/skus/:id` 的参数名不同（`userId` vs `id`），在不同路径段（`accounts` vs `skus`），不冲突。`/skus/:id/...` 与 `/tiers/:tierId` 也分属不同前缀，无通配冲突。

- [ ] **Step 3：全量回归 + 质量门（paste 输出）**

```bash
cd /home/root/workspace005/gloryphone-code/backend
gofmt -l modules/billing/
go vet ./...
go test ./... 2>&1 | tail -20
go test ./framework/ -run TestModuleBoundaries -v 2>&1 | tail -5
go build ./...
```
Expect 全 ok、边界 PASS。

- [ ] **Step 4：提交**

```bash
cd /home/root/workspace005/gloryphone-code
git add backend/modules/billing/internal
git commit -m "feat(billing): 后台 API SKU/折扣阶梯 CRUD + 路由

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## 计划 2 验收对照（PRD §6）

| 需求 | 覆盖 |
|---|---|
| SKU（实例费/包月开机包/时长包）三类 + 单价 + 上下架 | Task 1/2 |
| 规格（周期 月/季/年）× 数量 → 阶梯折扣 | Task 1（模型）/Task 3（解析计价） |
| 时长档折扣 | Task 1 seed + Task 3 |
| 价格服务端权威计算（前端不可信） | Task 3 `Quote` + Task 4 `/billing/quote` |
| 运营定价/折扣/上下架管理 | Task 5 后台 CRUD |
| 优惠码/代金券 | 不做（Non-goal，PRD §6/§15） |

> 价格快照入订单项在**计划 4（订单/收银台）**：下单时调 `Quote` 取 `PayableCents` 落快照。

## Self-Review 记录

- **Spec 覆盖**：覆盖 PRD §6 全部条目（见上表）；优惠码明确不做。
- **占位扫描**：无 TODO/占位；每步给出完整代码与命令。Task 1 的 `catalog_service.go` 先放 `SeedCatalog`，Task 2 在同文件补服务体并合并 import——已显式说明，非遗留占位。
- **类型一致**：`Sku`/`DiscountTier`/`SkuCreate`/`SkuUpdate`/`TierCreate`/`TierUpdate`/`SkuWithTiers`/`QuoteResult` 字段贯穿一致；`CatalogService` 方法签名 `CreateSku/UpdateSku/DeleteSku/GetSku/ListSkus/ListListedSkus/CreateTier(skuID,...)/UpdateTier(id,...)/DeleteTier/ListTiers/Quote(skuCode,cycleMonths,quantity)` 在测试与 API 引用一致；`DiscountBps` 语义（10000=全价）统一；金额整数分。
- **金额/折扣**：整数分 + bps；四舍五入 `(x*bps + 5000)/10000`。
