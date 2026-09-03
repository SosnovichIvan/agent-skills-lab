// Package repository contains the in-memory persistence used by the IAM
// application.
package repository

import (
	"sync"

	"benchmark.local/iam/internal/domain"
)

// UserRepository is a concurrency-safe in-memory store for users. The email
// index contains normalized addresses and is maintained under the same lock
// as users, so an email cannot be claimed by two users.
type UserRepository struct {
	mu         sync.RWMutex
	users      map[domain.UserID]domain.User
	emailIndex map[string]domain.UserID
}

// NewUserRepository creates an empty user repository.
func NewUserRepository() *UserRepository {
	return &UserRepository{
		users:      make(map[domain.UserID]domain.User),
		emailIndex: make(map[string]domain.UserID),
	}
}

// Create stores a user and reserves its normalized email address.
func (r *UserRepository) Create(user domain.User) error {
	if r == nil {
		return domain.NewError(domain.ErrorInvalid, "user repository is nil")
	}
	if user.ID == "" {
		return domain.NewError(domain.ErrorInvalid, "user id and email are required")
	}
	key, err := domain.ValidateEmail(user.Email)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.users[user.ID]; exists {
		return domain.NewError(domain.ErrorConflict, "user already exists")
	}
	if _, exists := r.emailIndex[key]; exists {
		return domain.NewError(domain.ErrorConflict, "email already exists")
	}
	// Store both the aggregate and its credential byte slices detached from the
	// caller. This keeps later caller mutations from bypassing repository lock.
	user.Email = key
	r.users[user.ID] = cloneUser(user)
	r.emailIndex[key] = user.ID
	return nil
}

// GetByID returns a detached copy of the user with the supplied ID.
func (r *UserRepository) GetByID(id domain.UserID) (domain.User, error) {
	if r == nil {
		return domain.User{}, domain.ErrNotFound
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	user, ok := r.users[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return cloneUser(user), nil
}

// GetByEmail returns a detached copy of the user matching the normalized
// email address. Lookup is case-insensitive and ignores surrounding spaces.
func (r *UserRepository) GetByEmail(email string) (domain.User, error) {
	if r == nil {
		return domain.User{}, domain.ErrNotFound
	}
	key := domain.NormalizeEmail(email)
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.emailIndex[key]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return cloneUser(r.users[id]), nil
}

// Update replaces an existing user and, atomically, updates its email index.
// A changed email must not already belong to another user.
func (r *UserRepository) Update(user domain.User) error {
	if r == nil {
		return domain.NewError(domain.ErrorInvalid, "user repository is nil")
	}
	if user.ID == "" {
		return domain.NewError(domain.ErrorInvalid, "user id and email are required")
	}
	key, err := domain.ValidateEmail(user.Email)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	previous, exists := r.users[user.ID]
	if !exists {
		return domain.ErrNotFound
	}
	if owner, claimed := r.emailIndex[key]; claimed && owner != user.ID {
		return domain.NewError(domain.ErrorConflict, "email already exists")
	}
	if oldKey := domain.NormalizeEmail(previous.Email); oldKey != key {
		delete(r.emailIndex, oldKey)
	}
	user.Email = key
	r.users[user.ID] = cloneUser(user)
	r.emailIndex[key] = user.ID
	return nil
}

// Delete removes a user and releases its email address.
func (r *UserRepository) Delete(id domain.UserID) error {
	if r == nil {
		return domain.ErrNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.users[id]
	if !ok {
		return domain.ErrNotFound
	}
	delete(r.users, id)
	delete(r.emailIndex, domain.NormalizeEmail(user.Email))
	return nil
}

func cloneUser(user domain.User) domain.User {
	user.PasswordMaterial = user.PasswordMaterial.Clone()
	return user
}
