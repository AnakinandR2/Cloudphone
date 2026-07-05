package billing

import (
	"errors"
	"strconv"
	"time"

	"manager-backend/framework/apperr"

	"gorm.io/gorm"
)

func itoa(n int) string { return strconv.Itoa(n) }

type trialRepository interface {
	createPolicy(p *TrialPolicy) error
	updatePolicy(id int, fields map[string]interface{}) error
	replacePolicyItems(policyID int, items []TrialPolicyItem) error
	deletePolicy(id int) error
	getPolicy(id int) (*TrialPolicy, error)
	getPolicyByCode(code string) (*TrialPolicy, error)
	listPolicies(enabledOnly bool) ([]TrialPolicy, error)
	getFeatured() (*TrialPolicy, error)
	setFeatured(id int) error

	countClaims(policyID, userID int) (int, error)
	manualEligible(policyID, userID int) (bool, error)
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

// getFeatured 取被标记营销展示的策略（含发放项）；无则 ErrRecordNotFound。
func (r *gormTrialRepository) getFeatured() (*TrialPolicy, error) {
	var p TrialPolicy
	if err := r.db.Preload("Items").Where("marketing_featured = ?", true).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// setFeatured 单选设置营销展示：事务内先清空全部标记，再置 id（id<=0 仅清空=取消）。
func (r *gormTrialRepository) setFeatured(id int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&TrialPolicy{}).Where("marketing_featured = ?", true).
			Update("marketing_featured", false).Error; err != nil {
			return err
		}
		if id > 0 {
			return tx.Model(&TrialPolicy{}).Where("id = ?", id).
				Update("marketing_featured", true).Error
		}
		return nil
	})
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

func (r *gormTrialRepository) createEligibility(e *TrialEligibility) error {
	return r.db.Create(e).Error
}

func (r *gormTrialRepository) listGrants(policyID int) ([]TrialGrant, error) {
	var items []TrialGrant
	err := r.db.Where("policy_id = ?", policyID).Order("id DESC").Find(&items).Error
	return items, err
}

// claim 单事务：守卫式限领（按 TrialClaim）+ 建领取头 + 遍历各发放项发放（新模型）。
//   - seat / boot_slot：在 tx 内创建 Quantity 个 LicenseUnit（到期 = now + ExpireDays 天，0 视为永久=100 年）。
//   - runtime_minute：给 RuntimeMinuteWallet 加 Quantity 分钟。
//
// 每项写一条 LedgerEntry(type=trial)。与 TrialClaim/TrialGrant 同一 tx，保证原子性。
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
		ref := "trial:" + policy.Code
		for _, it := range policy.Items {
			if err := tx.Create(&TrialGrant{ClaimID: claim.ID, PolicyID: policy.ID, UserID: uint(userID), Subject: it.Subject, Quantity: it.Quantity}).Error; err != nil {
				return err
			}
			balanceAfter, err := grantTrialItem(tx, userID, it, now, ref)
			if err != nil {
				return err
			}
			if err := tx.Create(&LedgerEntry{
				UserID: uint(userID), Subject: it.Subject, Type: LedgerTrial,
				Delta: it.Quantity, BalanceAfter: balanceAfter, Reason: ref, Operator: "user:" + itoa(userID),
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// grantTrialItem 在事务内发放单个试用项，返回发放后的容量/余量（写入 LedgerEntry.BalanceAfter）。
func grantTrialItem(tx *gorm.DB, userID int, it TrialPolicyItem, now time.Time, ref string) (int64, error) {
	switch it.Subject {
	case KindSeat, KindBootSlot:
		// ExpireDays==0 视为永久 → 100 年后；否则 now + ExpireDays 天。
		expire := now.AddDate(100, 0, 0)
		if it.ExpireDays > 0 {
			expire = now.AddDate(0, 0, it.ExpireDays)
		}
		units := make([]LicenseUnit, 0, it.Quantity)
		for i := int64(0); i < it.Quantity; i++ {
			units = append(units, LicenseUnit{
				UserID: uint(userID), Kind: it.Subject, Status: LicenseActive,
				Source: SourceTrial, SourceRef: ref, ExpireAt: expire,
			})
		}
		if err := tx.Create(&units).Error; err != nil {
			return 0, err
		}
		// 发放后该类未过期单元数（容量）。
		var cap int64
		if err := tx.Model(&LicenseUnit{}).
			Where("user_id = ? AND kind = ? AND status = ? AND expire_at > ?", userID, it.Subject, LicenseActive, now).
			Count(&cap).Error; err != nil {
			return 0, err
		}
		return cap, nil
	case SubjectRuntimeMinute:
		var w RuntimeMinuteWallet
		if err := tx.Where(RuntimeMinuteWallet{UserID: uint(userID)}).FirstOrCreate(&w).Error; err != nil {
			return 0, err
		}
		if err := tx.Model(&RuntimeMinuteWallet{}).Where("user_id = ?", userID).
			Update("remaining_minutes", gorm.Expr("remaining_minutes + ?", it.Quantity)).Error; err != nil {
			return 0, err
		}
		if err := tx.Where("user_id = ?", userID).First(&w).Error; err != nil {
			return 0, err
		}
		return w.RemainingMinutes, nil
	}
	return 0, apperr.Validation("非法的试用发放科目")
}

func isNotFoundTrial(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }
