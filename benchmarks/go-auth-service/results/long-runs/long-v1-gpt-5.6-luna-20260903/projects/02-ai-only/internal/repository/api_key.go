package repository

import (
	"encoding/hex"
	"sync"

	"benchmark.local/iam/internal/domain"
)

type APIKeyRepository struct {
	mu     sync.RWMutex
	byID   map[domain.APIKeyID]domain.APIKey
	byHash map[string]domain.APIKeyID
}

func NewAPIKeyRepository() *APIKeyRepository {
	return &APIKeyRepository{byID: make(map[domain.APIKeyID]domain.APIKey), byHash: make(map[string]domain.APIKeyID)}
}

func (r *APIKeyRepository) Create(key domain.APIKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if key.ID == "" || key.OrganizationID == "" || key.OwnerID == "" || key.Name == "" || len(key.KeyHash) == 0 || len(key.Scopes) == 0 {
		return domain.Invalid("API key is incomplete")
	}
	hashKey := hex.EncodeToString(key.KeyHash)
	if _, exists := r.byID[key.ID]; exists {
		return domain.Conflict("API key already exists")
	}
	if _, exists := r.byHash[hashKey]; exists {
		return domain.Conflict("API key already exists")
	}
	key.KeyHash = append([]byte(nil), key.KeyHash...)
	key.Scopes = cloneAPIKeyPermissions(key.Scopes)
	r.byID[key.ID] = key
	r.byHash[hashKey] = key.ID
	return nil
}

func (r *APIKeyRepository) Get(id domain.APIKeyID) (domain.APIKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key, exists := r.byID[id]
	if !exists {
		return domain.APIKey{}, domain.NotFound("API key not found")
	}
	return cloneAPIKey(key), nil
}

func (r *APIKeyRepository) FindByKeyHash(hash []byte) (domain.APIKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, exists := r.byHash[hex.EncodeToString(hash)]
	if !exists {
		return domain.APIKey{}, domain.Unauthorized("API key is invalid")
	}
	key, exists := r.byID[id]
	if !exists {
		return domain.APIKey{}, domain.Unauthorized("API key is invalid")
	}
	return cloneAPIKey(key), nil
}

func (r *APIKeyRepository) ListByOrganization(organizationID domain.OrganizationID) []domain.APIKey {
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

func (r *APIKeyRepository) Revoke(id domain.APIKeyID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key, exists := r.byID[id]
	if !exists {
		return domain.NotFound("API key not found")
	}
	key.Revoked = true
	r.byID[id] = key
	return nil
}

func cloneAPIKey(key domain.APIKey) domain.APIKey {
	key.KeyHash = append([]byte(nil), key.KeyHash...)
	key.Scopes = cloneAPIKeyPermissions(key.Scopes)
	return key
}

func cloneAPIKeyPermissions(permissions map[domain.Permission]bool) map[domain.Permission]bool {
	clone := make(map[domain.Permission]bool, len(permissions))
	for permission, enabled := range permissions {
		clone[permission] = enabled
	}
	return clone
}
