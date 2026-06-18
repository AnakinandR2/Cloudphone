package openapi

import "time"

// APIKey 是前台用户签发的开放 API 密钥。明文只在创建时返回一次，库内只存 sha256。
//
//	key_prefix  展示用前缀（gp_live_ + 明文前 4 位），便于掩码与排查
//	key_last4   明文末 4 位（掩码展示）
//	key_hash    sha256(完整明文)，鉴权按此查（唯一）
//	revoked_at  非空即失效（软撤销）
type APIKey struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	UserID     uint       `gorm:"index;not null" json:"-"`
	Name       string     `gorm:"size:100" json:"name"`
	KeyPrefix  string     `gorm:"size:32;index" json:"-"`
	KeyLast4   string     `gorm:"size:8" json:"-"`
	KeyHash    string     `gorm:"size:64;uniqueIndex" json:"-"`
	KeyCipher  string     `gorm:"type:text" json:"-"` // AES-GCM(明文)，供「显示」解密；旧密钥为空不可显示
	LastUsedAt *time.Time `json:"lastUsedAt"`
	RevokedAt  *time.Time `json:"revokedAt"`
	CreatedAt  time.Time  `json:"createdAt"`
}

func (APIKey) TableName() string { return "api_keys" }

// masked 返回掩码展示串：gp_live_3f2a••••a17c。
func (k APIKey) masked() string { return k.KeyPrefix + "••••" + k.KeyLast4 }

// KeyView 是密钥对前端的安全视图（不含明文/哈希）。
type KeyView struct {
	ID         uint       `json:"id"`
	Name       string     `json:"name"`
	Masked     string     `json:"masked"`
	Status     string     `json:"status"` // active / revoked
	LastUsedAt *time.Time `json:"lastUsedAt"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// toView 把 APIKey 转为安全视图。
func toView(k APIKey) KeyView {
	status := "active"
	if k.RevokedAt != nil {
		status = "revoked"
	}
	return KeyView{
		ID: k.ID, Name: k.Name, Masked: k.masked(), Status: status,
		LastUsedAt: k.LastUsedAt, CreatedAt: k.CreatedAt,
	}
}

// CreateKeyResult 是创建密钥的返回：安全视图 + 完整明文（仅此一次）。
type CreateKeyResult struct {
	KeyView
	FullKey string `json:"fullKey"`
}
