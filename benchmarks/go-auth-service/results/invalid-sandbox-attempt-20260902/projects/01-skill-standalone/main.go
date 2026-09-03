package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	passwordIterations = 120000
	saltSize           = 16
)

type Config struct {
	Secret  []byte
	Issuer  string
	Address string
	TTL     time.Duration
}

type User struct {
	ID           string
	Email        string
	PasswordSalt []byte
	PasswordHash []byte
}

type UserRepository struct {
	mu      sync.RWMutex
	byEmail map[string]User
	byID    map[string]User
}

func NewUserRepository() *UserRepository {
	return &UserRepository{byEmail: make(map[string]User), byID: make(map[string]User)}
}

func (r *UserRepository) Create(user User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byEmail[user.Email]; exists {
		return errors.New("email already registered")
	}
	r.byEmail[user.Email], r.byID[user.ID] = user, user
	return nil
}

func (r *UserRepository) ByEmail(email string) (User, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.byEmail[email]
	return u, ok
}

func (r *UserRepository) ByID(id string) (User, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.byID[id]
	return u, ok
}

func derivePassword(password string, salt []byte) []byte {
	mac := hmac.New(sha256.New, salt)
	mac.Write([]byte(password))
	previous := mac.Sum(nil)
	result := append([]byte(nil), previous...)
	for i := 1; i < passwordIterations; i++ {
		mac = hmac.New(sha256.New, salt)
		mac.Write(previous)
		previous = mac.Sum(nil)
		for j := range result {
			result[j] ^= previous[j]
		}
	}
	return result
}

func hashPassword(password string) ([]byte, []byte, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, nil, err
	}
	return salt, derivePassword(password, salt), nil
}

func verifyPassword(password string, salt, expected []byte) bool {
	actual := derivePassword(password, salt)
	return len(actual) == len(expected) && subtle.ConstantTimeCompare(actual, expected) == 1
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

type TokenManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

func (t TokenManager) Sign(user User, now time.Time) (string, error) {
	enc := base64.RawURLEncoding.EncodeToString
	header := enc([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, err := json.Marshal(map[string]any{"sub": user.ID, "iss": t.issuer, "iat": now.Unix(), "exp": now.Add(t.ttl).Unix()})
	if err != nil {
		return "", err
	}
	body := header + "." + enc(payload)
	mac := hmac.New(sha256.New, t.secret)
	mac.Write([]byte(body))
	return body + "." + enc(mac.Sum(nil)), nil
}

func (t TokenManager) Verify(token string, now time.Time) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", errors.New("invalid token")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", errors.New("invalid token")
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if json.Unmarshal(headerBytes, &header) != nil || header.Alg != "HS256" {
		return "", errors.New("invalid token algorithm")
	}
	mac := hmac.New(sha256.New, t.secret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || subtle.ConstantTimeCompare(mac.Sum(nil), signature) != 1 {
		return "", errors.New("invalid token signature")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", errors.New("invalid token")
	}
	var payload struct {
		Sub, Iss string
		Exp      int64 `json:"exp"`
	}
	if json.Unmarshal(payloadBytes, &payload) != nil || payload.Sub == "" || payload.Iss != t.issuer || payload.Exp <= now.Unix() {
		return "", errors.New("expired or invalid token")
	}
	return payload.Sub, nil
}

type API struct {
	users  *UserRepository
	tokens TokenManager
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": errorBody{Code: code, Message: message}})
}
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if dec.Decode(dst) != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return false
	}
	return true
}
func normalizeEmail(email string) (string, bool) {
	email = strings.ToLower(strings.TrimSpace(email))
	parsed, err := mail.ParseAddress(email)
	return email, err == nil && parsed.Address == email && len(email) <= 254
}
func validatePassword(password string) bool { return len(password) >= 8 && len(password) <= 128 }

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	var req credentials
	if !decodeJSON(w, r, &req) {
		return
	}
	email, ok := normalizeEmail(req.Email)
	if !ok {
		writeError(w, 400, "invalid_email", "email is invalid")
		return
	}
	if !validatePassword(req.Password) {
		writeError(w, 400, "invalid_password", "password must be 8 to 128 characters")
		return
	}
	salt, hash, err := hashPassword(req.Password)
	if err != nil {
		writeError(w, 500, "internal_error", "could not create user")
		return
	}
	id, err := newID()
	if err != nil {
		writeError(w, 500, "internal_error", "could not create user")
		return
	}
	if err := a.users.Create(User{ID: id, Email: email, PasswordSalt: salt, PasswordHash: hash}); err != nil {
		writeError(w, 409, "email_exists", "email is already registered")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "email": email})
}
func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var req credentials
	if !decodeJSON(w, r, &req) {
		return
	}
	email, ok := normalizeEmail(req.Email)
	if !ok || !validatePassword(req.Password) {
		writeError(w, 401, "invalid_credentials", "email or password is incorrect")
		return
	}
	user, found := a.users.ByEmail(email)
	if !found || !verifyPassword(req.Password, user.PasswordSalt, user.PasswordHash) {
		writeError(w, 401, "invalid_credentials", "email or password is incorrect")
		return
	}
	token, err := a.tokens.Sign(user, time.Now())
	if err != nil {
		writeError(w, 500, "internal_error", "could not issue token")
		return
	}
	writeJSON(w, 200, map[string]string{"access_token": token, "token_type": "Bearer", "expires_in": fmt.Sprint(int64(a.tokens.ttl.Seconds()))})
}
func (a *API) me(w http.ResponseWriter, r *http.Request) {
	id := r.Context().Value(userIDKey{}).(string)
	user, ok := a.users.ByID(id)
	if !ok {
		writeError(w, 401, "unauthorized", "user is not available")
		return
	}
	writeJSON(w, 200, map[string]string{"id": user.ID, "email": user.Email})
}

type userIDKey struct{}

func (a *API) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeError(w, 401, "unauthorized", "Bearer token is required")
			return
		}
		id, err := a.tokens.Verify(parts[1], time.Now())
		if err != nil {
			writeError(w, 401, "unauthorized", "token is invalid")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey{}, id)))
	})
}

func loadConfig() (Config, error) {
	secret := os.Getenv("AUTH_SECRET")
	if len(secret) < 32 {
		return Config{}, errors.New("AUTH_SECRET must be at least 32 bytes")
	}
	issuer := strings.TrimSpace(os.Getenv("AUTH_ISSUER"))
	if issuer == "" {
		return Config{}, errors.New("AUTH_ISSUER is required")
	}
	address := os.Getenv("AUTH_ADDR")
	if address == "" {
		address = ":8080"
	}
	ttl := 15 * time.Minute
	if raw := os.Getenv("AUTH_TOKEN_TTL"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed <= 0 {
			return Config{}, errors.New("AUTH_TOKEN_TTL must be a positive duration")
		}
		ttl = parsed
	}
	return Config{[]byte(secret), issuer, address, ttl}, nil
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}
	api := &API{users: NewUserRepository(), tokens: TokenManager{cfg.Secret, cfg.Issuer, cfg.TTL}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /register", api.register)
	mux.HandleFunc("POST /login", api.login)
	mux.Handle("GET /me", api.auth(http.HandlerFunc(api.me)))
	server := &http.Server{Addr: cfg.Address, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		log.Printf("authentication service listening on %s", cfg.Address)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
			stop <- syscall.SIGTERM
		}
	}()
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown error: %v", err)
	}
}
