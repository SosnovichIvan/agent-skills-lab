package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var rawBase64 = base64.RawURLEncoding

type tokenHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type tokenClaims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Iss   string `json:"iss"`
	Iat   int64  `json:"iat"`
	Exp   int64  `json:"exp"`
	JTI   string `json:"jti"`
}

type TokenManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

func NewTokenManager(secret []byte, issuer string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: append([]byte(nil), secret...), issuer: issuer, ttl: ttl}
}

func (m *TokenManager) Create(user User) (string, error) {
	jtiBytes := make([]byte, 16)
	if _, err := rand.Read(jtiBytes); err != nil {
		return "", err
	}
	now := time.Now().UTC()
	claims := tokenClaims{Sub: user.ID, Email: user.Email, Iss: m.issuer, Iat: now.Unix(), Exp: now.Add(m.ttl).Unix(), JTI: rawBase64.EncodeToString(jtiBytes)}
	header, err := json.Marshal(tokenHeader{Alg: "HS256", Typ: "JWT"})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	unsigned := rawBase64.EncodeToString(header) + "." + rawBase64.EncodeToString(payload)
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + rawBase64.EncodeToString(mac.Sum(nil)), nil
}

func (m *TokenManager) Validate(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return Claims{}, errors.New("invalid token format")
	}
	headerBytes, err := rawBase64.DecodeString(parts[0])
	if err != nil {
		return Claims{}, errors.New("invalid token header")
	}
	var header tokenHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil || header.Alg != "HS256" || header.Typ != "JWT" {
		return Claims{}, errors.New("unsupported token header")
	}
	provided, err := rawBase64.DecodeString(parts[2])
	if err != nil || len(provided) != sha256.Size {
		return Claims{}, errors.New("invalid token signature")
	}
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return Claims{}, errors.New("invalid token signature")
	}
	payloadBytes, err := rawBase64.DecodeString(parts[1])
	if err != nil {
		return Claims{}, errors.New("invalid token payload")
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return Claims{}, errors.New("invalid token payload")
	}
	var c tokenClaims
	if err := decodeClaims(payload, &c); err != nil {
		return Claims{}, err
	}
	now := time.Now().Unix()
	if c.Sub == "" || c.Email == "" || c.Iss == "" || c.JTI == "" || c.Iss != m.issuer || c.Exp <= now || c.Iat > now+60 || c.Exp <= c.Iat {
		return Claims{}, errors.New("invalid token claims")
	}
	return Claims{Subject: c.Sub, Email: c.Email, Issuer: c.Iss, Issued: time.Unix(c.Iat, 0), Expires: time.Unix(c.Exp, 0), JTI: c.JTI}, nil
}

func decodeClaims(raw map[string]json.RawMessage, c *tokenClaims) error {
	required := []string{"sub", "email", "iss", "iat", "exp", "jti"}
	for _, key := range required {
		if _, ok := raw[key]; !ok {
			return errors.New("missing token claim: " + key)
		}
	}
	if err := json.Unmarshal(raw["sub"], &c.Sub); err != nil {
		return errors.New("invalid sub claim")
	}
	if err := json.Unmarshal(raw["email"], &c.Email); err != nil {
		return errors.New("invalid email claim")
	}
	if err := json.Unmarshal(raw["iss"], &c.Iss); err != nil {
		return errors.New("invalid issuer claim")
	}
	if err := json.Unmarshal(raw["jti"], &c.JTI); err != nil {
		return errors.New("invalid jti claim")
	}
	for key, target := range map[string]*int64{"iat": &c.Iat, "exp": &c.Exp} {
		var number json.Number
		decoder := json.NewDecoder(strings.NewReader(string(raw[key])))
		decoder.UseNumber()
		if err := decoder.Decode(&number); err != nil {
			return fmt.Errorf("invalid %s claim", key)
		}
		value, err := strconv.ParseInt(number.String(), 10, 64)
		if err != nil {
			return fmt.Errorf("invalid %s claim", key)
		}
		*target = value
	}
	return nil
}
