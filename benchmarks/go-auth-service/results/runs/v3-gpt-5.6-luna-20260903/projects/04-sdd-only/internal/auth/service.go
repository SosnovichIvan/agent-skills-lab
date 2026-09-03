package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrInvalidEmail = errors.New("invalid email")
var ErrInvalidPassword = errors.New("invalid password")

type Service struct {
	repo   *UserRepository
	tokens *TokenManager
}

func NewService(repo *UserRepository, tokens *TokenManager) *Service {
	return &Service{repo: repo, tokens: tokens}
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(email) < 3 || len(email) > 254 || strings.Count(email, "@") != 1 || strings.ContainsAny(email, " \t\r\n") {
		return "", ErrInvalidEmail
	}
	at := strings.IndexByte(email, '@')
	if at < 1 || at == len(email)-1 || !strings.Contains(email[at+1:], ".") || strings.HasPrefix(email[at+1:], ".") || strings.HasSuffix(email, ".") {
		return "", ErrInvalidEmail
	}
	return email, nil
}

func (s *Service) Register(email, password string) (Profile, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return Profile{}, err
	}
	if len(password) < 8 || len(password) > 1024 {
		return Profile{}, ErrInvalidPassword
	}
	salt, hash, err := hashPassword(password)
	if err != nil {
		return Profile{}, err
	}
	idBytes := make([]byte, 16)
	if _, err = rand.Read(idBytes); err != nil {
		return Profile{}, err
	}
	user := User{ID: hex.EncodeToString(idBytes), Email: email, PasswordSalt: salt, PasswordHash: hash}
	if err = s.repo.Create(user); err != nil {
		return Profile{}, err
	}
	return user.PublicProfile(), nil
}

func (s *Service) Login(email, password string, now time.Time) (string, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return "", ErrInvalidCredentials
	}
	user, err := s.repo.FindByEmail(email)
	if err != nil || !verifyPassword(password, user.PasswordSalt, user.PasswordHash) {
		return "", ErrInvalidCredentials
	}
	return s.tokens.Create(user.PublicProfile(), now)
}
