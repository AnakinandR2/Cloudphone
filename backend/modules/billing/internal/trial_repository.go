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
	replacePolicyItems(policyID int, items []TrialPolicyItem) error
	deletePolicy(id int) error
	getPolicy(id int) (*TrialPolicy, error)
	getPolicyByCode(code string) (*TrialPolicy, error)
	listPolicies(enabledOnly bool) ([]TrialPolicy, error)

	countClaims(policyID, userID int) (int, error)
	manualEligible(policyID, userID int) (bool, error)
	countPaidOrders(userID int) (int, error)
	createEligibility(e *TrialEligibility) error
	listGrants(policyID int) ([]TrialGrant, error)

	claim(policy *TrialPolicy, userID int) error
}

type gormTrialRepository struct{ db *gorm.DB }

func newTrialRepository(db *gorm.DB) trialRepository { return &gormTrialRepository{db: db} }

// createPolicy 建策略 + 其发放项（GORM 关联级联创建）。
func (r *gormTrialRepository) createPolicy(p *TrialPolicy) error { return r.db.Create(p).Error }

func (r *gormTrialRepository) updatePolicy(id int, fields map[string]interface{}) error {
	return r.db.Model(&TrialPolicy{}).Where("id = ?", id).Updates(fields).Error
}

// replacePolicyItems 整体替换某策略的发放项（删旧建新，单事务）。
func (r *gormTrialRepository) replacePolicyItems(policyID int, items []TrialPolicyItem) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("policy_id = ?", policyID).Delete(&TrialPolicyItem{}).Error; err != nil {
			return err
		}
		for i := range items {
			items[i].ID = 0
			items[i].PolicyID = uint(policyID)
			if err := tx.Create(&items[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *gormTrialRepository) deletePolicy(id int) error {
	return r.db.Where("id = ?", id).Delete(&TrialPolicy{}).Error
}

func (r *gormTrialRepository) getPolicy(id int) (*TrialPolicy, error) {
	var p TrialPolicy
	if err := r.db.Preload("Items").Where("id = ?", id).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *gormTrialRepository) getPolicyByCode(code string) (*TrialPolicy, error) {
	var p TrialPolicy
	if err := r.db.Preload("Items").Where("code = ?", code).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *gormTrialRepository) listPolicies(enabledOnly bool) ([]TrialPolicy, error) {
	q := r.db.Preload("Items").Model(&TrialPolicy{})
	if enabledOnly {
		q = q.Where("enabled = ?", true)
	}
	var items []TrialPolicy
	err := q.Order("id DESC").Find(&items).Error
	return items, err
}

// countClaims 按领取头 TrialClaim 计数（一次领取一行，不受多发放项影响）。
func (r *gormTrialRepository) countClaims(policyID, userID int) (int, error) {
	var n int64
	err := r.db.Model(&TrialClaim{}).Where("policy_id = ? AND user_id = ?", policyID, userID).Count(&n).Error
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

// claim 单事务：守卫式限领（按 TrialClaim）+ 建领取头 + 遍历各发放项发放权益(type=trial)。
func (r *gormTrialRepository) claim(policy *TrialPolicy, userID int) error {
	now := time.Now()
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 并发硬化（Phase 2 前置）：sqlite 串行化下安全；迁 MySQL/PG 后，限领守卫为
		// 读后写(COUNT 再 INSERT)、非原子，需对 (policy_id,user_id) 加唯一约束或行锁防并发超领。
		var claimed int64
		if err := tx.Model(&TrialClaim{}).Where("policy_id = ? AND user_id = ?", policy.ID, userID).Count(&claimed).Error; err != nil {
			return err
		}
		if int(claimed) >= policy.PerUserLimit {
			return apperr.Conflict("已达领取上限")
		}
		claim := TrialClaim{PolicyID: policy.ID, UserID: uint(userID)}
		if err := tx.Create(&claim).Error; err != nil {
			return err
		}
		entRepo := &gormEntitlementRepository{db: tx}
		for _, it := range policy.Items {
			var expireAt *time.Time
			if it.ExpireDays > 0 {
				exp := now.AddDate(0, 0, it.ExpireDays)
				expireAt = &exp
			}
			if err := tx.Create(&TrialGrant{ClaimID: claim.ID, PolicyID: policy.ID, UserID: uint(userID), Subject: it.Subject, Quantity: it.Quantity}).Error; err != nil {
				return err
			}
			batch := EntitlementBatch{UserID: uint(userID), Subject: it.Subject, Quantity: it.Quantity, Source: SourceTrial, SourceRef: "trial:" + policy.Code, ExpireAt: expireAt}
			if err := entRepo.createBatch(&batch); err != nil {
				return err
			}
			capacity, err := entRepo.capacity(userID, it.Subject, now)
			if err != nil {
				return err
			}
			if err := entRepo.insertLedger(&LedgerEntry{
				UserID: uint(userID), Subject: it.Subject, Type: LedgerTrial,
				Delta: it.Quantity, BalanceAfter: capacity, Reason: "trial:" + policy.Code, Operator: "user:" + itoa(userID),
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func isNotFoundTrial(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
