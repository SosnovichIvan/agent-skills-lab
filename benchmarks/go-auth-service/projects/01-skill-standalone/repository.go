package main

import (
	"errors"
	"strings"
	"sync"
)

var errUserExists = errors.New("user already exists")
var errUserNotFound = errors.New("user not found")

type user struct {
	ID       string
	Email    string
	Password passwordHash
}
type userRepository struct {
	mu      sync.RWMutex
	byEmail map[string]user
	byID    map[string]user
}

func newUserRepository() *userRepository {
	return &userRepository{byEmail: map[string]user{}, byID: map[string]user{}}
}
func (r *userRepository) create(u user) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byEmail[u.Email]; ok {
		return errUserExists
	}
	r.byEmail[u.Email] = u
	r.byID[u.ID] = u
	return nil
}
func (r *userRepository) findByEmail(email string) (user, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.byEmail[strings.ToLower(email)]
	if !ok {
		return user{}, errUserNotFound
	}
	return u, nil
}
func (r *userRepository) findByID(id string) (user, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.byID[id]
	if !ok {
		return user{}, errUserNotFound
	}
	return u, nil
}
