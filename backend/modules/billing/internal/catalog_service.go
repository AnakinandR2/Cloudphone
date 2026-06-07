package billing

import "gorm.io/gorm"

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
