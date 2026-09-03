package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type TokenManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

func (m *TokenManager) Create(user User) (string, error) {
	now := time.Now().Unix()
	payload := map[string]any{"sub": user.ID, "email": user.Email, "iss": m.issuer, "iat": now, "exp": time.Now().Add(m.ttl).Unix(), "jti": newID()}
	return m.sign(map[string]string{"typ": "JWT", "alg": "HS256"}, payload)
}
func (m *TokenManager) sign(header map[string]string, payload map[string]any) (string, error) {
	h, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	p, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	unsigned := base64URL(h) + "." + base64URL(p)
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(unsigned))
	return unsigned + "." + base64URL(mac.Sum(nil)), nil
}
func (m *TokenManager) Validate(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, errors.New("invalid token")
	}
	rawHeader, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, errors.New("invalid token")
	}
	var header struct{ Alg, Typ string }
	if json.Unmarshal(rawHeader, &header) != nil || header.Alg != "HS256" {
		return Claims{}, errors.New("invalid token algorithm")
	}
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	expected := mac.Sum(nil)
	actual, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || subtle.ConstantTimeCompare(expected, actual) != 1 {
		return Claims{}, errors.New("invalid token signature")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, errors.New("invalid token")
	}
	var v struct {
		Sub, Email, Iss string
		Iat, Exp        int64
		JTI             string `json:"jti"`
	}
	if json.Unmarshal(data, &v) != nil || v.Sub == "" || v.Email == "" || v.Iss != m.issuer || v.Iat <= 0 || v.Exp <= v.Iat || v.JTI == "" {
		return Claims{}, errors.New("invalid token claims")
	}
	if time.Now().Unix() >= v.Exp {
		return Claims{}, errors.New("token expired")
	}
	return Claims{Subject: v.Sub, Email: v.Email, Issuer: v.Iss, IssuedAt: v.Iat, ExpiresAt: v.Exp, JTI: v.JTI}, nil
}
func base64URL(data []byte) string     { return base64.RawURLEncoding.EncodeToString(data) }
func (m *TokenManager) String() string { return fmt.Sprintf("issuer=%s", m.issuer) }
