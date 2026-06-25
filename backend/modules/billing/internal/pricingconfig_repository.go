package billing

import (
	"encoding/json"

	"gorm.io/gorm"
)

// pricingConfigRepository 定价配置读写（单行 JSON）。
type pricingConfigRepository interface {
	load() (*PricingConfigData, error)
	save(data *PricingConfigData) error
}

type gormPricingConfigRepository struct{ db *gorm.DB }

func newPricingConfigRepository(db *gorm.DB) pricingConfigRepository {
	return &gormPricingConfigRepository{db: db}
}

func (r *gormPricingConfigRepository) load() (*PricingConfigData, error) {
	var row PricingConfig
	if err := r.db.Where(PricingConfig{ID: 1}).First(&row).Error; err != nil {
		return nil, err
	}
	return decodePricingConfig(&row)
}

func (r *gormPricingConfigRepository) save(data *PricingConfigData) error {
	row, err := encodePricingConfig(data)
	if err != nil {
		return err
	}
	row.ID = 1
	return r.db.Save(row).Error
}

func decodePricingConfig(row *PricingConfig) (*PricingConfigData, error) {
	d := &PricingConfigData{Kinds: map[string]KindPricing{}}
	if row.PaymentMethodsJSON != "" {
		if err := json.Unmarshal([]byte(row.PaymentMethodsJSON), &d.PaymentMethods); err != nil {
			return nil, err
		}
	}
	if row.RechargePresetsJSON != "" {
		if err := json.Unmarshal([]byte(row.RechargePresetsJSON), &d.RechargePresets); err != nil {
			return nil, err
		}
	}
	if row.KindsJSON != "" {
		if err := json.Unmarshal([]byte(row.KindsJSON), &d.Kinds); err != nil {
			return nil, err
		}
	}
	if row.RuntimeJSON != "" {
		if err := json.Unmarshal([]byte(row.RuntimeJSON), &d.Runtime); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func encodePricingConfig(data *PricingConfigData) (*PricingConfig, error) {
	pm, err := json.Marshal(data.PaymentMethods)
	if err != nil {
		return nil, err
	}
	rp, err := json.Marshal(data.RechargePresets)
	if err != nil {
		return nil, err
	}
	ks, err := json.Marshal(data.Kinds)
	if err != nil {
		return nil, err
	}
	rt, err := json.Marshal(data.Runtime)
	if err != nil {
		return nil, err
	}
	return &PricingConfig{
		PaymentMethodsJSON:  string(pm),
		RechargePresetsJSON: string(rp),
		KindsJSON:           string(ks),
		RuntimeJSON:         string(rt),
	}, nil
}

// seedPricingConfig 幂等写入默认定价配置（仅当配置行不存在时）。
func seedPricingConfig(db *gorm.DB) error {
	var cnt int64
	if err := db.Model(&PricingConfig{}).Where("id = ?", 1).Count(&cnt).Error; err != nil {
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

// defaultPricingConfig 合理默认值（参考接口契约示例数值），seed 用。
func defaultPricingConfig() *PricingConfigData {
	return &PricingConfigData{
		PaymentMethods: []PaymentMethod{
			{Code: "balance", Name: "余额支付", Enabled: true, Sort: 0},
			{Code: "wechat", Name: "微信支付", Enabled: true, Sort: 1, LogoURL: "https://cdn.simpleicons.org/wechat/07C160"},
			{Code: "alipay", Name: "支付宝", Enabled: true, Sort: 2, LogoURL: "https://cdn.simpleicons.org/alipay/1677FF"},
		},
		RechargePresets: []int64{1000, 5000, 10000, 50000},
		Kinds: map[string]KindPricing{
			KindSeat: {
				UnitPriceCents: 3000,
				UnitLabel:      "台",
				QtyTiers: []QtyTierCfg{
					{MinQuantity: 10, DiscountBps: 9000},
					{MinQuantity: 100, DiscountBps: 8000},
				},
				DurationUnit: "month",
				DurationOptions: []DurationOptCfg{
					{Value: 1, DiscountBps: 10000},
					{Value: 3, DiscountBps: 8500},
					{Value: 12, DiscountBps: 7000},
				},
				Notice:               "云手机实例席位为固定套餐，决定可创建的实例数量。",
				BillingNote:          "席位按月计费，自购买起生效，到期后实例进入回收站。",
				RecycleRetentionDays: 30,
			},
			KindBootSlot: {
				UnitPriceCents: 2000,
				UnitLabel:      "个",
				QtyTiers: []QtyTierCfg{
					{MinQuantity: 10, DiscountBps: 9000},
				},
				DurationUnit: "day",
				DurationOptions: []DurationOptCfg{
					{Value: 7, DiscountBps: 10000},
					{Value: 30, DiscountBps: 9000},
				},
				Notice:      "包月开机数决定可同时开机运行的实例数，多实例轮流共享。",
				BillingNote: "包月开机数按天计费，占用名额的实例运行不扣临时时长。",
			},
		},
		Runtime: RuntimePackCfg{
			UnitPriceCentsPerMinute: 20,
			MinMinutes:              60,
			Packs: []RuntimePack{
				{Minutes: 600, DiscountBps: 10000},
				{Minutes: 3000, DiscountBps: 9000},
			},
			Notice:                  "临时开机时长无使用期限，用完为止；每台手机每天最多扣 200 分钟。",
			DailyCapMinutes:         200,
			GiftMinutesPerSeatMonth: 200,
		},
	}
}
