package repository

import (
	"context"
	"errors"
	"sync"
)

var ErrUserExists = errors.New("user already exists")
var ErrUserNotFound = errors.New("user not found")

type User struct {
	ID           string
	Email        string
	PasswordHash []byte
	PasswordSalt []byte
	Iterations   int
}

type Users interface {
	Create(context.Context, User) error
	FindByEmail(context.Context, string) (User, error)
	FindByID(context.Context, string) (User, error)
}

type MemoryUsers struct {
	mu      sync.RWMutex
	byEmail map[string]User
	byID    map[string]User
}

func NewMemoryUsers() *MemoryUsers {
	return &MemoryUsers{byEmail: make(map[string]User), byID: make(map[string]User)}
}

func (m *MemoryUsers) Create(_ context.Context, user User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byEmail[user.Email]; ok {
		return ErrUserExists
	}
	copyUser := clone(user)
	m.byEmail[user.Email], m.byID[user.ID] = copyUser, copyUser
	return nil
}

func (m *MemoryUsers) FindByEmail(_ context.Context, email string) (User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, ok := m.byEmail[email]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return clone(user), nil
}

func (m *MemoryUsers) FindByID(_ context.Context, id string) (User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, ok := m.byID[id]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return clone(user), nil
}

func clone(u User) User {
	u.PasswordHash = append([]byte(nil), u.PasswordHash...)
	u.PasswordSalt = append([]byte(nil), u.PasswordSalt...)
	return u
}
