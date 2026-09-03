package domain

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"time"
)

// AccessTokenClaims is the set of claims issued in an access token. Times use
// JWT NumericDate (seconds since the Unix epoch).
type AccessTokenClaims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Iss   string `json:"iss"`
	Iat   int64  `json:"iat"`
	Exp   int64  `json:"exp"`
	JTI   string `json:"jti"`
}

type accessTokenHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// AccessTokenIssuer creates short-lived JWT-compatible HS256 access tokens.
// The clock is injected so token lifetime behavior is deterministic for
// callers that need it to be.
type AccessTokenIssuer struct {
	secret []byte
	issuer string
	ttl    time.Duration
	clock  Clock
}

// AccessTokenValidator validates tokens issued by AccessTokenIssuer. It does
// not trust any identity or authorization data supplied by the caller: all
// claims are taken from the verified token.
type AccessTokenValidator struct {
	secret []byte
	issuer string
	clock  Clock
}

// NewAccessTokenValidator constructs a validator with the supplied signing
// key and expected issuer. The key is copied for the same reason as in the
// issuer: callers cannot mutate validator state after construction.
func NewAccessTokenValidator(secret []byte, issuer string, clock Clock) (*AccessTokenValidator, error) {
	if len(secret) == 0 || issuer == "" {
		return nil, NewError(ErrorInvalid, "invalid access token settings")
	}
	if clock == nil {
		clock = RealClock{}
	}
	return &AccessTokenValidator{
		secret: append([]byte(nil), secret...),
		issuer: issuer,
		clock:  clock,
	}, nil
}

// Validate verifies the token format, algorithm, signature, issuer and
// expiration. Every failure is returned as the same safe unauthorized error;
// token material and claim contents are deliberately not included in it.
func (v *AccessTokenValidator) Validate(token string) (AccessTokenClaims, error) {
	if v == nil || len(v.secret) == 0 || v.issuer == "" || v.clock == nil {
		return AccessTokenClaims{}, NewError(ErrorUnauthorized, "invalid access token")
	}

	parts := splitAccessToken(token)
	if parts == nil {
		return AccessTokenClaims{}, NewError(ErrorUnauthorized, "invalid access token")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return AccessTokenClaims{}, NewError(ErrorUnauthorized, "invalid access token")
	}
	var header accessTokenHeader
	if err := decodeStrictJSON(headerBytes, &header); err != nil || header.Alg != "HS256" {
		return AccessTokenClaims{}, NewError(ErrorUnauthorized, "invalid access token")
	}

	suppliedSignature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(suppliedSignature) != sha256.Size {
		return AccessTokenClaims{}, NewError(ErrorUnauthorized, "invalid access token")
	}
	mac := hmac.New(sha256.New, v.secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	if subtle.ConstantTimeCompare(mac.Sum(nil), suppliedSignature) == 0 {
		return AccessTokenClaims{}, NewError(ErrorUnauthorized, "invalid access token")
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return AccessTokenClaims{}, NewError(ErrorUnauthorized, "invalid access token")
	}
	var claims AccessTokenClaims
	now := v.clock.Now().Unix()
	if err := decodeStrictJSON(claimsBytes, &claims); err != nil ||
		claims.Sub == "" || claims.Email == "" || claims.Iss != v.issuer || claims.JTI == "" ||
		claims.Exp <= claims.Iat || claims.Exp <= now || claims.Iat > now {
		return AccessTokenClaims{}, NewError(ErrorUnauthorized, "invalid access token")
	}
	if _, err := ValidateEmail(claims.Email); err != nil {
		return AccessTokenClaims{}, NewError(ErrorUnauthorized, "invalid access token")
	}
	return claims, nil
}

// ValidateAccessToken is the stateless convenience form of validation.
func ValidateAccessToken(secret []byte, issuer string, now time.Time, token string) (AccessTokenClaims, error) {
	validator, err := NewAccessTokenValidator(secret, issuer, fixedClock{now: now})
	if err != nil {
		return AccessTokenClaims{}, NewError(ErrorUnauthorized, "invalid access token")
	}
	return validator.Validate(token)
}

func splitAccessToken(token string) *[3]string {
	first := -1
	second := -1
	for index, character := range token {
		if character != '.' {
			continue
		}
		if first < 0 {
			first = index
		} else {
			second = index
			break
		}
	}
	if first <= 0 || second <= first+1 || second == len(token)-1 ||
		indexOfByte(token[second+1:], '.') >= 0 {
		return nil
	}
	return &[3]string{token[:first], token[first+1 : second], token[second+1:]}
}

func indexOfByte(value string, target byte) int {
	for index := 0; index < len(value); index++ {
		if value[index] == target {
			return index
		}
	}
	return -1
}

func decodeStrictJSON(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("invalid json")
	}
	return nil
}

// NewAccessTokenIssuer constructs an issuer with the supplied signing key and
// token settings. The key is copied so the issuer does not retain caller-owned
// mutable storage.
func NewAccessTokenIssuer(secret []byte, issuer string, ttl time.Duration, clock Clock) (*AccessTokenIssuer, error) {
	if len(secret) == 0 || issuer == "" || ttl <= 0 {
		return nil, NewError(ErrorInvalid, "invalid access token settings")
	}
	if clock == nil {
		clock = RealClock{}
	}
	return &AccessTokenIssuer{
		secret: append([]byte(nil), secret...),
		issuer: issuer,
		ttl:    ttl,
		clock:  clock,
	}, nil
}

// Create issues an access token for the supplied user. Roles and permissions
// are intentionally absent: authorization must use current membership state.
func (i *AccessTokenIssuer) Create(user User) (string, error) {
	if i == nil || len(i.secret) == 0 || i.issuer == "" || i.ttl <= 0 || i.clock == nil {
		return "", NewError(ErrorInvalid, "invalid access token settings")
	}
	if user.ID == "" || user.Email == "" {
		return "", NewError(ErrorInvalid, "user identity is required")
	}

	now := i.clock.Now()
	issuedAt := now.Unix()
	expiresAt := now.Add(i.ttl).Unix()
	jti, err := NewID()
	if err != nil {
		return "", errors.New("could not create access token")
	}
	claims := AccessTokenClaims{
		Sub:   string(user.ID),
		Email: user.Email,
		Iss:   i.issuer,
		Iat:   issuedAt,
		Exp:   expiresAt,
		JTI:   string(jti),
	}
	return signAccessToken(i.secret, accessTokenHeader{Alg: "HS256", Typ: "JWT"}, claims)
}

// CreateAccessToken is the stateless convenience form of AccessTokenIssuer.
func CreateAccessToken(secret []byte, issuer string, now time.Time, ttl time.Duration, user User) (string, error) {
	issuerService, err := NewAccessTokenIssuer(secret, issuer, ttl, fixedClock{now: now})
	if err != nil {
		return "", err
	}
	return issuerService.Create(user)
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func signAccessToken(secret []byte, header accessTokenHeader, claims AccessTokenClaims) (string, error) {
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", errors.New("could not encode access token header")
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", errors.New("could not encode access token claims")
	}
	encode := base64.RawURLEncoding.EncodeToString
	unsigned := encode(headerJSON) + "." + encode(claimsJSON)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + encode(mac.Sum(nil)), nil
}
