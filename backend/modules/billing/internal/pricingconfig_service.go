package billing

import (
	"manager-backend/framework/apperr"
)

// pricingConfigServiceImpl 定价配置服务：admin 读写 + 用户端报价计价的配置来源。
type pricingConfigServiceImpl struct{ repo pricingConfigRepository }

// PricingConfigService 模块内单例，由 module.Init 注入。
var PricingConfigService *pricingConfigServiceImpl

func newPricingConfigService(repo pricingConfigRepository) *pricingConfigServiceImpl {
	return &pricingConfigServiceImpl{repo: repo}
}

// Get 读取完整定价配置（不存在时返回默认值，便于测试环境无 seed 也可用）。
func (s *pricingConfigServiceImpl) Get() (*PricingConfigData, error) {
	d, err := s.repo.load()
	if err != nil {
		// 配置行缺失：回退默认值（不持久化）。
		return defaultPricingConfig(), nil
	}
	if d.Kinds == nil || len(d.Kinds) == 0 {
		return defaultPricingConfig(), nil
	}
	return d, nil
}

// Save 覆盖保存完整定价配置。
func (s *pricingConfigServiceImpl) Save(data *PricingConfigData) error {
	if data == nil {
		return apperr.Validation("配置不能为空")
	}
	return s.repo.save(data)
}

// SavePartial 仅更新部分字段，其余沿用现有配置（admin 各页面独立保存用）。
func (s *pricingConfigServiceImpl) SavePartial(mutate func(*PricingConfigData)) (*PricingConfigData, error) {
	cur, err := s.Get()
	if err != nil {
		return nil, err
	}
	mutate(cur)
	if err := s.repo.save(cur); err != nil {
		return nil, err
	}
	return cur, nil
}

// priceConfigFor 把某 kind 的持久化配置翻译成计价引擎 PriceConfig。
func (s *pricingConfigServiceImpl) priceConfigFor(kind string) (PriceConfig, error) {
	d, err := s.Get()
	if err != nil {
		return PriceConfig{}, err
	}
	kp, ok := d.Kinds[kind]
	if !ok {
		return PriceConfig{}, apperr.Validation("未配置该资源定价")
	}
	cfg := PriceConfig{UnitPriceCents: kp.UnitPriceCents}
	for _, t := range kp.QtyTiers {
		cfg.QtyTiers = append(cfg.QtyTiers, QtyTier{MinQuantity: t.MinQuantity, DiscountBps: t.DiscountBps})
	}
	for _, o := range kp.DurationOptions {
		cfg.DurationOpts = append(cfg.DurationOpts, DurationOption{Value: o.Value, DiscountBps: o.DiscountBps})
	}
	return cfg, nil
}

// QuoteKindDuration 报价：seat/boot_slot 新购或续费（按数量 + 时长）。
func (s *pricingConfigServiceImpl) QuoteKindDuration(kind string, quantity, durationValue int) (PriceQuote, error) {
	cfg, err := s.priceConfigFor(kind)
	if err != nil {
		return PriceQuote{}, err
	}
	return computeQuote(cfg, quantity, durationValue)
}

// QuoteRuntimePack 报价：临时时长包（按分钟数 × 每分钟单价，命中包预设享折扣）。
func (s *pricingConfigServiceImpl) QuoteRuntimePack(minutes int) (PriceQuote, error) {
	d, err := s.Get()
	if err != nil {
		return PriceQuote{}, err
	}
	rt := d.Runtime
	if minutes < rt.MinMinutes {
		return PriceQuote{}, apperr.Validation("低于最低购买时长")
	}
	bps := DiscountBpsFull
	for _, p := range rt.Packs {
		if p.Minutes == minutes {
			bps = p.DiscountBps
			break
		}
	}
	original := rt.UnitPriceCentsPerMinute * int64(minutes)
	payable := applyBps(original, bps)
	return PriceQuote{
		Quantity:            minutes,
		DurationValue:       1,
		UnitPriceCents:      rt.UnitPriceCentsPerMinute,
		BillingUnits:        minutes,
		OriginalCents:       original,
		QtyDiscountBps:      bps,
		DurationDiscountBps: DiscountBpsFull,
		PayableCents:        payable,
	}, nil
}

// DailyCapMinutes 当天每台封顶值（admin 可配）。
func (s *pricingConfigServiceImpl) DailyCapMinutes() (int, error) {
	d, err := s.Get()
	if err != nil {
		return 0, err
	}
	if d.Runtime.DailyCapMinutes <= 0 {
		return 200, nil
	}
	return d.Runtime.DailyCapMinutes, nil
}

// RecycleRetentionDays 回收站保留天数（admin 可配，默认 30）。
func (s *pricingConfigServiceImpl) RecycleRetentionDays() (int, error) {
	d, err := s.Get()
	if err != nil {
		return 0, err
	}
	if d.Runtime.RecycleRetentionDays <= 0 {
		return 30, nil
	}
	return d.Runtime.RecycleRetentionDays, nil
}
