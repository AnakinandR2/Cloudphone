package billing

import (
	"errors"
	"time"

	"manager-backend/framework/apperr"

	"gorm.io/gorm"
)

type trialRepository interface {
	createPolicy(p *TrialPolicy) error
	updatePolicy(id int, fields map[string]interface{}) error
	deletePolicy(id int) error
	getPolicy(id int) (*TrialPolicy, error)
	getPolicyByCode(code string) (*TrialPolicy, error)
	listPolicies(enabledOnly bool) ([]TrialPolicy, error)

	countClaims(policyID, userID int) (int, error)
	manualEligible(policyID, userID int) (bool, error)
	countPaidOrders(userID int) (int, error)
	createEligibility(e *TrialEligibility) error
	listGrants(policyID int) ([]TrialGrant, error)

	claim(policy *TrialPolicy, userID int, expireAt *time.Time) error
}

type gormTrialRepository struct{ db *gorm.DB }

func newTrialRepository(db *gorm.DB) trialRepository { return &gormTrialRepository{db: db} }

func (r *gormTrialRepository) createPolicy(p *TrialPolicy) error { return r.db.Create(p).Error }

func (r *gormTrialRepository) updatePolicy(id int, fields map[string]interface{}) error {
	return r.db.Model(&TrialPolicy{}).Where("id = ?", id).Updates(fields).Error
}

func (r *gormTrialRepository) deletePolicy(id int) error {
	return r.db.Where("id = ?", id).Delete(&TrialPolicy{}).Error
}

func (r *gormTrialRepository) getPolicy(id int) (*TrialPolicy, error) {
	var p TrialPolicy
	if err := r.db.Where("id = ?", id).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *gormTrialRepository) getPolicyByCode(code string) (*TrialPolicy, error) {
	var p TrialPolicy
	if err := r.db.Where("code = ?", code).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *gormTrialRepository) listPolicies(enabledOnly bool) ([]TrialPolicy, error) {
	q := r.db.Model(&TrialPolicy{})
	if enabledOnly {
		q = q.Where("enabled = ?", true)
	}
	var items []TrialPolicy
	err := q.Order("id DESC").Find(&items).Error
	return items, err
}

func (r *gormTrialRepository) countClaims(policyID, userID int) (int, error) {
	var n int64
	err := r.db.Model(&TrialGrant{}).Where("policy_id = ? AND user_id = ?", policyID, userID).Count(&n).Error
	return int(n), err
}

func (r *gormTrialRepository) manualEligible(policyID, userID int) (bool, error) {
	var n int64
	err := r.db.Model(&TrialEligibility{}).Where("policy_id = ? AND user_id = ?", policyID, userID).Count(&n).Error
	return n > 0, err
}

func (r *gormTrialRepository) countPaidOrders(userID int) (int, error) {
	var n int64
	err := r.db.Model(&Order{}).Where("user_id = ? AND status = ?", userID, OrderPaid).Count(&n).Error
	return int(n), err
}

func (r *gormTrialRepository) createEligibility(e *TrialEligibility) error {
	return r.db.Create(e).Error
}

func (r *gormTrialRepository) listGrants(policyID int) ([]TrialGrant, error) {
	var items []TrialGrant
	err := r.db.Where("policy_id = ?", policyID).Order("id DESC").Find(&items).Error
	return items, err
}

// claim 单事务：守卫式限领 + 写 TrialGrant + 发放权益(type=trial)。
func (r *gormTrialRepository) claim(policy *TrialPolicy, userID int, expireAt *time.Time) error {
	now := time.Now()
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 并发硬化（Phase 2 前置）：sqlite 串行化下安全；迁 MySQL/PG 后，限领守卫为
		// 读后写(COUNT 再 INSERT)、非原子，需对 (policy_id,user_id) 加唯一约束或行锁防并发超领。
		var claimed int64
		if err := tx.Model(&TrialGrant{}).Where("policy_id = ? AND user_id = ?", policy.ID, userID).Count(&claimed).Error; err != nil {
			return err
		}
		if int(claimed) >= policy.PerUserLimit {
			return apperr.Conflict("已达领取上限")
		}
		if err := tx.Create(&TrialGrant{PolicyID: policy.ID, UserID: uint(userID), Subject: policy.GrantSubject, Quantity: policy.GrantQuantity}).Error; err != nil {
			return err
		}
		entRepo := &gormEntitlementRepository{db: tx}
		batch := EntitlementBatch{UserID: uint(userID), Subject: policy.GrantSubject, Quantity: policy.GrantQuantity, Source: SourceTrial, SourceRef: "trial:" + policy.Code, ExpireAt: expireAt}
		if err := entRepo.createBatch(&batch); err != nil {
			return err
		}
		capacity, err := entRepo.capacity(userID, policy.GrantSubject, now)
		if err != nil {
			return err
		}
		return entRepo.insertLedger(&LedgerEntry{
			UserID: uint(userID), Subject: policy.GrantSubject, Type: LedgerTrial,
			Delta: policy.GrantQuantity, BalanceAfter: capacity, Reason: "trial:" + policy.Code, Operator: "user:" + itoa(userID),
		})
	})
}

func isNotFoundTrial(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
