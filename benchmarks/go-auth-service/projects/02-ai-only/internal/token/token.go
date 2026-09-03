package token

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInvalid = errors.New("invalid token")

type Issuer struct {
	secret []byte
	issuer string
	ttl    time.Duration
}
type claims struct {
	Sub string `json:"sub"`
	Iss string `json:"iss"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
	JTI string `json:"jti"`
}

func NewIssuer(secret, issuer string, ttl time.Duration) *Issuer {
	return &Issuer{[]byte(secret), issuer, ttl}
}
func (i *Issuer) Issue(subject string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	now := time.Now().UTC()
	c := claims{subject, i.issuer, now.Unix(), now.Add(i.ttl).Unix(), base64.RawURLEncoding.EncodeToString(b)}
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, _ := json.Marshal(c)
	enc := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, i.secret)
	mac.Write([]byte(enc))
	return enc + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func (i *Issuer) Validate(raw string) (string, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return "", ErrInvalid
	}
	mac := hmac.New(sha256.New, i.secret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(mac.Sum(nil), sig) {
		return "", ErrInvalid
	}
	var header map[string]string
	hb, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || json.Unmarshal(hb, &header) != nil || header["alg"] != "HS256" || header["typ"] != "JWT" {
		return "", ErrInvalid
	}
	pb, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", ErrInvalid
	}
	var c claims
	if json.Unmarshal(pb, &c) != nil || c.Sub == "" || c.Iss != i.issuer {
		return "", ErrInvalid
	}
	now := time.Now().Unix()
	if c.Exp <= now || c.Iat > now+60 || c.Exp <= c.Iat {
		return "", ErrInvalid
	}
	return c.Sub, nil
}
func Bearer(value string) (string, error) {
	p := strings.Fields(value)
	if len(p) != 2 || !strings.EqualFold(p[0], "Bearer") || p[1] == "" {
		return "", fmt.Errorf("%w: bearer token", ErrInvalid)
	}
	return p[1], nil
}
