package library

import (
	"encoding/json"

	"gorm.io/gorm"
)

// pricingConfigRepository 定价配置读写（单行 JSON）。
type pricingConfigRepository interface {
	load() (*LibraryPricingConfigData, error)
	save(data *LibraryPricingConfigData) error
}

type gormPricingConfigRepository struct{ db *gorm.DB }

func newPricingConfigRepository(db *gorm.DB) pricingConfigRepository {
	return &gormPricingConfigRepository{db: db}
}

func (r *gormPricingConfigRepository) load() (*LibraryPricingConfigData, error) {
	var row LibraryPricingConfig
	if err := r.db.Where(LibraryPricingConfig{ID: 1}).First(&row).Error; err != nil {
		return nil, err
	}
	return decodePricingConfig(&row)
}

func (r *gormPricingConfigRepository) save(data *LibraryPricingConfigData) error {
	row, err := encodePricingConfig(data)
	if err != nil {
		return err
	}
	row.ID = 1
	return r.db.Save(row).Error
}

func decodePricingConfig(row *LibraryPricingConfig) (*LibraryPricingConfigData, error) {
	d := &LibraryPricingConfigData{
		FreeQuotaBytes: row.FreeQuotaBytes,
		Notice:         row.Notice,
		BillingNote:    row.BillingNote,
	}
	if row.TiersJSON != "" {
		if err := json.Unmarshal([]byte(row.TiersJSON), &d.Tiers); err != nil {
			return nil, err
		}
	}
	if row.CustomTierJSON != "" {
		if err := json.Unmarshal([]byte(row.CustomTierJSON), &d.CustomTier); err != nil {
			return nil, err
		}
	}
	if row.DurationsJSON != "" {
		if err := json.Unmarshal([]byte(row.DurationsJSON), &d.DurationOptions); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func encodePricingConfig(data *LibraryPricingConfigData) (*LibraryPricingConfig, error) {
	tiers, err := json.Marshal(data.Tiers)
	if err != nil {
		return nil, err
	}
	custom, err := json.Marshal(data.CustomTier)
	if err != nil {
		return nil, err
	}
	durs, err := json.Marshal(data.DurationOptions)
	if err != nil {
		return nil, err
	}
	return &LibraryPricingConfig{
		FreeQuotaBytes: data.FreeQuotaBytes,
		TiersJSON:      string(tiers),
		CustomTierJSON: string(custom),
		DurationsJSON:  string(durs),
		Notice:         data.Notice,
		BillingNote:    data.BillingNote,
	}, nil
}

// seedLibraryPricingConfig 幂等写入默认定价配置（仅当配置行不存在时）。
func seedLibraryPricingConfig(db *gorm.DB) error {
	var cnt int64
	if err := db.Model(&LibraryPricingConfig{}).Where("id = ?", 1).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt > 0 {
		return nil
	}
	row, err := encodePricingConfig(defaultPricingConfig())
	if err != nil {
		return err
	}
	row.ID = 1
	return db.Create(row).Error
}

// defaultPricingConfig 默认值（§10 的具体数值），seed 用。
func defaultPricingConfig() *LibraryPricingConfigData {
	return &LibraryPricingConfigData{
		FreeQuotaBytes: 5 * GiB, // 免费 5 GiB
		Tiers: []TierCfg{
			{Code: "t50", Name: "50GB 套餐", CapacityGB: 50, MonthlyPriceCents: 1000, TierDiscountBps: 10000, Enabled: true, Sort: 1},
			{Code: "t100", Name: "100GB 套餐", CapacityGB: 100, MonthlyPriceCents: 1800, TierDiscountBps: 10000, Enabled: true, Sort: 2},
			{Code: "t500", Name: "500GB 套餐", CapacityGB: 500, MonthlyPriceCents: 8000, TierDiscountBps: 10000, Enabled: true, Sort: 3},
		},
		CustomTier: CustomTierCfg{
			Enabled:              true,
			MinCapacityGB:        50,
			PricePerGBMonthCents: 20, // ¥0.2/GB/月
			TierDiscountBps:      10000,
		},
		DurationOptions: []DurationOptCfg{
			{Days: 30, DiscountBps: 10000},
			{Days: 90, DiscountBps: 9500},
			{Days: 180, DiscountBps: 9000},
			{Days: 365, DiscountBps: 8000},
		},
		Notice:      "套餐过期后将降级为免费套餐；若已用容量超过免费额度，文件仍保留但只能查看或删除，无法继续上传或取用，请及时扩容或清理。",
		BillingNote: "容量套餐按所选时长一次性计费；升级按剩余天数补差价、到期时间不变；降级立即生效、不可选时长、已付费用不退。文件暂不做去重合并，每次上传均独立计入容量。",
	}
}
