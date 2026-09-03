package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// AccessTokenClaims is the payload carried by an access token. Times use the
// NumericDate representation required by JWT (seconds since Unix epoch).
type AccessTokenClaims struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Issuer  string `json:"iss"`
	Issued  int64  `json:"iat"`
	Expiry  int64  `json:"exp"`
	JTI     string `json:"jti"`
}

type accessTokenHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

// AccessTokenValidator validates access tokens issued by AccessTokenIssuer.
// Clock is injected to make expiry checks deterministic for callers and tests.
type AccessTokenValidator struct {
	Secret []byte
	Issuer string
	Clock  Clock
}

// Validate checks the compact JWT format, the signing algorithm, the HMAC
// signature, the issuer, and the required time and identity claims.
func (v AccessTokenValidator) Validate(token string) (AccessTokenClaims, error) {
	var empty AccessTokenClaims
	if len(v.Secret) < 32 || strings.TrimSpace(v.Issuer) == "" {
		return empty, errors.New("invalid access token")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return empty, errors.New("invalid access token")
	}
	codec := base64.RawURLEncoding
	headerBytes, err := codec.DecodeString(parts[0])
	if err != nil {
		return empty, errors.New("invalid access token")
	}
	var header accessTokenHeader
	if err := decodeStrictJSON(headerBytes, &header); err != nil ||
		header.Algorithm != "HS256" || header.Type != "JWT" {
		return empty, errors.New("invalid access token")
	}

	suppliedSignature, err := codec.DecodeString(parts[2])
	if err != nil || len(suppliedSignature) != sha256.Size {
		return empty, errors.New("invalid access token")
	}
	mac := hmac.New(sha256.New, v.Secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	if subtle.ConstantTimeCompare(mac.Sum(nil), suppliedSignature) != 1 {
		return empty, errors.New("invalid access token")
	}

	payload, err := codec.DecodeString(parts[1])
	if err != nil {
		return empty, errors.New("invalid access token")
	}
	var claims AccessTokenClaims
	if err := decodeStrictJSON(payload, &claims); err != nil ||
		strings.TrimSpace(claims.Subject) == "" || strings.TrimSpace(claims.Email) == "" ||
		strings.TrimSpace(claims.Issuer) == "" || strings.TrimSpace(claims.JTI) == "" ||
		claims.Issuer != v.Issuer || claims.Issued <= 0 || claims.Expiry <= 0 ||
		claims.Expiry <= claims.Issued {
		return empty, errors.New("invalid access token")
	}
	if _, err := ValidateEmail(claims.Email); err != nil {
		return empty, errors.New("invalid access token")
	}
	clock := v.Clock
	if clock == nil {
		clock = RealClock{}
	}
	if claims.Expiry <= clock.Now().UTC().Unix() {
		return empty, errors.New("invalid access token")
	}
	return claims, nil
}

// ValidateAccessToken is a convenience wrapper for callers that do not need
// to retain a validator instance.
func ValidateAccessToken(token, issuer string, now time.Time, secret []byte) (AccessTokenClaims, error) {
	return (AccessTokenValidator{
		Secret: secret,
		Issuer: issuer,
		Clock:  ClockFunc(func() time.Time { return now }),
	}).Validate(token)
}

func decodeStrictJSON(data []byte, destination any) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("invalid JSON")
	}
	return nil
}

// AccessTokenIssuer creates JWT-compatible HS256 access tokens.
// The issuer's clock is injected so token lifetimes can be deterministic in
// callers and tests without coupling this code to the global clock.
type AccessTokenIssuer struct {
	Secret []byte
	Issuer string
	TTL    time.Duration
	Clock  Clock
}

// Issue creates a signed access token for subject and email.
func (i AccessTokenIssuer) Issue(subject, email string) (string, AccessTokenClaims, error) {
	if len(i.Secret) < 32 {
		return "", AccessTokenClaims{}, errors.New("HMAC secret must be at least 32 bytes")
	}
	if strings.TrimSpace(i.Issuer) == "" {
		return "", AccessTokenClaims{}, errors.New("issuer must not be empty")
	}
	if strings.TrimSpace(subject) == "" || strings.TrimSpace(email) == "" {
		return "", AccessTokenClaims{}, errors.New("subject and email must not be empty")
	}
	if i.TTL <= 0 {
		return "", AccessTokenClaims{}, errors.New("access token TTL must be positive")
	}
	clock := i.Clock
	if clock == nil {
		clock = RealClock{}
	}
	now := clock.Now().UTC().Truncate(time.Second)
	expiry := now.Add(i.TTL).Unix()
	if expiry <= now.Unix() {
		return "", AccessTokenClaims{}, errors.New("access token TTL must be at least one second")
	}

	jti, err := randomJTI()
	if err != nil {
		return "", AccessTokenClaims{}, fmt.Errorf("generate access token id: %w", err)
	}
	claims := AccessTokenClaims{
		Subject: subject,
		Email:   email,
		Issuer:  i.Issuer,
		Issued:  now.Unix(),
		Expiry:  expiry,
		JTI:     jti,
	}
	header, err := json.Marshal(accessTokenHeader{Algorithm: "HS256", Type: "JWT"})
	if err != nil {
		return "", AccessTokenClaims{}, fmt.Errorf("encode access token header: %w", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", AccessTokenClaims{}, fmt.Errorf("encode access token claims: %w", err)
	}
	codec := base64.RawURLEncoding
	unsigned := codec.EncodeToString(header) + "." + codec.EncodeToString(payload)
	mac := hmac.New(sha256.New, i.Secret)
	_, _ = mac.Write([]byte(unsigned))
	token := unsigned + "." + codec.EncodeToString(mac.Sum(nil))
	return token, claims, nil
}

// CreateAccessToken is a convenience wrapper for issuing an access token.
func CreateAccessToken(subject, email, issuer string, now time.Time, ttl time.Duration, secret []byte) (string, error) {
	token, _, err := (AccessTokenIssuer{
		Secret: secret,
		Issuer: issuer,
		TTL:    ttl,
		Clock:  ClockFunc(func() time.Time { return now }),
	}).Issue(subject, email)
	return token, err
}

func randomJTI() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value[:]), nil
}
