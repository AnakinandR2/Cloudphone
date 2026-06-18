// Package crypto 提供对称加密工具，用于把需要「可还原展示」的敏感串（如 API 密钥明文）
// 加密存库。鉴权类场景仍应使用不可逆哈希，本包仅供「加密存储以便重复查看」用。
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"sync"

	"manager-backend/framework"
)

var (
	keyOnce sync.Once
	encKey  []byte // 32 字节 AES-256 密钥
)

// derivedKey 返回 AES-256 密钥：优先用配置 APIKEY_ENC_KEY（hex/base64 的 32 字节），
// 否则回退 sha256(JWTSecret + "apikey")（保证总有一把 32 字节密钥）。
func derivedKey() []byte {
	keyOnce.Do(func() {
		cfg := framework.AppConfig
		if cfg != nil && cfg.APIKeyEncKey != "" {
			if k := parse32(cfg.APIKeyEncKey); k != nil {
				encKey = k
				return
			}
		}
		seed := "change-me-in-production"
		if cfg != nil && cfg.JWTSecret != "" {
			seed = cfg.JWTSecret
		}
		sum := sha256.Sum256([]byte(seed + "|apikey-enc"))
		encKey = sum[:]
	})
	return encKey
}

// parse32 把 hex 或 base64 字符串解析为 32 字节密钥；不是 32 字节则返回 nil。
func parse32(s string) []byte {
	if b, err := hex.DecodeString(s); err == nil && len(b) == 32 {
		return b
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && len(b) == 32 {
		return b
	}
	return nil
}

// Encrypt 用 AES-256-GCM 加密明文，返回 base64(nonce || 密文)。
func Encrypt(plain string) (string, error) {
	block, err := aes.NewCipher(derivedKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt 解密 Encrypt 产出的 base64(nonce || 密文)。
func Decrypt(token string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(derivedKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("crypto: 密文过短")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
