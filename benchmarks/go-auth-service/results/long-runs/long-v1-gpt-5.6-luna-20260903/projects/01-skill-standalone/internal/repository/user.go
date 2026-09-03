// Package repository contains concurrency-safe in-memory persistence.
package repository

import (
	"sync"

	"benchmark.local/iam/internal/domain"
)

// UserRepository stores users and maintains a unique, normalized email index.
// Both maps are protected by mu; callers never receive a reference to the
// repository's stored PasswordMaterial byte slices.
type UserRepository struct {
	mu      sync.RWMutex
	users   map[domain.UserID]domain.User
	byEmail map[string]domain.UserID
}

// NewUserRepository creates an empty in-memory user repository.
func NewUserRepository() *UserRepository {
	return &UserRepository{
		users:   make(map[domain.UserID]domain.User),
		byEmail: make(map[string]domain.UserID),
	}
}

// NormalizeEmail returns the key used by the repository's email index.
func NormalizeEmail(email string) string {
	return domain.NormalizeEmail(email)
}

// Create stores a new user. The user's ID must be unique and its normalized
// email must not already belong to another user.
func (r *UserRepository) Create(user domain.User) error {
	normalizedEmail, err := domain.ValidateEmail(user.Email)
	if err != nil {
		return err
	}
	user.Email = normalizedEmail
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.users[user.ID]; exists {
		return domain.NewConflict("user already exists")
	}
	if _, exists := r.byEmail[user.Email]; exists {
		return domain.NewConflict("email already exists")
	}
	r.users[user.ID] = cloneUser(user)
	r.byEmail[user.Email] = user.ID
	return nil
}

// GetByID returns a detached copy of the user with the requested ID.
func (r *UserRepository) GetByID(id domain.UserID) (domain.User, error) {
	r.mu.RLock()
	user, exists := r.users[id]
	r.mu.RUnlock()
	if !exists {
		return domain.User{}, domain.NewNotFound("user not found")
	}
	return cloneUser(user), nil
}

// GetByEmail returns a detached copy of the user for an email address.
func (r *UserRepository) GetByEmail(email string) (domain.User, error) {
	r.mu.RLock()
	id, exists := r.byEmail[NormalizeEmail(email)]
	if exists {
		user := r.users[id]
		r.mu.RUnlock()
		return cloneUser(user), nil
	}
	r.mu.RUnlock()
	return domain.User{}, domain.NewNotFound("user not found")
}

// Update replaces an existing user while preserving email uniqueness.
func (r *UserRepository) Update(user domain.User) error {
	normalizedEmail, err := domain.ValidateEmail(user.Email)
	if err != nil {
		return err
	}
	user.Email = normalizedEmail
	r.mu.Lock()
	defer r.mu.Unlock()
	old, exists := r.users[user.ID]
	if !exists {
		return domain.NewNotFound("user not found")
	}
	if otherID, used := r.byEmail[user.Email]; used && otherID != user.ID {
		return domain.NewConflict("email already exists")
	}
	oldEmail := NormalizeEmail(old.Email)
	if oldEmail != user.Email {
		delete(r.byEmail, oldEmail)
		r.byEmail[user.Email] = user.ID
	}
	r.users[user.ID] = cloneUser(user)
	return nil
}

// Delete removes a user from both the primary store and email index.
func (r *UserRepository) Delete(id domain.UserID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, exists := r.users[id]
	if !exists {
		return domain.NewNotFound("user not found")
	}
	delete(r.users, id)
	delete(r.byEmail, NormalizeEmail(user.Email))
	return nil
}

// List returns detached copies of all users. The order is unspecified.
func (r *UserRepository) List() []domain.User {
	r.mu.RLock()
	users := make([]domain.User, 0, len(r.users))
	for _, user := range r.users {
		users = append(users, cloneUser(user))
	}
	r.mu.RUnlock()
	return users
}

func cloneUser(user domain.User) domain.User {
	user.PasswordMaterial.Salt = append([]byte(nil), user.PasswordMaterial.Salt...)
	user.PasswordMaterial.Hash = append([]byte(nil), user.PasswordMaterial.Hash...)
	return user
}
