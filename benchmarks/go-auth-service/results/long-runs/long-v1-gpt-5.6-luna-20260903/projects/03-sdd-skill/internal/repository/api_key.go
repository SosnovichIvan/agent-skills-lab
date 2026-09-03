package repository

import (
	"crypto/sha256"
	"sync"
	"time"

	"benchmark.local/iam/internal/domain"
)

// APIKeyRepository is a concurrency-safe store indexed by secret digest.
type APIKeyRepository struct {
	mu     sync.RWMutex
	keys   map[domain.APIKeyID]domain.APIKey
	byHash map[[sha256.Size]byte]domain.APIKeyID
}

func NewAPIKeyRepository() *APIKeyRepository {
	return &APIKeyRepository{keys: make(map[domain.APIKeyID]domain.APIKey), byHash: make(map[[sha256.Size]byte]domain.APIKeyID)}
}

func cloneAPIKey(k domain.APIKey) domain.APIKey {
	k.SecretHash = append([]byte(nil), k.SecretHash...)
	k.Scopes = append([]domain.Permission(nil), k.Scopes...)
	return k
}

func (r *APIKeyRepository) Create(k domain.APIKey) error {
	if r == nil || k.ID == "" || k.OrganizationID == "" || k.UserID == "" || k.Name == "" || len(k.SecretHash) != sha256.Size || len(k.Scopes) == 0 {
		return domain.NewError(domain.ErrorInvalid, "invalid api key")
	}
	var hash [sha256.Size]byte
	copy(hash[:], k.SecretHash)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.keys[k.ID]; ok {
		return domain.ErrConflict
	}
	if _, ok := r.byHash[hash]; ok {
		return domain.ErrConflict
	}
	r.keys[k.ID] = cloneAPIKey(k)
	r.byHash[hash] = k.ID
	return nil
}

func (r *APIKeyRepository) GetByID(id domain.APIKeyID) (domain.APIKey, error) {
	if r == nil {
		return domain.APIKey{}, domain.ErrNotFound
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	k, ok := r.keys[id]
	if !ok {
		return domain.APIKey{}, domain.ErrNotFound
	}
	return cloneAPIKey(k), nil
}

func (r *APIKeyRepository) ListByOrganization(orgID domain.OrganizationID) []domain.APIKey {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]domain.APIKey, 0)
	for _, k := range r.keys {
		if k.OrganizationID == orgID {
			result = append(result, cloneAPIKey(k))
		}
	}
	return result
}

// Authenticate resolves an opaque key without exposing its secret.
func (r *APIKeyRepository) Authenticate(raw string) (domain.APIKey, error) {
	if r == nil || raw == "" {
		return domain.APIKey{}, domain.ErrUnauthorized
	}
	var hash [sha256.Size]byte
	copy(hash[:], domain.HashAPIKey(raw))
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byHash[hash]
	if !ok {
		return domain.APIKey{}, domain.ErrUnauthorized
	}
	k := r.keys[id]
	if k.RevokedAt != nil {
		return domain.APIKey{}, domain.ErrUnauthorized
	}
	return cloneAPIKey(k), nil
}

func (r *APIKeyRepository) Revoke(id domain.APIKeyID, now time.Time) error {
	if r == nil {
		return domain.ErrNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	k, ok := r.keys[id]
	if !ok {
		return domain.ErrNotFound
	}
	if k.RevokedAt == nil {
		k.RevokedAt = &now
		r.keys[id] = k
	}
	return nil
}
