package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"net/mail"
	"strings"
	"sync"
	"time"
)

type User struct {
	ID, Email string
	Password  PasswordMaterial
}
type PublicUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}
type PasswordMaterial struct {
	Salt, Hash []byte
	Iterations int
}

type UserRepository struct {
	mu    sync.RWMutex
	users map[string]User
}

func NewUserRepository() *UserRepository { return &UserRepository{users: make(map[string]User)} }
func (r *UserRepository) Create(user User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.users[user.Email]; ok {
		return errEmailExists
	}
	r.users[user.Email] = user
	return nil
}
func (r *UserRepository) Find(email string) (User, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	user, ok := r.users[email]
	return user, ok
}

func normalizeEmail(value string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(value))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || !strings.Contains(email, "@") {
		return "", errors.New("invalid email")
	}
	return email, nil
}

const passwordIterations = 120000

func hashPassword(password string) (PasswordMaterial, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return PasswordMaterial{}, err
	}
	return PasswordMaterial{Salt: salt, Hash: deriveKey([]byte(password), salt, passwordIterations), Iterations: passwordIterations}, nil
}
func deriveKey(password, salt []byte, iterations int) []byte {
	var input [4]byte
	binary.BigEndian.PutUint32(input[:], 1)
	h := hmac.New(sha256.New, password)
	h.Write(salt)
	h.Write(input[:])
	u := h.Sum(nil)
	result := append([]byte(nil), u...)
	for i := 1; i < iterations; i++ {
		h = hmac.New(sha256.New, password)
		h.Write(u)
		u = h.Sum(nil)
		for j := range result {
			result[j] ^= u[j]
		}
	}
	return result
}
func verifyPassword(password string, material PasswordMaterial) bool {
	actual := deriveKey([]byte(password), material.Salt, material.Iterations)
	return subtle.ConstantTimeCompare(actual, material.Hash) == 1
}

type AuthService struct {
	repo   *UserRepository
	tokens *TokenManager
}

func (s *AuthService) Register(email, password string) (PublicUser, error) {
	normalized, err := normalizeEmail(email)
	if err != nil {
		return PublicUser{}, err
	}
	if len([]byte(password)) < 8 {
		return PublicUser{}, errors.New("password must be at least 8 bytes")
	}
	material, err := hashPassword(password)
	if err != nil {
		return PublicUser{}, err
	}
	user := User{ID: newID(), Email: normalized, Password: material}
	if err := s.repo.Create(user); err != nil {
		return PublicUser{}, err
	}
	return publicUser(user), nil
}
func (s *AuthService) Login(email, password string) (string, error) {
	normalized, err := normalizeEmail(email)
	if err != nil {
		return "", errInvalidCredentials
	}
	user, ok := s.repo.Find(normalized)
	if !ok || !verifyPassword(password, user.Password) {
		return "", errInvalidCredentials
	}
	return s.tokens.Create(user)
}
func publicUser(user User) PublicUser { return PublicUser{ID: user.ID, Email: user.Email} }
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		sum := sha256.Sum256([]byte(time.Now().String()))
		return base64URL(sum[:])
	}
	return base64URL(b)
}

type Claims struct {
	Subject, Email, Issuer, JTI string
	IssuedAt, ExpiresAt         int64
}
type contextKey string

const claimsKey contextKey = "auth.claims"

func claimsFromContext(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(claimsKey).(Claims)
	return claims, ok
}
