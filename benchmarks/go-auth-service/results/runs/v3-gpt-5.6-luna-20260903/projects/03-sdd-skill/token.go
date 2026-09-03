package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrInvalidToken = errors.New("invalid token")
var rawURL = base64.RawURLEncoding

type TokenManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

func NewTokenManager(secret []byte, issuer string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: append([]byte(nil), secret...), issuer: issuer, ttl: ttl}
}

func (m *TokenManager) Create(user PublicUser) (string, error) {
	now := time.Now().UTC()
	jtiBytes := make([]byte, 16)
	if _, err := rand.Read(jtiBytes); err != nil {
		return "", err
	}
	header := map[string]string{"typ": "JWT", "alg": "HS256"}
	claims := map[string]any{"sub": user.ID, "email": user.Email, "iss": m.issuer, "iat": now.Unix(), "exp": now.Add(m.ttl).Unix(), "jti": rawURL.EncodeToString(jtiBytes)}
	encodedHeader, _ := json.Marshal(header)
	encodedClaims, _ := json.Marshal(claims)
	unsigned := rawURL.EncodeToString(encodedHeader) + "." + rawURL.EncodeToString(encodedClaims)
	return unsigned + "." + sign(unsigned, m.secret), nil
}

func sign(value string, secret []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(value))
	return rawURL.EncodeToString(h.Sum(nil))
}

func (m *TokenManager) Validate(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return Claims{}, ErrInvalidToken
	}
	expected := sign(parts[0]+"."+parts[1], m.secret)
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return Claims{}, ErrInvalidToken
	}
	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}
	if err := decodeJSON(parts[0], &header); err != nil || header.Algorithm != "HS256" || header.Type != "JWT" {
		return Claims{}, ErrInvalidToken
	}
	var payload struct {
		Subject string `json:"sub"`
		Email   string `json:"email"`
		Issuer  string `json:"iss"`
		Issued  int64  `json:"iat"`
		Expires int64  `json:"exp"`
		JTI     string `json:"jti"`
	}
	if err := decodeJSON(parts[1], &payload); err != nil || payload.Subject == "" || payload.Email == "" || payload.Issuer != m.issuer || payload.JTI == "" {
		return Claims{}, ErrInvalidToken
	}
	now := time.Now().Unix()
	if payload.Issued <= 0 || payload.Expires <= payload.Issued || now >= payload.Expires || payload.Issued > now+60 {
		return Claims{}, ErrInvalidToken
	}
	return Claims{Subject: payload.Subject, Email: payload.Email, Issuer: payload.Issuer, Issued: time.Unix(payload.Issued, 0), Expires: time.Unix(payload.Expires, 0), JTI: payload.JTI}, nil
}

func decodeJSON(part string, destination any) error {
	b, err := rawURL.DecodeString(part)
	if err != nil {
		return ErrInvalidToken
	}
	if err := json.Unmarshal(b, destination); err != nil {
		return ErrInvalidToken
	}
	return nil
}
