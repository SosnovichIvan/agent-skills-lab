package repository

import (
	"crypto/subtle"
	"encoding/hex"
	"sort"
	"sync"
	"time"

	"benchmark.local/iam/internal/domain"
)

type APIKeyRepository struct {
	mu     sync.RWMutex
	keys   map[domain.APIKeyID]domain.APIKey
	byHash map[string]domain.APIKeyID
}

func NewAPIKeyRepository() *APIKeyRepository {
	return &APIKeyRepository{keys: make(map[domain.APIKeyID]domain.APIKey), byHash: make(map[string]domain.APIKeyID)}
}

func (r *APIKeyRepository) Create(key domain.APIKey) error {
	if key.ID == "" || key.OrganizationID == "" || key.CreatedBy == "" || key.Name == "" || len(key.Hash) != 32 || len(key.Scopes) == 0 {
		return domain.NewInvalid("invalid api key")
	}
	seen := make(map[domain.Permission]bool)
	for _, scope := range key.Scopes {
		if !domain.ValidPermission(scope) || seen[scope] {
			return domain.NewInvalid("invalid api key scope")
		}
		seen[scope] = true
	}
	hash := hex.EncodeToString(key.Hash)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.keys[key.ID]; ok || r.byHash[hash] != "" {
		return domain.NewConflict("api key already exists")
	}
	key.Hash = append([]byte(nil), key.Hash...)
	key.Scopes = append([]domain.Permission(nil), key.Scopes...)
	r.keys[key.ID] = key
	r.byHash[hash] = key.ID
	return nil
}

func cloneAPIKey(key domain.APIKey) domain.APIKey {
	key.Hash = append([]byte(nil), key.Hash...)
	key.Scopes = append([]domain.Permission(nil), key.Scopes...)
	return key
}

func (r *APIKeyRepository) GetByKey(hash []byte) (domain.APIKey, error) {
	r.mu.RLock()
	id := r.byHash[hex.EncodeToString(hash)]
	key, ok := r.keys[id]
	r.mu.RUnlock()
	if !ok || subtle.ConstantTimeCompare(key.Hash, hash) != 1 || key.RevokedAt != nil {
		return domain.APIKey{}, domain.NewNotFound("api key not found")
	}
	return cloneAPIKey(key), nil
}

func (r *APIKeyRepository) ListByOrganization(orgID domain.OrganizationID) []domain.APIKey {
	r.mu.RLock()
	result := make([]domain.APIKey, 0)
	for _, key := range r.keys {
		if key.OrganizationID == orgID {
			result = append(result, cloneAPIKey(key))
		}
	}
	r.mu.RUnlock()
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result
}

func (r *APIKeyRepository) Revoke(orgID domain.OrganizationID, id domain.APIKeyID, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key, ok := r.keys[id]
	if !ok || key.OrganizationID != orgID {
		return domain.NewNotFound("api key not found")
	}
	if key.RevokedAt != nil {
		return domain.NewConflict("api key already revoked")
	}
	key.RevokedAt = &now
	r.keys[id] = key
	return nil
}
