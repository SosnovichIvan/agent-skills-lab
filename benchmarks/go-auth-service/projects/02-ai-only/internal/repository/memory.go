package repository

import (
	"errors"
	"sync"
)

var ErrNotFound = errors.New("user not found")
var ErrConflict = errors.New("user already exists")

type User struct{ ID, Email, PasswordHash string }

type Users interface {
	Create(User) error
	ByEmail(string) (User, error)
	ByID(string) (User, error)
}

type Memory struct {
	mu      sync.RWMutex
	byEmail map[string]User
	byID    map[string]User
}

func NewMemory() *Memory { return &Memory{byEmail: make(map[string]User), byID: make(map[string]User)} }
func (m *Memory) Create(u User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byEmail[u.Email]; ok {
		return ErrConflict
	}
	m.byEmail[u.Email], m.byID[u.ID] = u, u
	return nil
}
func (m *Memory) ByEmail(email string) (User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.byEmail[email]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}
func (m *Memory) ByID(id string) (User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.byID[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}
