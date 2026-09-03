package repository

import (
	"encoding/hex"
	"strings"
	"sync"

	"benchmark.local/iam/internal/domain"
)

type APIKeyStore interface {
	Create(key domain.APIKey) error
	ListByOrganization(organizationID domain.ID) []domain.APIKey
	Revoke(organizationID, keyID domain.ID) error
}

type APIKeyRepository struct {
	mu     sync.RWMutex
	byID   map[domain.ID]domain.APIKey
	byHash map[string]domain.ID
}

func NewAPIKeyRepository() *APIKeyRepository {
	return &APIKeyRepository{byID: make(map[domain.ID]domain.APIKey), byHash: make(map[string]domain.ID)}
}

func (r *APIKeyRepository) Create(key domain.APIKey) error {
	if key.ID == "" || key.OrganizationID == "" || strings.TrimSpace(key.Name) == "" || len(key.KeyHash) != 32 || len(key.Scopes) == 0 {
		return domain.NewError(domain.KindInvalid, "API key is invalid")
	}
	for _, scope := range key.Scopes {
		if !knownPermission(scope) {
			return domain.NewError(domain.KindInvalid, "API key scope is invalid")
		}
	}
	hashKey := hex.EncodeToString(key.KeyHash)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byID[key.ID]; exists {
		return domain.NewError(domain.KindConflict, "API key already exists")
	}
	if _, exists := r.byHash[hashKey]; exists {
		return domain.NewError(domain.KindConflict, "API key already exists")
	}
	r.byID[key.ID] = cloneAPIKey(key)
	r.byHash[hashKey] = key.ID
	return nil
}

func (r *APIKeyRepository) ListByOrganization(organizationID domain.ID) []domain.APIKey {
	r.mu.RLock()
	defer r.mu.RUnlock()
	keys := make([]domain.APIKey, 0)
	for _, key := range r.byID {
		if key.OrganizationID == organizationID {
			keys = append(keys, cloneAPIKey(key))
		}
	}
	return keys
}

func (r *APIKeyRepository) Revoke(organizationID, keyID domain.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key, exists := r.byID[keyID]
	if !exists || key.OrganizationID != organizationID {
		return domain.NewError(domain.KindNotFound, "API key not found")
	}
	key.Revoked = true
	r.byID[keyID] = cloneAPIKey(key)
	return nil
}

func cloneAPIKey(key domain.APIKey) domain.APIKey {
	key.KeyHash = append([]byte(nil), key.KeyHash...)
	key.Scopes = append([]domain.Permission(nil), key.Scopes...)
	return key
}
