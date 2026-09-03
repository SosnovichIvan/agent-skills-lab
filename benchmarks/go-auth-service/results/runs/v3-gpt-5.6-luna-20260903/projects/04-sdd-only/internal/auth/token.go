package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrInvalidToken = errors.New("invalid token")

type TokenManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

func NewTokenManager(secret []byte, issuer string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: append([]byte(nil), secret...), issuer: issuer, ttl: ttl}
}

func (m *TokenManager) Create(user Profile, now time.Time) (string, error) {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return "", err
	}
	header, _ := json.Marshal(map[string]string{"typ": "JWT", "alg": "HS256"})
	claims, err := json.Marshal(map[string]any{"sub": user.ID, "email": user.Email, "iss": m.issuer, "iat": now.Unix(), "exp": now.Add(m.ttl).Unix(), "jti": hex.EncodeToString(id)})
	if err != nil {
		return "", err
	}
	part := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(part))
	return part + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (m *TokenManager) Validate(token string, now time.Time) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return Claims{}, ErrInvalidToken
	}
	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || json.Unmarshal(headerBytes, &header) != nil || header.Alg != "HS256" || header.Typ != "JWT" {
		return Claims{}, ErrInvalidToken
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return Claims{}, ErrInvalidToken
	}
	var raw struct {
		Sub, Email, Iss string
		IAT, Exp        int64
		JTI             string
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || json.Unmarshal(payload, &raw) != nil || raw.Iss != m.issuer || raw.Sub == "" || raw.Email == "" || raw.JTI == "" || raw.IAT <= 0 || raw.Exp <= raw.IAT || now.Unix() >= raw.Exp {
		return Claims{}, ErrInvalidToken
	}
	return Claims{Subject: raw.Sub, Email: raw.Email, Issuer: raw.Iss, IssuedAt: time.Unix(raw.IAT, 0), ExpiresAt: time.Unix(raw.Exp, 0), JWTID: raw.JTI}, nil
}
