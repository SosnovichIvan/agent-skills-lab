package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"
)

var errEmailExists = errors.New("email already registered")

type AuthService struct {
	users  *UserRepository
	tokens TokenManager
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || !strings.Contains(email, "@") {
		return "", errors.New("invalid email")
	}
	return email, nil
}

func (s *AuthService) Register(email, password string) (User, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return User{}, err
	}
	if err = validatePassword(password); err != nil {
		return User{}, err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return User{}, err
	}
	idBytes := make([]byte, 16)
	if _, err = rand.Read(idBytes); err != nil {
		return User{}, err
	}
	u := User{ID: hex.EncodeToString(idBytes), Email: email, PasswordHash: hash}
	if !s.users.Create(u) {
		return User{}, errEmailExists
	}
	return u, nil
}

func (s *AuthService) Login(email, password string) (string, User, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return "", User{}, errors.New("invalid credentials")
	}
	u, ok := s.users.FindByEmail(email)
	if !ok || !verifyPassword(password, u.PasswordHash) {
		return "", User{}, errors.New("invalid credentials")
	}
	token, err := s.tokens.Sign(u, nowUTC())
	if err != nil {
		return "", User{}, err
	}
	return token, u, nil
}

var nowUTC = func() time.Time { return time.Now().UTC() }
