package main

import "sync"

type User struct {
	ID           string
	Email        string
	PasswordHash string
}

type UserRepository struct {
	mu     sync.RWMutex
	byMail map[string]User
}

func NewUserRepository() *UserRepository { return &UserRepository{byMail: make(map[string]User)} }

func (r *UserRepository) Create(user User) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byMail[user.Email]; exists {
		return false
	}
	r.byMail[user.Email] = user
	return true
}

func (r *UserRepository) FindByEmail(email string) (User, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.byMail[email]
	return u, ok
}
