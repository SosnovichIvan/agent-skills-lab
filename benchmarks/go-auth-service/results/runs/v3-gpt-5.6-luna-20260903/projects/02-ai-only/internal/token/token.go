package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrInvalid = errors.New("invalid access token")

type Service struct {
	secret []byte
	issuer string
	ttl    time.Duration
}
type Claims struct {
	Subject, Email      string
	IssuedAt, ExpiresAt time.Time
}
type header struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}
type payload struct {
	Subject   string `json:"sub"`
	Email     string `json:"email"`
	Issuer    string `json:"iss"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

func New(secret []byte, issuer string, ttl time.Duration) Service {
	return Service{append([]byte(nil), secret...), issuer, ttl}
}

func (s Service) Issue(subject, email string, now time.Time) (string, error) {
	h := encode(header{"HS256", "JWT"})
	p := encode(payload{subject, email, s.issuer, now.Unix(), now.Add(s.ttl).Unix()})
	return h + "." + p + "." + s.sign(h+"."+p), nil
}

func (s Service) Validate(raw string, now time.Time) (Claims, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return Claims{}, ErrInvalid
	}
	var h header
	var p payload
	if !decode(parts[0], &h) || !decode(parts[1], &p) || h.Algorithm != "HS256" || h.Type != "JWT" {
		return Claims{}, ErrInvalid
	}
	expectedText := s.sign(parts[0] + "." + parts[1])
	provided, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, ErrInvalid
	}
	expected, err := base64.RawURLEncoding.DecodeString(expectedText)
	if err != nil || subtle.ConstantTimeCompare(expected, provided) != 1 {
		return Claims{}, ErrInvalid
	}
	if p.Subject == "" || p.Email == "" || p.Issuer != s.issuer || p.IssuedAt <= 0 || p.ExpiresAt <= p.IssuedAt || now.Unix() >= p.ExpiresAt {
		return Claims{}, ErrInvalid
	}
	return Claims{p.Subject, p.Email, time.Unix(p.IssuedAt, 0), time.Unix(p.ExpiresAt, 0)}, nil
}

func (s Service) sign(input string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func encode(v any) string { b, _ := json.Marshal(v); return base64.RawURLEncoding.EncodeToString(b) }
func decode(raw string, v any) bool {
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return false
	}
	return json.Unmarshal(b, v) == nil
}
