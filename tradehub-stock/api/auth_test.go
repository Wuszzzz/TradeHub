package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

// makeToken 构造一个与 simplejwt 同构的 HS256 token，便于测试验签。
func makeToken(secret string, claims map[string]any) string {
	header := map[string]any{"alg": "HS256", "typ": "JWT"}
	enc := func(v any) string {
		b, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	signingInput := enc(header) + "." + enc(claims)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signingInput + "." + sig
}

func TestVerifyToken(t *testing.T) {
	secret := "tradehub-test-secret"
	cfg := authConfig{secret: []byte(secret), enabled: true}
	future := time.Now().Add(time.Hour).Unix()

	t.Run("有效 access token", func(t *testing.T) {
		tok := makeToken(secret, map[string]any{"user_id": 42, "token_type": "access", "exp": future})
		uid, err := cfg.verifyToken(tok)
		if err != nil {
			t.Fatalf("期望验签通过，得到错误: %v", err)
		}
		if uid != "42" {
			t.Fatalf("期望 user_id=42，得到 %q", uid)
		}
	})

	t.Run("错误密钥拒绝", func(t *testing.T) {
		tok := makeToken("wrong-secret", map[string]any{"user_id": 1, "token_type": "access", "exp": future})
		if _, err := cfg.verifyToken(tok); err != errBadSignature {
			t.Fatalf("期望 errBadSignature，得到 %v", err)
		}
	})

	t.Run("过期 token 拒绝", func(t *testing.T) {
		tok := makeToken(secret, map[string]any{"user_id": 1, "token_type": "access", "exp": time.Now().Add(-time.Minute).Unix()})
		if _, err := cfg.verifyToken(tok); err != errTokenExpired {
			t.Fatalf("期望 errTokenExpired，得到 %v", err)
		}
	})

	t.Run("refresh token 拒绝", func(t *testing.T) {
		tok := makeToken(secret, map[string]any{"user_id": 1, "token_type": "refresh", "exp": future})
		if _, err := cfg.verifyToken(tok); err != errWrongTokenType {
			t.Fatalf("期望 errWrongTokenType，得到 %v", err)
		}
	})

	t.Run("缺 user_id 拒绝", func(t *testing.T) {
		tok := makeToken(secret, map[string]any{"token_type": "access", "exp": future})
		if _, err := cfg.verifyToken(tok); err != errNoUserID {
			t.Fatalf("期望 errNoUserID，得到 %v", err)
		}
	})

	t.Run("格式错误拒绝", func(t *testing.T) {
		if _, err := cfg.verifyToken("not.a.valid.jwt.token"); err != errMalformedToken {
			t.Fatalf("期望 errMalformedToken，得到 %v", err)
		}
	})
}
