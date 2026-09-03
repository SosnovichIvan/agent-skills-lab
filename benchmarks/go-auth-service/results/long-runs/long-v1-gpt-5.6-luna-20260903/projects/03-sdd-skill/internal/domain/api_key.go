package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"time"
)

const APIKeySize = 32

// APIKey contains only non-secret metadata. SecretHash is persisted instead
// of the opaque value, which is returned to a caller only when the key is made.
type APIKey struct {
	ID             APIKeyID       `json:"id"`
	OrganizationID OrganizationID `json:"organization_id"`
	UserID         UserID         `json:"created_by"`
	Name           string         `json:"name"`
	Scopes         []Permission   `json:"scopes"`
	SecretHash     []byte         `json:"-"`
	CreatedAt      time.Time      `json:"created_at"`
	RevokedAt      *time.Time     `json:"revoked_at,omitempty"`
}

func (k APIKey) MarshalJSON() ([]byte, error) {
	type publicKey APIKey
	k.SecretHash = nil
	k.Scopes = append([]Permission(nil), k.Scopes...)
	return json.Marshal(publicKey(k))
}

func NewAPIKey() (string, error) {
	b := make([]byte, APIKeySize)
	if _, err := rand.Read(b); err != nil {
		return "", NewError(ErrorInvalid, "could not create api key")
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func HashAPIKey(raw string) []byte {
	digest := sha256.Sum256([]byte(raw))
	return append([]byte(nil), digest[:]...)
}
