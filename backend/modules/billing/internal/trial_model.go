package billing

import "time"

type TrialPolicy struct {
	ID           uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Code         string `gorm:"type:varchar(64);not null;uniqueIndex:idx_billing_trial_code" json:"code"`
	Name         string `gorm:"type:varchar(100);not null" json:"name"`
	Enabled      bool   `gorm:"not null;default:true" json:"enabled"`
	PerUserLimit int    `gorm:"not null;default:1" json:"per_user_limit"`
	AllowNewUser bool   `gorm:"not null;default:false" json:"allow_new_user"`
	InviteCode   string `gorm:"type:varchar(64)" json:"invite_code"`
	// MarketingFeatured 标记本策略在营销站(www /pricing)展示；全局至多一条（服务层单选保证）。
	MarketingFeatured bool              `gorm:"not null;default:false" json:"marketing_featured"`
	Items             []TrialPolicyItem `gorm:"foreignKey:PolicyID" json:"items"`
	CreatedAt         time.Time         `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time         `gorm:"autoUpdateTime" json:"updated_at"`
}

func (TrialPolicy) TableName() string { return "billing_trial_policies" }

// TrialPolicyItem 是一个试用策略的一条发放项（一策略可含多条，每科目至多一条）。
type TrialPolicyItem struct {
	ID         uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	PolicyID   uint   `gorm:"not null;uniqueIndex:idx_billing_trialitem_policy_subject" json:"policy_id"`
	Subject    string `gorm:"type:varchar(20);not null;uniqueIndex:idx_billing_trialitem_policy_subject" json:"subject"`
	Quantity   int64  `gorm:"not null" json:"quantity"`
	ExpireDays int    `gorm:"not null;default:0" json:"expire_days"` // 0 = 永久
}

func (TrialPolicyItem) TableName() string { return "billing_trial_policy_items" }

// TrialClaim 是一次领取（领取头）：一次领取一行，限领次数按本表计。
type TrialClaim struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	PolicyID  uint      `gorm:"not null;index:idx_billing_trialclaim_policy_user" json:"policy_id"`
	UserID    uint      `gorm:"not null;index:idx_billing_trialclaim_policy_user" json:"user_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (TrialClaim) TableName() string { return "billing_trial_claims" }

type TrialGrant struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ClaimID   uint      `gorm:"not null;index:idx_billing_trialgrant_claim" json:"claim_id"`
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

// TrialPolicyItemInput 是创建/更新策略时的发放项入参。
type TrialPolicyItemInput struct {
	Subject    string `json:"subject" binding:"required"`
	Quantity   int64  `json:"quantity" binding:"required"`
	ExpireDays int    `json:"expire_days"`
}

type TrialPolicyCreate struct {
	Code         string                 `json:"code" binding:"required"`
	Name         string                 `json:"name" binding:"required"`
	Items        []TrialPolicyItemInput `json:"items" binding:"required"`
	PerUserLimit int                    `json:"per_user_limit"`
	AllowNewUser bool                   `json:"allow_new_user"`
	InviteCode   string                 `json:"invite_code"`
	Enabled      *bool                  `json:"enabled"`
}

type TrialPolicyUpdate struct {
	Name         string                  `json:"name"`
	Items        *[]TrialPolicyItemInput `json:"items"` // nil=不改；非 nil=整体替换
	PerUserLimit *int                    `json:"per_user_limit"`
	AllowNewUser *bool                   `json:"allow_new_user"`
	InviteCode   *string                 `json:"invite_code"`
	Enabled      *bool                   `json:"enabled"`
}

type ClaimRequest struct {
	InviteCode string `json:"invite_code"`
}

type EligibilityGrantRequest struct {
	UserID int `json:"user_id" binding:"required"`
}

// TrialFeatureRequest 营销展示单选标记入参（featured=true 设为展示，false 取消）。
type TrialFeatureRequest struct {
	Featured bool `json:"featured"`
}

type ClaimableItem struct {
	Policy       TrialPolicy `json:"policy"`
	Claimable    bool        `json:"claimable"`
	NeedInvite   bool        `json:"need_invite"`
	ClaimedCount int         `json:"claimed_count"`
	Reason       string      `json:"reason"`
}
