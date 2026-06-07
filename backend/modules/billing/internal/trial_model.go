package billing

import "time"

type TrialPolicy struct {
	ID              uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Code            string    `gorm:"type:varchar(64);not null;uniqueIndex:idx_billing_trial_code" json:"code"`
	Name            string    `gorm:"type:varchar(100);not null" json:"name"`
	Enabled         bool      `gorm:"not null;default:true" json:"enabled"`
	GrantSubject    string    `gorm:"type:varchar(20);not null" json:"grant_subject"`
	GrantQuantity   int64     `gorm:"not null" json:"grant_quantity"`
	GrantExpireDays int       `gorm:"not null;default:0" json:"grant_expire_days"`
	PerUserLimit    int       `gorm:"not null;default:1" json:"per_user_limit"`
	AllowNewUser    bool      `gorm:"not null;default:false" json:"allow_new_user"`
	InviteCode      string    `gorm:"type:varchar(64)" json:"invite_code"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (TrialPolicy) TableName() string { return "billing_trial_policies" }

type TrialGrant struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	PolicyID  uint      `gorm:"not null;index:idx_billing_trialgrant_policy_user" json:"policy_id"`
	UserID    uint      `gorm:"not null;index:idx_billing_trialgrant_policy_user" json:"user_id"`
	Subject   string    `gorm:"type:varchar(20);not null" json:"subject"`
	Quantity  int64     `gorm:"not null" json:"quantity"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (TrialGrant) TableName() string { return "billing_trial_grants" }

type TrialEligibility struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	PolicyID  uint      `gorm:"not null;index:idx_billing_trialelig_policy_user" json:"policy_id"`
	UserID    uint      `gorm:"not null;index:idx_billing_trialelig_policy_user" json:"user_id"`
	GrantedBy string    `gorm:"type:varchar(64)" json:"granted_by"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (TrialEligibility) TableName() string { return "billing_trial_eligibilities" }

type TrialPolicyCreate struct {
	Code            string `json:"code" binding:"required"`
	Name            string `json:"name" binding:"required"`
	GrantSubject    string `json:"grant_subject" binding:"required"`
	GrantQuantity   int64  `json:"grant_quantity" binding:"required"`
	GrantExpireDays int    `json:"grant_expire_days"`
	PerUserLimit    int    `json:"per_user_limit"`
	AllowNewUser    bool   `json:"allow_new_user"`
	InviteCode      string `json:"invite_code"`
	Enabled         *bool  `json:"enabled"`
}

type TrialPolicyUpdate struct {
	Name            string  `json:"name"`
	GrantQuantity   *int64  `json:"grant_quantity"`
	GrantExpireDays *int    `json:"grant_expire_days"`
	PerUserLimit    *int    `json:"per_user_limit"`
	AllowNewUser    *bool   `json:"allow_new_user"`
	InviteCode      *string `json:"invite_code"`
	Enabled         *bool   `json:"enabled"`
}

type ClaimRequest struct {
	InviteCode string `json:"invite_code"`
}

type EligibilityGrantRequest struct {
	UserID int `json:"user_id" binding:"required"`
}

type ClaimableItem struct {
	Policy       TrialPolicy `json:"policy"`
	Claimable    bool        `json:"claimable"`
	NeedInvite   bool        `json:"need_invite"`
	ClaimedCount int         `json:"claimed_count"`
	Reason       string      `json:"reason"`
}
