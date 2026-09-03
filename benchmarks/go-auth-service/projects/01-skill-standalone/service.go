package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"
)

var errInvalidCredentials = errors.New("invalid credentials")

type authService struct {
	users  *userRepository
	tokens tokenManager
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	p, e := mail.ParseAddress(email)
	if e != nil || p.Address != email || len(email) > 254 || !strings.Contains(email, "@") {
		return "", errors.New("invalid email")
	}
	return email, nil
}
func (s *authService) register(email, password string) (user, error) {
	email, e := normalizeEmail(email)
	if e != nil {
		return user{}, e
	}
	ph, e := hashPassword(password)
	if e != nil {
		return user{}, e
	}
	id, e := newID()
	if e != nil {
		return user{}, e
	}
	u := user{id, email, ph}
	if e = s.users.create(u); e != nil {
		return user{}, e
	}
	return u, nil
}
func (s *authService) login(email, password string) (string, error) {
	email, e := normalizeEmail(email)
	if e != nil {
		return "", errInvalidCredentials
	}
	u, e := s.users.findByEmail(email)
	if e != nil || !u.Password.verify(password) {
		return "", errInvalidCredentials
	}
	return s.tokens.sign(u.ID, time.Now().UTC())
}
func newID() (string, error) {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return hex.EncodeToString(b), nil
}
