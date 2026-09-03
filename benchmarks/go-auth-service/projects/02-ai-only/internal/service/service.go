package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"

	"authservice/internal/password"
	"authservice/internal/repository"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type Service struct {
	users  repository.Users
	tokens TokenIssuer
}
type TokenIssuer interface{ Issue(string) (string, error) }

func New(users repository.Users, tokens TokenIssuer) *Service { return &Service{users, tokens} }
func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || !strings.Contains(email, "@") {
		return "", errors.New("invalid email")
	}
	return email, nil
}
func (s *Service) Register(email, plain string) (repository.User, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return repository.User{}, err
	}
	hash, err := password.Hash(plain)
	if err != nil {
		return repository.User{}, err
	}
	idb := make([]byte, 16)
	if _, err = rand.Read(idb); err != nil {
		return repository.User{}, err
	}
	u := repository.User{ID: hex.EncodeToString(idb), Email: email, PasswordHash: hash}
	if err = s.users.Create(u); err != nil {
		return repository.User{}, err
	}
	return u, nil
}
func (s *Service) Login(email, plain string) (string, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return "", ErrInvalidCredentials
	}
	u, err := s.users.ByEmail(email)
	if err != nil || !password.Compare(plain, u.PasswordHash) {
		return "", ErrInvalidCredentials
	}
	return s.tokens.Issue(u.ID)
}
func (s *Service) User(id string) (repository.User, error) { return s.users.ByID(id) }
