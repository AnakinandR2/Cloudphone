package billing

import (
	"manager-backend/framework/apperr"

	"gorm.io/gorm"
)

// SeedCatalog 幂等写入三类默认 SKU 及其折扣阶梯（按 code 查重，存在即跳过该 SKU）。
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
			continue
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

func (s *catalogServiceImpl) ListSkus(includeUnlisted bool) ([]Sku, error) {
	return s.repo.listSkus(!includeUnlisted)
}

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

func (s *catalogServiceImpl) UpdateTier(id int, req *TierUpdate) (*DiscountTier, error) {
	if _, err := s.repo.getTier(id); err != nil {
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

func (s *catalogServiceImpl) DeleteTier(id int) error {
	if _, err := s.repo.getTier(id); err != nil {
		if isNotFound(err) {
			return apperr.NotFound("折扣阶梯不存在")
		}
		return err
	}
	return s.repo.deleteTier(id)
}

func (s *catalogServiceImpl) ListTiers(skuID int) ([]DiscountTier, error) {
	if _, err := s.GetSku(skuID); err != nil {
		return nil, err
	}
	return s.repo.listTiersBySku(skuID)
}

// Quote 服务端权威计价。订阅类(instance_fee/boot_pack)：cycleMonths>0，quantity=台数；
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

	payableCents := (originalCents*int64(bps) + int64(DiscountBpsFull)/2) / int64(DiscountBpsFull)

	return &QuoteResult{
		SkuCode: sku.Code, Category: sku.Category, CycleMonths: cycleMonths, Quantity: quantity,
		UnitPriceCents: sku.UnitPriceCents, BillingUnits: billingUnits,
		OriginalCents: originalCents, DiscountBps: bps, PayableCents: payableCents,
	}, nil
}
