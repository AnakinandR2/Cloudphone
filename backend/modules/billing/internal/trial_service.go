package billing

import (
	"time"

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
		out = append(out, ClaimableItem{Policy: policies[i], Claimable: ok, NeedInvite: needInvite, ClaimedCount: claimed, Reason: reason})
	}
	return out, nil
}

var trialGrantSubjects = resourceSubjects // 试用仅发资源

func (s *trialServiceImpl) CreatePolicy(req *TrialPolicyCreate) (*TrialPolicy, error) {
	if !trialGrantSubjects[req.GrantSubject] {
		return nil, apperr.Validation("试用只能发放资源科目")
	}
	if req.GrantQuantity <= 0 {
		return nil, apperr.Validation("发放数量必须大于0")
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
		Code: req.Code, Name: req.Name, Enabled: enabled, GrantSubject: req.GrantSubject,
		GrantQuantity: req.GrantQuantity, GrantExpireDays: req.GrantExpireDays, PerUserLimit: limit,
		AllowNewUser: req.AllowNewUser, InviteCode: req.InviteCode,
	}
	if err := s.repo.createPolicy(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *trialServiceImpl) UpdatePolicy(id int, req *TrialPolicyUpdate) (*TrialPolicy, error) {
	if _, err := s.getPolicy(id); err != nil {
		return nil, err
	}
	fields := map[string]interface{}{}
	if req.Name != "" {
		fields["name"] = req.Name
	}
	if req.GrantQuantity != nil {
		if *req.GrantQuantity <= 0 {
			return nil, apperr.Validation("发放数量必须大于0")
		}
		fields["grant_quantity"] = *req.GrantQuantity
	}
	if req.GrantExpireDays != nil {
		fields["grant_expire_days"] = *req.GrantExpireDays
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
	var expireAt *time.Time
	if p.GrantExpireDays > 0 {
		exp := time.Now().AddDate(0, 0, p.GrantExpireDays)
		expireAt = &exp
	}
	return s.repo.claim(p, userID, expireAt)
}
