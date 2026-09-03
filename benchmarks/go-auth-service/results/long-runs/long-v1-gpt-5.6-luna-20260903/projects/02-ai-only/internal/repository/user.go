// Package repository contains concurrent in-memory persistence adapters.
package repository

import (
	"sync"

	"benchmark.local/iam/internal/domain"
)

// UserRepository stores users by ID and maintains a unique normalized email
// index. All access to both maps is synchronized by mu.
type UserRepository struct {
	mu      sync.RWMutex
	byID    map[domain.UserID]domain.User
	byEmail map[string]domain.UserID
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		byID:    make(map[domain.UserID]domain.User),
		byEmail: make(map[string]domain.UserID),
	}
}

// NormalizeEmail defines the canonical key used for uniqueness and lookup.
func NormalizeEmail(email string) string {
	normalized, err := domain.NormalizeEmail(email)
	if err != nil {
		return ""
	}
	return normalized
}

func (r *UserRepository) Create(user domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if user.ID == "" {
		return domain.Invalid("user id must not be empty")
	}
	if NormalizeEmail(string(user.Email)) == "" {
		return domain.Invalid("email must not be empty")
	}
	if _, exists := r.byID[user.ID]; exists {
		return domain.Conflict("user already exists")
	}
	emailKey := NormalizeEmail(string(user.Email))
	if _, exists := r.byEmail[emailKey]; exists {
		return domain.Conflict("email already exists")
	}

	r.byID[user.ID] = cloneUser(user)
	r.byEmail[emailKey] = user.ID
	return nil
}

func (r *UserRepository) Get(id domain.UserID) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.byID[id]
	if !exists {
		return domain.User{}, domain.NotFound("user not found")
	}
	return cloneUser(user), nil
}

func (r *UserRepository) FindByEmail(email domain.Email) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, exists := r.byEmail[NormalizeEmail(string(email))]
	if !exists {
		return domain.User{}, domain.NotFound("user not found")
	}
	return cloneUser(r.byID[id]), nil
}

// Update replaces a user while preserving the email uniqueness invariant.
func (r *UserRepository) Update(user domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	old, exists := r.byID[user.ID]
	if !exists {
		return domain.NotFound("user not found")
	}
	if NormalizeEmail(string(user.Email)) == "" {
		return domain.Invalid("email must not be empty")
	}
	newEmailKey := NormalizeEmail(string(user.Email))
	oldEmailKey := NormalizeEmail(string(old.Email))
	if owner, occupied := r.byEmail[newEmailKey]; occupied && owner != user.ID {
		return domain.Conflict("email already exists")
	}
	if oldEmailKey != newEmailKey {
		delete(r.byEmail, oldEmailKey)
		r.byEmail[newEmailKey] = user.ID
	}
	r.byID[user.ID] = cloneUser(user)
	return nil
}

func (r *UserRepository) Delete(id domain.UserID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, exists := r.byID[id]
	if !exists {
		return domain.NotFound("user not found")
	}
	delete(r.byID, id)
	delete(r.byEmail, NormalizeEmail(string(user.Email)))
	return nil
}

func cloneUser(user domain.User) domain.User {
	user.PasswordMaterial.Salt = append([]byte(nil), user.PasswordMaterial.Salt...)
	user.PasswordMaterial.Hash = append([]byte(nil), user.PasswordMaterial.Hash...)
	return user
}
