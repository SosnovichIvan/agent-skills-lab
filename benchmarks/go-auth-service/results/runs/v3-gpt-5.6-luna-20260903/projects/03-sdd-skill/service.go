package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrInvalidInput = errors.New("invalid input")

type AuthService struct {
	repo   *UserRepository
	tokens *TokenManager
}

func NewAuthService(repo *UserRepository, tokens *TokenManager) *AuthService {
	return &AuthService{repo: repo, tokens: tokens}
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || len(email) > 254 || !strings.Contains(email, "@") {
		return "", ErrInvalidInput
	}
	return email, nil
}

func (s *AuthService) Register(email, password string) (PublicUser, error) {
	email, err := normalizeEmail(email)
	if err != nil || len(password) < minPasswordLength {
		return PublicUser{}, ErrInvalidInput
	}
	salt, hash, err := hashPassword(password)
	if err != nil {
		return PublicUser{}, err
	}
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return PublicUser{}, err
	}
	user := User{ID: hex.EncodeToString(idBytes), Email: email, PasswordSalt: salt, PasswordHash: hash}
	if err := s.repo.Create(user); err != nil {
		return PublicUser{}, err
	}
	return user.Public(), nil
}

func (s *AuthService) Login(email, password string) (string, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return "", ErrInvalidCredentials
	}
	user, err := s.repo.FindByEmail(email)
	if err != nil || !verifyPassword(password, user.PasswordSalt, user.PasswordHash) {
		return "", ErrInvalidCredentials
	}
	return s.tokens.Create(user.Public())
}
