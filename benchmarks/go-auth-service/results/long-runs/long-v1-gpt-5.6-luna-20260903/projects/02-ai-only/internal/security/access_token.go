package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"benchmark.local/iam/internal/domain"
)

type AccessTokenIssuer struct {
	secret []byte
	issuer string
	ttl    time.Duration
	clock  domain.Clock
}

type AccessTokenClaims struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Issuer  string `json:"iss"`
	Issued  int64  `json:"iat"`
	Expires int64  `json:"exp"`
	JTI     string `json:"jti"`
}

func NewAccessTokenIssuer(secret []byte, issuer string, ttl time.Duration, clock domain.Clock) (*AccessTokenIssuer, error) {
	if len(secret) < 32 {
		return nil, errors.New("access token secret must be at least 32 bytes")
	}
	if issuer == "" || ttl <= 0 || clock == nil {
		return nil, errors.New("access token issuer configuration is invalid")
	}
	return &AccessTokenIssuer{
		secret: append([]byte(nil), secret...),
		issuer: issuer,
		ttl:    ttl,
		clock:  clock,
	}, nil
}

func (i *AccessTokenIssuer) Issue(userID domain.UserID, email domain.Email) (string, error) {
	if i == nil || i.clock == nil {
		return "", errors.New("access token issuer is not configured")
	}
	var rawJTI [16]byte
	if _, err := rand.Read(rawJTI[:]); err != nil {
		return "", errors.New("access token id generation failed")
	}
	now := i.clock.Now().UTC()
	claims := AccessTokenClaims{
		Subject: string(userID),
		Email:   string(email),
		Issuer:  i.issuer,
		Issued:  now.Unix(),
		Expires: now.Add(i.ttl).Unix(),
		JTI:     hex.EncodeToString(rawJTI[:]),
	}
	header, err := json.Marshal(struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}{Algorithm: "HS256", Type: "JWT"})
	if err != nil {
		return "", fmt.Errorf("encode access token header: %w", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("encode access token claims: %w", err)
	}
	codec := base64.RawURLEncoding
	signingInput := codec.EncodeToString(header) + "." + codec.EncodeToString(payload)
	mac := hmac.New(sha256.New, i.secret)
	_, _ = mac.Write([]byte(signingInput))
	return signingInput + "." + codec.EncodeToString(mac.Sum(nil)), nil
}

// Validate verifies an access token's structure, algorithm, signature and
// registered claims. The returned claims are safe to use only after all these
// checks have succeeded.
func (i *AccessTokenIssuer) Validate(token string) (AccessTokenClaims, error) {
	var zero AccessTokenClaims
	if i == nil || len(i.secret) == 0 || i.clock == nil {
		return zero, errors.New("access token validator is not configured")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return zero, errors.New("access token format is invalid")
	}
	codec := base64.RawURLEncoding
	headerBytes, err := codec.DecodeString(parts[0])
	if err != nil {
		return zero, errors.New("access token header is invalid")
	}
	payloadBytes, err := codec.DecodeString(parts[1])
	if err != nil {
		return zero, errors.New("access token claims are invalid")
	}
	providedSignature, err := codec.DecodeString(parts[2])
	if err != nil {
		return zero, errors.New("access token signature is invalid")
	}

	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}
	if err := decodeStrictJSON(headerBytes, &header); err != nil || header.Algorithm != "HS256" || header.Type != "JWT" {
		return zero, errors.New("access token header is invalid")
	}

	var claims AccessTokenClaims
	if err := decodeStrictJSON(payloadBytes, &claims); err != nil {
		return zero, errors.New("access token claims are invalid")
	}
	if claims.Subject == "" || claims.Email == "" || claims.Issuer == "" || claims.JTI == "" || claims.Issued <= 0 || claims.Expires <= 0 {
		return zero, errors.New("access token claims are incomplete")
	}
	if claims.Issuer != i.issuer {
		return zero, errors.New("access token issuer is invalid")
	}
	if claims.Expires <= claims.Issued || i.clock.Now().Unix() >= claims.Expires {
		return zero, errors.New("access token is expired")
	}

	mac := hmac.New(sha256.New, i.secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	expectedSignature := mac.Sum(nil)
	if subtle.ConstantTimeCompare(expectedSignature, providedSignature) != 1 {
		return zero, errors.New("access token signature is invalid")
	}
	return claims, nil
}

func decodeStrictJSON(data []byte, destination any) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}
