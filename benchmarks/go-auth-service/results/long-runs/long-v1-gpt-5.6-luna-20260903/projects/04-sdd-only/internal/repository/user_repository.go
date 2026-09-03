// Package repository contains concurrent in-memory repositories.
package repository

import (
	"sync"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/security"
)

// UserStore is the user persistence contract consumed by application code.
type UserStore interface {
	Create(user domain.User) error
	Get(id domain.ID) (domain.User, error)
	FindByEmail(email string) (domain.User, error)
	Update(user domain.User) error
	Delete(id domain.ID) error
}

// UserRepository stores users and maintains a unique normalized-email index.
// Both maps and the User values they contain are protected by mu.
type UserRepository struct {
	mu      sync.RWMutex
	byID    map[domain.ID]domain.User
	byEmail map[string]domain.ID
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		byID:    make(map[domain.ID]domain.User),
		byEmail: make(map[string]domain.ID),
	}
}

// Create inserts a user and rejects duplicate IDs and normalized email
// addresses. The repository stores a private copy of the supplied value.
func (r *UserRepository) Create(user domain.User) error {
	if user.ID == "" {
		return domain.NewError(domain.KindInvalid, "user ID is required")
	}
	email, err := security.NormalizeEmail(user.Email)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byID[user.ID]; exists {
		return domain.NewError(domain.KindConflict, "user ID already exists")
	}
	if _, exists := r.byEmail[email]; exists {
		return domain.NewError(domain.KindConflict, "user email already exists")
	}
	user.Email = email
	r.byID[user.ID] = cloneUser(user)
	r.byEmail[email] = user.ID
	return nil
}

// Get returns a copy of the user identified by id.
func (r *UserRepository) Get(id domain.ID) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	user, ok := r.byID[id]
	if !ok {
		return domain.User{}, domain.NewError(domain.KindNotFound, "user not found")
	}
	return cloneUser(user), nil
}

// FindByEmail returns a copy of the user matching a normalized email.
func (r *UserRepository) FindByEmail(email string) (domain.User, error) {
	normalized, err := security.NormalizeEmail(email)
	if err != nil {
		return domain.User{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byEmail[normalized]
	if !ok {
		return domain.User{}, domain.NewError(domain.KindNotFound, "user not found")
	}
	return cloneUser(r.byID[id]), nil
}

// Update replaces a user while preserving email-index uniqueness.
func (r *UserRepository) Update(user domain.User) error {
	if user.ID == "" {
		return domain.NewError(domain.KindInvalid, "user ID is required")
	}
	email, err := security.NormalizeEmail(user.Email)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	previous, exists := r.byID[user.ID]
	if !exists {
		return domain.NewError(domain.KindNotFound, "user not found")
	}
	if indexedID, exists := r.byEmail[email]; exists && indexedID != user.ID {
		return domain.NewError(domain.KindConflict, "user email already exists")
	}
	previousEmail, _ := security.NormalizeEmail(previous.Email)
	delete(r.byEmail, previousEmail)
	user.Email = email
	r.byID[user.ID] = cloneUser(user)
	r.byEmail[email] = user.ID
	return nil
}

// Delete removes a user and its email index entry.
func (r *UserRepository) Delete(id domain.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, exists := r.byID[id]
	if !exists {
		return domain.NewError(domain.KindNotFound, "user not found")
	}
	delete(r.byID, id)
	userEmail, _ := security.NormalizeEmail(user.Email)
	delete(r.byEmail, userEmail)
	return nil
}

func cloneUser(user domain.User) domain.User {
	user.PasswordMaterial.Salt = append([]byte(nil), user.PasswordMaterial.Salt...)
	user.PasswordMaterial.Hash = append([]byte(nil), user.PasswordMaterial.Hash...)
	return user
}
