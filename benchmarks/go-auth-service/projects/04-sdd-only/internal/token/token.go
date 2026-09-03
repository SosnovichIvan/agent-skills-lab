package token

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInvalid = errors.New("invalid token")

type Claims struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Issuer  string `json:"iss"`
	Issued  int64  `json:"iat"`
	Expires int64  `json:"exp"`
	JTI     string `json:"jti"`
}

type header struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

type Manager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

func NewManager(secret []byte, issuer string, ttl time.Duration) (*Manager, error) {
	if len(secret) < 32 || issuer == "" || ttl <= 0 {
		return nil, errors.New("invalid token configuration")
	}
	return &Manager{secret: append([]byte(nil), secret...), issuer: issuer, ttl: ttl}, nil
}

func (m *Manager) Create(subject, email string, now time.Time) (string, error) {
	var rawJTI [16]byte
	if _, err := rand.Read(rawJTI[:]); err != nil {
		return "", err
	}
	c := Claims{Subject: subject, Email: email, Issuer: m.issuer, Issued: now.Unix(), Expires: now.Add(m.ttl).Unix(), JTI: base64.RawURLEncoding.EncodeToString(rawJTI[:])}
	h, _ := json.Marshal(header{Algorithm: "HS256", Type: "JWT"})
	p, _ := json.Marshal(c)
	encodedHeader := base64.RawURLEncoding.EncodeToString(h)
	encodedPayload := base64.RawURLEncoding.EncodeToString(p)
	unsigned := encodedHeader + "." + encodedPayload
	return unsigned + "." + m.sign(unsigned), nil
}

func (m *Manager) Validate(value string, now time.Time) (Claims, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return Claims{}, ErrInvalid
	}
	var h header
	if decodeJSON(parts[0], &h) != nil || h.Algorithm != "HS256" || h.Type != "JWT" {
		return Claims{}, ErrInvalid
	}
	expected := m.sign(parts[0] + "." + parts[1])
	if subtle.ConstantTimeCompare([]byte(expected), []byte(parts[2])) != 1 {
		return Claims{}, ErrInvalid
	}
	var c Claims
	if decodeJSON(parts[1], &c) != nil || c.Subject == "" || c.Email == "" || c.JTI == "" || c.Issuer != m.issuer {
		return Claims{}, ErrInvalid
	}
	if c.Expires <= now.Unix() || c.Issued > now.Add(time.Minute).Unix() {
		return Claims{}, ErrInvalid
	}
	return c, nil
}

func (m *Manager) sign(value string) string {
	h := hmac.New(sha256.New, m.secret)
	h.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func decodeJSON(value string, destination any) error {
	b, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return json.Unmarshal(b, destination)
}
