package crypto

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	plain := "gp_live_0123456789abcdef0123456789abcdef0123456789abcdef"
	token, err := Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if token == plain {
		t.Fatal("密文不应等于明文")
	}
	got, err := Decrypt(token)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got != plain {
		t.Fatalf("往返不一致：%q != %q", got, plain)
	}
	// 两次加密同一明文应产出不同密文（随机 nonce）。
	token2, _ := Encrypt(plain)
	if token2 == token {
		t.Fatal("相同明文两次加密不应得到相同密文")
	}
	// 篡改密文解密应失败。
	if _, err := Decrypt(token + "AA"); err == nil {
		t.Fatal("篡改密文应解密失败")
	}
}
