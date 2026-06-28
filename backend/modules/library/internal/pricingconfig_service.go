package library

import "manager-backend/framework/apperr"

// pricingConfigServiceImpl 素材库定价配置服务：admin 读写 + 计价的配置来源。
type pricingConfigServiceImpl struct{ repo pricingConfigRepository }

// PricingConfigService 模块内单例，由 module.Init 注入。
var PricingConfigService *pricingConfigServiceImpl

func newPricingConfigService(repo pricingConfigRepository) *pricingConfigServiceImpl {
	return &pricingConfigServiceImpl{repo: repo}
}

// Get 读取完整定价配置（缺失或为空时回退默认值，便于测试环境无 seed 也可用）。
func (s *pricingConfigServiceImpl) Get() (*LibraryPricingConfigData, error) {
	d, err := s.repo.load()
	if err != nil {
		return defaultPricingConfig(), nil
	}
	if d.FreeQuotaBytes <= 0 && len(d.Tiers) == 0 {
		return defaultPricingConfig(), nil
	}
	return d, nil
}

// Save 覆盖保存完整定价配置。
func (s *pricingConfigServiceImpl) Save(data *LibraryPricingConfigData) error {
	if data == nil {
		return apperr.Validation("配置不能为空")
	}
	if data.FreeQuotaBytes < 0 {
		return apperr.Validation("免费额度不能为负")
	}
	return s.repo.save(data)
}

// FreeQuotaBytes 免费容量额度（字节）；配置缺失回退 5 GiB。
func (s *pricingConfigServiceImpl) FreeQuotaBytes() (int64, error) {
	d, err := s.Get()
	if err != nil {
		return 0, err
	}
	if d.FreeQuotaBytes <= 0 {
		return 5 * GiB, nil
	}
	return d.FreeQuotaBytes, nil
}
