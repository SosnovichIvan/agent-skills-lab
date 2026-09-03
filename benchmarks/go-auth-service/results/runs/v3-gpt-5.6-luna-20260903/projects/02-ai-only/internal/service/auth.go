package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"

	"authservice/internal/password"
	"authservice/internal/repository"
	"authservice/internal/token"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrInvalidEmail = errors.New("invalid email")

type Auth struct {
	users  repository.Users
	hasher password.Hasher
	tokens token.Service
}

func New(users repository.Users, hasher password.Hasher, tokens token.Service) *Auth {
	return &Auth{users, hasher, tokens}
}

func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if len(email) > 254 {
		return "", ErrInvalidEmail
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || !strings.Contains(email, "@") {
		return "", ErrInvalidEmail
	}
	return email, nil
}

func (a *Auth) Register(ctx context.Context, email, plain string) (repository.User, error) {
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return repository.User{}, err
	}
	if err := password.Validate(plain); err != nil {
		return repository.User{}, err
	}
	hash, err := a.hasher.Hash(plain)
	if err != nil {
		return repository.User{}, err
	}
	id, err := randomID()
	if err != nil {
		return repository.User{}, err
	}
	user := repository.User{ID: id, Email: normalized, PasswordHash: hash.DerivedKey, PasswordSalt: hash.Salt, Iterations: hash.Iterations}
	if err := a.users.Create(ctx, user); err != nil {
		return repository.User{}, err
	}
	return user, nil
}

func (a *Auth) Login(ctx context.Context, email, plain string) (string, repository.User, error) {
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return "", repository.User{}, ErrInvalidCredentials
	}
	user, err := a.users.FindByEmail(ctx, normalized)
	if err != nil || !a.hasher.Verify(plain, password.Hash{Salt: user.PasswordSalt, DerivedKey: user.PasswordHash, Iterations: user.Iterations}) {
		return "", repository.User{}, ErrInvalidCredentials
	}
	tok, err := a.tokens.Issue(user.ID, user.Email, time.Now())
	return tok, user, err
}

func (a *Auth) ValidateToken(raw string) (token.Claims, error) {
	return a.tokens.Validate(raw, time.Now())
}
func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
