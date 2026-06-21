package partner

import (
	"manager-backend/framework/apperr"
	"manager-backend/framework/query"
)

// serviceImpl 合作商业务服务。合作商是运营全局内容，无属主隔离；
// 点击上报对匿名/脏数据宽容，不阻断前端跳转。
type serviceImpl struct {
	repo repository
}

// PartnerService 模块内服务实例，由 module.Init 注入 DB 后装配。
var PartnerService *serviceImpl

func newService(repo repository) *serviceImpl { return &serviceImpl{repo: repo} }

// orderable admin 列表允许排序的字段白名单（杜绝 SQL 注入）。
var orderable = map[string]bool{"id": true, "name": true, "sort": true, "click_count": true, "created_at": true}

// --- admin（运营，全量）---

func (s *serviceImpl) AdminList(page, size int, kw, order, sort string) ([]Partner, int64, error) {
	total, err := s.repo.count(kw, false)
	if err != nil {
		return nil, 0, err
	}
	orderClause := query.SafeOrder(order, sort, orderable, "sort ASC, id ASC")
	items, err := s.repo.list((page-1)*size, size, kw, false, orderClause)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *serviceImpl) GetByID(id int) (*Partner, error) {
	item, err := s.repo.findByID(id)
	if err != nil {
		return nil, apperr.NotFound("合作商不存在")
	}
	return item, nil
}

func (s *serviceImpl) Create(req *PartnerCreate) (*Partner, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	item := Partner{
		Name:     req.Name,
		LogoURL:  req.LogoURL,
		ImageURL: req.ImageURL,
		Intro:    req.Intro,
		PromoURL: req.PromoURL,
		Sort:     req.Sort,
		Enabled:  enabled,
	}
	if err := s.repo.create(&item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *serviceImpl) Update(id int, req *PartnerUpdate) (*Partner, error) {
	if _, err := s.GetByID(id); err != nil {
		return nil, err
	}
	fields := map[string]interface{}{}
	if req.Name != "" {
		fields["name"] = req.Name
	}
	if req.PromoURL != "" {
		fields["promo_url"] = req.PromoURL
	}
	if req.LogoURL != nil {
		fields["logo_url"] = *req.LogoURL
	}
	if req.ImageURL != nil {
		fields["image_url"] = *req.ImageURL
	}
	if req.Intro != nil {
		fields["intro"] = *req.Intro
	}
	if req.Sort != nil {
		fields["sort"] = *req.Sort
	}
	if req.Enabled != nil {
		fields["enabled"] = *req.Enabled
	}
	if len(fields) > 0 {
		if err := s.repo.update(id, fields); err != nil {
			return nil, err
		}
	}
	return s.GetByID(id)
}

func (s *serviceImpl) Delete(id int) error {
	if _, err := s.GetByID(id); err != nil {
		return err
	}
	return s.repo.delete(id)
}

// ListClicks 点击明细分页（admin 查看趋势/明细）。
func (s *serviceImpl) ListClicks(partnerID, page, size int) ([]PartnerClick, int64, error) {
	total, err := s.repo.countClicks(partnerID)
	if err != nil {
		return nil, 0, err
	}
	items, err := s.repo.listClicks(partnerID, (page-1)*size, size)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// --- 公开（my/www）---

// ListEnabled 启用合作商列表（精简视图），按 sort、id 排序。
func (s *serviceImpl) ListEnabled() ([]PublicPartner, error) {
	items, err := s.repo.listEnabled()
	if err != nil {
		return nil, err
	}
	out := make([]PublicPartner, 0, len(items))
	for _, p := range items {
		out = append(out, toPublic(p))
	}
	return out, nil
}

// RecordClick 记录一次点击：落明细 + 原子自增 click_count。
// 对脏数据宽容：合作商不存在/已禁用则跳过写入但不报错（不阻断前端跳转）。
// 返回是否真正记录（false=被跳过）。
func (s *serviceImpl) RecordClick(partnerID int, click *PartnerClick) (bool, error) {
	p, err := s.repo.findByID(partnerID)
	if err != nil || p == nil || !p.Enabled {
		return false, nil // 静默跳过，调用方仍返回成功语义
	}
	click.PartnerID = uint(partnerID)
	if err := s.repo.createClick(click); err != nil {
		return false, err
	}
	if err := s.repo.incrClick(partnerID); err != nil {
		return false, err
	}
	return true, nil
}
