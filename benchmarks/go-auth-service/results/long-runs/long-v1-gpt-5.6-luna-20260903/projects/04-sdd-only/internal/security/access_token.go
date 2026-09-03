package security

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"benchmark.local/iam/internal/domain"
)

const accessTokenAlgorithm = "HS256"

type jwtHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

// AccessTokenClaims are the claims emitted in an access token. Numeric time
// claims follow the JWT NumericDate convention.
type AccessTokenClaims struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Issuer  string `json:"iss"`
	Issued  int64  `json:"iat"`
	Expires int64  `json:"exp"`
	JWTID   string `json:"jti"`
}

// AccessTokenService creates short-lived HS256 access tokens.
type AccessTokenService struct {
	secret []byte
	issuer string
	clock  domain.Clock
	id     domain.IDGenerator
	ttl    time.Duration
}

func NewAccessTokenService(secret []byte, issuer string, clock domain.Clock, id domain.IDGenerator, ttl time.Duration) (*AccessTokenService, error) {
	if len(secret) < 32 {
		return nil, errors.New("access token secret must be at least 32 bytes")
	}
	if strings.TrimSpace(issuer) == "" {
		return nil, errors.New("access token issuer is required")
	}
	if clock == nil || id == nil {
		return nil, errors.New("access token dependencies are required")
	}
	if ttl <= 0 {
		return nil, errors.New("access token TTL must be positive")
	}
	return &AccessTokenService{
		secret: append([]byte(nil), secret...),
		issuer: issuer,
		clock:  clock,
		id:     id,
		ttl:    ttl,
	}, nil
}

// Issue creates a three-part JWT-compatible token for a user.
func (s *AccessTokenService) Issue(user domain.User) (string, error) {
	if user.ID == "" || user.Email == "" {
		return "", errors.New("access token subject is required")
	}
	jti, err := s.id.NewID()
	if err != nil {
		return "", errors.New("generate access token ID")
	}
	now := s.clock.Now().UTC()
	claims := AccessTokenClaims{
		Subject: user.ID.String(),
		Email:   user.Email,
		Issuer:  s.issuer,
		Issued:  now.Unix(),
		Expires: now.Add(s.ttl).Unix(),
		JWTID:   jti.String(),
	}
	header, err := json.Marshal(jwtHeader{Algorithm: accessTokenAlgorithm, Type: "JWT"})
	if err != nil {
		return "", errors.New("encode access token header")
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", errors.New("encode access token claims")
	}
	encodedHeader := base64.RawURLEncoding.EncodeToString(header)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	unsigned := encodedHeader + "." + encodedPayload
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(unsigned))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return unsigned + "." + signature, nil
}

// Validate verifies the format, algorithm, signature, issuer, and expiry of
// an access token. All malformed token cases return the same generic error.
func (s *AccessTokenService) Validate(token string) (AccessTokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return AccessTokenClaims{}, errInvalidAccessToken
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return AccessTokenClaims{}, errInvalidAccessToken
	}
	var header jwtHeader
	if err := decodeTokenJSON(headerBytes, &header); err != nil || header.Algorithm != accessTokenAlgorithm {
		return AccessTokenClaims{}, errInvalidAccessToken
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return AccessTokenClaims{}, errInvalidAccessToken
	}
	var claims AccessTokenClaims
	if err := decodeTokenJSON(payloadBytes, &claims); err != nil || !validClaims(claims, s.issuer) {
		return AccessTokenClaims{}, errInvalidAccessToken
	}
	providedSignature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(providedSignature) != sha256.Size {
		return AccessTokenClaims{}, errInvalidAccessToken
	}
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	expectedSignature := mac.Sum(nil)
	if subtle.ConstantTimeCompare(expectedSignature, providedSignature) != 1 {
		return AccessTokenClaims{}, errInvalidAccessToken
	}
	if s.clock.Now().UTC().Unix() >= claims.Expires {
		return AccessTokenClaims{}, errInvalidAccessToken
	}
	return claims, nil
}

var errInvalidAccessToken = errors.New("invalid access token")

func decodeTokenJSON(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return errors.New("multiple JSON values")
}

func validClaims(claims AccessTokenClaims, issuer string) bool {
	return claims.Subject != "" && claims.Email != "" && claims.Issuer == issuer &&
		claims.JWTID != "" && claims.Issued > 0 && claims.Expires > claims.Issued
}
