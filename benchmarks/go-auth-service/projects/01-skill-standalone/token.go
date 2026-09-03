package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type tokenManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}
type tokenHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}
type tokenClaims struct {
	Subject   string `json:"sub"`
	Issuer    string `json:"iss"`
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
}

func (m tokenManager) sign(subject string, now time.Time) (string, error) {
	h, _ := json.Marshal(tokenHeader{"HS256", "JWT"})
	c, e := json.Marshal(tokenClaims{subject, m.issuer, now.Add(m.ttl).Unix(), now.Unix()})
	if e != nil {
		return "", e
	}
	s := encodeBytes(h) + "." + encodeBytes(c)
	return s + "." + encodeBytes(signHMAC(m.secret, []byte(s))), nil
}
func (m tokenManager) verify(token string, now time.Time) (tokenClaims, error) {
	p := strings.Split(token, ".")
	if len(p) != 3 {
		return tokenClaims{}, errors.New("invalid token")
	}
	hb, e := base64.RawURLEncoding.DecodeString(p[0])
	if e != nil {
		return tokenClaims{}, errors.New("invalid token")
	}
	var h tokenHeader
	if json.Unmarshal(hb, &h) != nil || h.Alg != "HS256" || h.Typ != "JWT" {
		return tokenClaims{}, errors.New("invalid token header")
	}
	sig, e := base64.RawURLEncoding.DecodeString(p[2])
	if e != nil || !hmac.Equal(signHMAC(m.secret, []byte(p[0]+"."+p[1])), sig) {
		return tokenClaims{}, errors.New("invalid token signature")
	}
	cb, e := base64.RawURLEncoding.DecodeString(p[1])
	if e != nil {
		return tokenClaims{}, errors.New("invalid token")
	}
	var c tokenClaims
	if json.Unmarshal(cb, &c) != nil || c.Subject == "" || c.Issuer != m.issuer || c.ExpiresAt <= now.Unix() || c.IssuedAt > now.Add(time.Minute).Unix() {
		return tokenClaims{}, errors.New("invalid token claims")
	}
	return c, nil
}
func signHMAC(k, v []byte) []byte { h := hmac.New(sha256.New, k); _, _ = h.Write(v); return h.Sum(nil) }
