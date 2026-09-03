package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"
)

// APIKey is an organization-scoped credential. Hash is never serialized.
type APIKey struct {
	ID             APIKeyID       `json:"id"`
	OrganizationID OrganizationID `json:"organization_id"`
	CreatedBy      UserID         `json:"created_by"`
	Name           string         `json:"name"`
	Scopes         []Permission   `json:"scopes"`
	Hash           []byte         `json:"-"`
	CreatedAt      time.Time      `json:"created_at"`
	RevokedAt      *time.Time     `json:"revoked_at,omitempty"`
}

func NewAPIKey() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "ak_" + base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func HashAPIKey(key string) []byte { sum := sha256.Sum256([]byte(key)); return sum[:] }

func ValidAPIKey(key string) bool {
	if len(key) < 20 || len(key) > 100 || len(key) < 3 || key[:3] != "ak_" {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(key[3:])
	return err == nil
}
