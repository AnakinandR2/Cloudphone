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
