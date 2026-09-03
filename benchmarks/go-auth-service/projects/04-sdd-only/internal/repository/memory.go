package repository

import (
	"errors"
	"sync"

	"authservice/internal/model"
)

var ErrDuplicateEmail = errors.New("email already registered")
var ErrNotFound = errors.New("user not found")

type Memory struct {
	mu     sync.RWMutex
	byMail map[string]model.User
}

func NewMemory() *Memory { return &Memory{byMail: make(map[string]model.User)} }

func (r *Memory) Create(user model.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byMail[user.Email]; exists {
		return ErrDuplicateEmail
	}
	user.PasswordSalt = append([]byte(nil), user.PasswordSalt...)
	user.PasswordHash = append([]byte(nil), user.PasswordHash...)
	r.byMail[user.Email] = user
	return nil
}

func (r *Memory) FindByEmail(email string) (model.User, error) {
	r.mu.RLock()
	user, ok := r.byMail[email]
	r.mu.RUnlock()
	if !ok {
		return model.User{}, ErrNotFound
	}
	user.PasswordSalt = append([]byte(nil), user.PasswordSalt...)
	user.PasswordHash = append([]byte(nil), user.PasswordHash...)
	return user, nil
}
