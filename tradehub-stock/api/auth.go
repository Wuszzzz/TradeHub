package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// JWT 鉴权：与 Django(djangorestframework-simplejwt) 同 SECRET_KEY、HS256 验签。
// simplejwt 默认：算法 HS256，签名密钥=SECRET_KEY，token_type=access，用户标识 claim=user_id。
// 这里仅做"验签 + 过期 + 类型"校验，不查库；用户身份通过 context 透传给下游 handler。

type ctxKey string

const ctxUserIDKey ctxKey = "stock_user_id"

// authConfig 持有鉴权中间件的运行期配置。
type authConfig struct {
	secret  []byte
	enabled bool
}

// newAuthConfig 从环境变量加载鉴权配置。
// SECRET_KEY 与 Django 共享；AUTH_ENABLED=false 可在纯本地联调时关闭（默认开启）。
func newAuthConfig() authConfig {
	secret := env("SECRET_KEY", "")
	enabled := strings.ToLower(env("AUTH_ENABLED", "true")) != "false"
	return authConfig{
		secret:  []byte(secret),
		enabled: enabled,
	}
}

// jwtClaims 仅声明我们关心的字段。
type jwtClaims struct {
	UserID    json.Number `json:"user_id"`
	TokenType string      `json:"token_type"`
	Exp       int64       `json:"exp"`
}

// requireAuth 包装一组路由，校验 Authorization: Bearer <token>。
// 校验通过则将 user_id 注入 context；失败返回 401。
func (a authConfig) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 鉴权关闭（本地联调）或未配置 SECRET_KEY 时直接放行，避免误伤开发环境。
		if !a.enabled || len(a.secret) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		// 放行 CORS 预检，浏览器预检不带 Authorization 头。
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		userID, err := a.verifyRequest(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error":  "unauthorized",
				"detail": err.Error(),
			})
			return
		}

		ctx := context.WithValue(r.Context(), ctxUserIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// verifyRequest 从请求头提取并校验 JWT，返回 user_id。
func (a authConfig) verifyRequest(r *http.Request) (string, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", errMissingToken
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", errMalformedHeader
	}
	return a.verifyToken(parts[1])
}

// verifyToken 校验 HS256 JWT 的签名、过期时间与 token 类型，返回 user_id。
func (a authConfig) verifyToken(token string) (string, error) {
	segments := strings.Split(token, ".")
	if len(segments) != 3 {
		return "", errMalformedToken
	}
	headerSeg, payloadSeg, signatureSeg := segments[0], segments[1], segments[2]

	// 1. 校验签名（HS256）。
	signingInput := headerSeg + "." + payloadSeg
	expectedSig := signHS256([]byte(signingInput), a.secret)
	gotSig, err := base64.RawURLEncoding.DecodeString(signatureSeg)
	if err != nil {
		return "", errMalformedToken
	}
	if subtle.ConstantTimeCompare(expectedSig, gotSig) != 1 {
		return "", errBadSignature
	}

	// 2. 校验头部算法，拒绝 alg=none 等降级攻击。
	headerBytes, err := base64.RawURLEncoding.DecodeString(headerSeg)
	if err != nil {
		return "", errMalformedToken
	}
	var head struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerBytes, &head); err != nil {
		return "", errMalformedToken
	}
	if !strings.EqualFold(head.Alg, "HS256") {
		return "", errUnsupportedAlg
	}

	// 3. 解析 payload，校验过期时间与 token 类型。
	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadSeg)
	if err != nil {
		return "", errMalformedToken
	}
	var claims jwtClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return "", errMalformedToken
	}
	if claims.Exp > 0 && time.Now().Unix() >= claims.Exp {
		return "", errTokenExpired
	}
	// simplejwt 的 access token 带 token_type=access；refresh token 不应放行。
	if claims.TokenType != "" && claims.TokenType != "access" {
		return "", errWrongTokenType
	}
	if claims.UserID.String() == "" {
		return "", errNoUserID
	}
	return claims.UserID.String(), nil
}

// signHS256 计算 HMAC-SHA256 签名。
func signHS256(data, secret []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write(data)
	return mac.Sum(nil)
}

// userIDFromContext 供下游 handler 读取已鉴权的用户 id。
func userIDFromContext(ctx context.Context) (int64, bool) {
	v, ok := ctx.Value(ctxUserIDKey).(string)
	if !ok || v == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

// 鉴权相关错误。
var (
	errMissingToken    = authError("missing authorization header")
	errMalformedHeader = authError("authorization header must be 'Bearer <token>'")
	errMalformedToken  = authError("malformed jwt")
	errBadSignature    = authError("invalid token signature")
	errUnsupportedAlg  = authError("unsupported jwt algorithm")
	errTokenExpired    = authError("token expired")
	errWrongTokenType  = authError("token type is not access")
	errNoUserID        = authError("token missing user_id claim")
)

type authError string

func (e authError) Error() string { return string(e) }
