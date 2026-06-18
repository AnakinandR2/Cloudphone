package openapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"manager-backend/framework/apperr"
	"manager-backend/framework/crypto"

	"gorm.io/gorm"
)

// Service 模块内服务实例，由 module.Init 注入。
var Service *serviceImpl

type serviceImpl struct {
	repo repository
}

func newService(repo repository) *serviceImpl { return &serviceImpl{repo: repo} }

const keyPrefixLiteral = "gp_live_"

// lastUsedThrottle 是 last_used_at 的最小写库间隔（避免每次请求都写库）。
const lastUsedThrottle = 60 * time.Second

// hashKey 计算密钥的 sha256 十六进制。
func hashKey(full string) string {
	sum := sha256.Sum256([]byte(full))
	return hex.EncodeToString(sum[:])
}

// generateKey 生成一把新密钥，返回明文 + 前缀 + 末4位 + 哈希。
func generateKey() (full, prefix, last4, hash string, err error) {
	b := make([]byte, 24)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", "", err
	}
	raw := hex.EncodeToString(b) // 48 个十六进制字符
	full = keyPrefixLiteral + raw
	prefix = keyPrefixLiteral + raw[:4]
	last4 = raw[len(raw)-4:]
	hash = hashKey(full)
	return full, prefix, last4, hash, nil
}

// Create 为某用户签发一把新密钥，返回安全视图 + 完整明文（仅此一次）。
func (s *serviceImpl) Create(userID int, name string) (*CreateKeyResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, apperr.BadRequest("请填写密钥名称")
	}
	full, prefix, last4, hash, err := generateKey()
	if err != nil {
		return nil, apperr.Internal("生成密钥失败：" + err.Error())
	}
	cipher, err := crypto.Encrypt(full) // 加密存储，供「显示」重复查看
	if err != nil {
		return nil, apperr.Internal("加密密钥失败：" + err.Error())
	}
	k := &APIKey{
		UserID: uint(userID), Name: name,
		KeyPrefix: prefix, KeyLast4: last4, KeyHash: hash, KeyCipher: cipher,
	}
	if err := s.repo.create(k); err != nil {
		return nil, err
	}
	return &CreateKeyResult{KeyView: toView(*k), FullKey: full}, nil
}

// List 列出某用户的密钥（安全视图）。
func (s *serviceImpl) List(userID int) ([]KeyView, error) {
	keys, err := s.repo.listByUser(userID)
	if err != nil {
		return nil, err
	}
	out := make([]KeyView, 0, len(keys))
	for _, k := range keys {
		out = append(out, toView(k))
	}
	return out, nil
}

// Revoke 撤销某用户的某把密钥（软删，立即失效）。
func (s *serviceImpl) Revoke(userID int, id uint) error {
	k, err := s.repo.ownedByID(userID, id)
	if err != nil {
		return apperr.NotFound("密钥不存在")
	}
	if k.RevokedAt != nil {
		return nil // 幂等
	}
	return s.repo.revoke(k.ID, time.Now())
}

// Reveal 解密返回某用户某把密钥的完整明文（供「重复查看」）。
// 旧密钥（加密前创建，无 cipher）→ 明确报错。撤销的密钥仍可显示（只读历史）。
func (s *serviceImpl) Reveal(userID int, id uint) (string, error) {
	k, err := s.repo.ownedByID(userID, id)
	if err != nil {
		return "", apperr.NotFound("密钥不存在")
	}
	if k.KeyCipher == "" {
		return "", apperr.Validation("该密钥不支持显示，请删除后重建")
	}
	full, err := crypto.Decrypt(k.KeyCipher)
	if err != nil {
		return "", apperr.Internal("解密密钥失败：" + err.Error())
	}
	return full, nil
}

// Authenticate 校验明文密钥，返回属主 userID。失败返回 ok=false。
// 命中后节流更新 last_used_at（best-effort，失败不影响鉴权）。
func (s *serviceImpl) Authenticate(rawKey string) (userID int, ok bool) {
	rawKey = strings.TrimSpace(rawKey)
	if !strings.HasPrefix(rawKey, keyPrefixLiteral) {
		return 0, false
	}
	k, err := s.repo.findByHash(hashKey(rawKey))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, false
		}
		return 0, false
	}
	if k.RevokedAt != nil {
		return 0, false
	}
	now := time.Now()
	if k.LastUsedAt == nil || now.Sub(*k.LastUsedAt) > lastUsedThrottle {
		_ = s.repo.touchLastUsed(k.ID, now)
	}
	return int(k.UserID), true
}
