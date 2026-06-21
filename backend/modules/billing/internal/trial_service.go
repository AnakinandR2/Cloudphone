package billing

import (
	"manager-backend/framework/apperr"
)

type trialServiceImpl struct{ repo trialRepository }

var TrialService *trialServiceImpl

func newTrialService(repo trialRepository) *trialServiceImpl { return &trialServiceImpl{repo: repo} }

func (s *trialServiceImpl) eligible(userID int, p *TrialPolicy) (bool, bool, int, string) {
	claimed, _ := s.repo.countClaims(int(p.ID), userID)
	if claimed >= p.PerUserLimit {
		return false, false, claimed, "已达领取上限"
	}
	if p.AllowNewUser {
		if paid, _ := s.repo.countPaidOrders(userID); paid == 0 {
			return true, false, claimed, ""
		}
	}
	if ok, _ := s.repo.manualEligible(int(p.ID), userID); ok {
		return true, false, claimed, ""
	}
	if p.InviteCode != "" {
		return false, true, claimed, "需邀请码"
	}
	return false, false, claimed, "不符合领取条件"
}

func (s *trialServiceImpl) ListClaimable(userID int) ([]ClaimableItem, error) {
	policies, err := s.repo.listPolicies(true)
	if err != nil {
		return nil, err
	}
	out := make([]ClaimableItem, 0, len(policies))
	for i := range policies {
		ok, needInvite, claimed, reason := s.eligible(userID, &policies[i])
		pub := policies[i]
		pub.InviteCode = "" // 不向前台泄露邀请码（NeedInvite 已提示需要输入）
		out = append(out, ClaimableItem{Policy: pub, Claimable: ok, NeedInvite: needInvite, ClaimedCount: claimed, Reason: reason})
	}
	return out, nil
}

// trialGrantSubjects 合法的试用发放科目（新模型命名）：seat / boot_slot 授权单元、runtime_minute 时长。
var trialGrantSubjects = map[string]bool{KindSeat: true, KindBootSlot: true, SubjectRuntimeMinute: true}

// validateItems 校验发放项：≥1 项、科目合法且不重复、数量>0、有效天数≥0。
func validateItems(in []TrialPolicyItemInput) ([]TrialPolicyItem, error) {
	if len(in) == 0 {
		return nil, apperr.Validation("至少配置一项发放")
	}
	seen := map[string]bool{}
	out := make([]TrialPolicyItem, 0, len(in))
	for _, it := range in {
		if !trialGrantSubjects[it.Subject] {
			return nil, apperr.Validation("试用只能发放资源科目")
		}
		if seen[it.Subject] {
			return nil, apperr.Validation("发放科目重复")
		}
		seen[it.Subject] = true
		if it.Quantity <= 0 {
			return nil, apperr.Validation("发放数量必须大于0")
		}
		if it.ExpireDays < 0 {
			return nil, apperr.Validation("有效天数不能为负")
		}
		out = append(out, TrialPolicyItem{Subject: it.Subject, Quantity: it.Quantity, ExpireDays: it.ExpireDays})
	}
	return out, nil
}

func (s *trialServiceImpl) CreatePolicy(req *TrialPolicyCreate) (*TrialPolicy, error) {
	items, err := validateItems(req.Items)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.getPolicyByCode(req.Code); err == nil {
		return nil, apperr.Conflict("策略编码已存在")
	} else if !isNotFoundTrial(err) {
		return nil, err
	}
	limit := req.PerUserLimit
	if limit < 1 {
		limit = 1
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	p := TrialPolicy{
		Code: req.Code, Name: req.Name, Enabled: enabled, PerUserLimit: limit,
		AllowNewUser: req.AllowNewUser, InviteCode: req.InviteCode, Items: items,
	}
	if err := s.repo.createPolicy(&p); err != nil {
		return nil, err
	}
	return s.getPolicy(int(p.ID))
}

func (s *trialServiceImpl) UpdatePolicy(id int, req *TrialPolicyUpdate) (*TrialPolicy, error) {
	if _, err := s.getPolicy(id); err != nil {
		return nil, err
	}
	fields := map[string]interface{}{}
	if req.Name != "" {
		fields["name"] = req.Name
	}
	if req.PerUserLimit != nil {
		if *req.PerUserLimit < 1 {
			return nil, apperr.Validation("限领次数必须≥1")
		}
		fields["per_user_limit"] = *req.PerUserLimit
	}
	if req.AllowNewUser != nil {
		fields["allow_new_user"] = *req.AllowNewUser
	}
	if req.InviteCode != nil {
		fields["invite_code"] = *req.InviteCode
	}
	if req.Enabled != nil {
		fields["enabled"] = *req.Enabled
	}
	if len(fields) > 0 {
		if err := s.repo.updatePolicy(id, fields); err != nil {
			return nil, err
		}
	}
	if req.Items != nil {
		items, err := validateItems(*req.Items)
		if err != nil {
			return nil, err
		}
		if err := s.repo.replacePolicyItems(id, items); err != nil {
			return nil, err
		}
	}
	return s.getPolicy(id)
}

func (s *trialServiceImpl) getPolicy(id int) (*TrialPolicy, error) {
	p, err := s.repo.getPolicy(id)
	if err != nil {
		if isNotFoundTrial(err) {
			return nil, apperr.NotFound("试用策略不存在")
		}
		return nil, err
	}
	return p, nil
}

func (s *trialServiceImpl) DeletePolicy(id int) error {
	if _, err := s.getPolicy(id); err != nil {
		return err
	}
	return s.repo.deletePolicy(id)
}

func (s *trialServiceImpl) ListPolicies() ([]TrialPolicy, error) {
	return s.repo.listPolicies(false)
}

func (s *trialServiceImpl) GrantEligibility(policyID, userID int, operator string) error {
	if _, err := s.getPolicy(policyID); err != nil {
		return err
	}
	return s.repo.createEligibility(&TrialEligibility{PolicyID: uint(policyID), UserID: uint(userID), GrantedBy: operator})
}

func (s *trialServiceImpl) ListGrants(policyID int) ([]TrialGrant, error) {
	if _, err := s.getPolicy(policyID); err != nil {
		return nil, err
	}
	return s.repo.listGrants(policyID)
}

func (s *trialServiceImpl) ClaimTrial(userID int, code, inviteCode string) error {
	p, err := s.repo.getPolicyByCode(code)
	if err != nil {
		if isNotFoundTrial(err) {
			return apperr.NotFound("试用不存在")
		}
		return err
	}
	if !p.Enabled {
		return apperr.Validation("试用已停用")
	}
	claimed, err := s.repo.countClaims(int(p.ID), userID)
	if err != nil {
		return err
	}
	if claimed >= p.PerUserLimit {
		return apperr.Conflict("已达领取上限")
	}
	ok := false
	if p.AllowNewUser {
		if paid, _ := s.repo.countPaidOrders(userID); paid == 0 {
			ok = true
		}
	}
	if !ok {
		if m, _ := s.repo.manualEligible(int(p.ID), userID); m {
			ok = true
		}
	}
	if !ok && p.InviteCode != "" && inviteCode == p.InviteCode {
		ok = true
	}
	if !ok {
		return apperr.Validation("不符合领取条件")
	}
	return s.repo.claim(p, userID)
}
