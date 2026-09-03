package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"

	"authservice/internal/model"
	"authservice/internal/password"
	"authservice/internal/repository"
	"authservice/internal/token"
)

var (
	ErrInvalidEmail    = errors.New("invalid email")
	ErrInvalidPassword = errors.New("password must be between 8 and 1024 characters")
	ErrDuplicate       = errors.New("email already registered")
	ErrCredentials     = errors.New("invalid credentials")
)

type UserRepository interface {
	Create(model.User) error
	FindByEmail(string) (model.User, error)
}

type Auth struct {
	users  UserRepository
	tokens *token.Manager
	now    func() time.Time
}

func New(users UserRepository, tokens *token.Manager) *Auth {
	return &Auth{users: users, tokens: tokens, now: time.Now}
}

func (a *Auth) Register(email, rawPassword string) (model.Profile, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		return model.Profile{}, err
	}
	if len([]rune(rawPassword)) < 8 || len([]rune(rawPassword)) > 1024 {
		return model.Profile{}, ErrInvalidPassword
	}
	salt, digest, err := password.Hash(rawPassword)
	if err != nil {
		return model.Profile{}, err
	}
	id, err := randomID()
	if err != nil {
		return model.Profile{}, err
	}
	user := model.User{ID: id, Email: email, PasswordSalt: salt, PasswordHash: digest, CreatedAt: a.now()}
	if err := a.users.Create(user); err != nil {
		if errors.Is(err, repository.ErrDuplicateEmail) {
			return model.Profile{}, ErrDuplicate
		}
		return model.Profile{}, err
	}
	return model.Profile{ID: user.ID, Email: user.Email}, nil
}

func (a *Auth) Login(email, rawPassword string) (string, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		return "", ErrCredentials
	}
	user, err := a.users.FindByEmail(email)
	if err != nil || !password.Verify(rawPassword, user.PasswordSalt, user.PasswordHash) {
		return "", ErrCredentials
	}
	return a.tokens.Create(user.ID, user.Email, a.now())
}

func (a *Auth) FindProfile(email string) (model.Profile, error) {
	user, err := a.users.FindByEmail(email)
	if err != nil {
		return model.Profile{}, err
	}
	return model.Profile{ID: user.ID, Email: user.Email}, nil
}

func NormalizeEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) == 0 || len(value) > 254 || strings.ContainsAny(value, "\r\n") {
		return "", ErrInvalidEmail
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value || !strings.Contains(value, "@") {
		return "", ErrInvalidEmail
	}
	return value, nil
}

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
