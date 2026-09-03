package main

import (
	"crypto/rand"
	"errors"
	"strings"
	"time"
)

var ErrInvalidInput = errors.New("invalid input")
var ErrInvalidCredentials = errors.New("invalid credentials")

type AuthService struct {
	repo   *UserRepository
	tokens *TokenManager
}

func NewAuthService(repo *UserRepository, tokens *TokenManager) *AuthService {
	return &AuthService{repo: repo, tokens: tokens}
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(email) < 3 || len(email) > 254 || strings.Count(email, "@") != 1 || strings.ContainsAny(email, " \t\r\n") {
		return "", ErrInvalidInput
	}
	parts := strings.Split(email, "@")
	if parts[0] == "" || parts[1] == "" || !strings.Contains(parts[1], ".") || strings.HasPrefix(parts[1], ".") || strings.HasSuffix(parts[1], ".") {
		return "", ErrInvalidInput
	}
	return email, nil
}

func validatePassword(password string) error {
	if len([]byte(password)) < 8 || len([]byte(password)) > 128 {
		return ErrInvalidInput
	}
	return nil
}

func (s *AuthService) Register(email, password string) (PublicUser, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return PublicUser{}, err
	}
	if err := validatePassword(password); err != nil {
		return PublicUser{}, err
	}
	salt, hash, err := hashPassword(password)
	if err != nil {
		return PublicUser{}, err
	}
	user := User{ID: newID(), Email: email, Salt: salt, PasswordHash: hash}
	if err := s.repo.Create(user); err != nil {
		return PublicUser{}, err
	}
	return PublicUser{ID: user.ID, Email: user.Email}, nil
}

func (s *AuthService) Login(email, password string) (string, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return "", ErrInvalidCredentials
	}
	user, err := s.repo.FindByEmail(email)
	if err != nil || checkPassword(password, user.Salt, user.PasswordHash) != nil {
		return "", ErrInvalidCredentials
	}
	return s.tokens.Create(user)
}

func newID() string {
	return time.Now().UTC().Format("20060102T150405.000000000Z07:00") + "-" + randomID()
}

func randomID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "fallback-id"
	}
	return rawBase64.EncodeToString(b)
}
