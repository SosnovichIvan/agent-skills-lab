package main

import (
	"errors"
	"sync"
)

var ErrUserExists = errors.New("user already exists")
var ErrUserNotFound = errors.New("user not found")

type UserRepository struct {
	mu    sync.RWMutex
	users map[string]User
}

func NewUserRepository() *UserRepository { return &UserRepository{users: make(map[string]User)} }

func (r *UserRepository) Create(user User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.users[user.Email]; exists {
		return ErrUserExists
	}
	r.users[user.Email] = User{ID: user.ID, Email: user.Email, Salt: append([]byte(nil), user.Salt...), PasswordHash: append([]byte(nil), user.PasswordHash...)}
	return nil
}

func (r *UserRepository) FindByEmail(email string) (User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	user, ok := r.users[email]
	if !ok {
		return User{}, ErrUserNotFound
	}
	user.Salt = append([]byte(nil), user.Salt...)
	user.PasswordHash = append([]byte(nil), user.PasswordHash...)
	return user, nil
}
