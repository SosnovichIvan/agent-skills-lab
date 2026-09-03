package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

type contextKey string

const claimsKey contextKey = "auth-claims"

type apiServer struct {
	auth      *AuthService
	bodyLimit int64
}
type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, apiError{Error: message})
}

func (s *apiServer) decode(w http.ResponseWriter, r *http.Request, value any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, s.bodyLimit)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(value); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func (s *apiServer) register(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if !s.decode(w, r, &input) {
		return
	}
	user, err := s.auth.Register(input.Email, input.Password)
	if errors.Is(err, ErrUserExists) {
		writeError(w, http.StatusConflict, "email already registered")
		return
	}
	if errors.Is(err, ErrInvalidInput) {
		writeError(w, http.StatusBadRequest, "invalid email or password")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, user)
}
func (s *apiServer) login(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if !s.decode(w, r, &input) {
		return
	}
	token, err := s.auth.Login(input.Email, input.Password)
	if errors.Is(err, ErrInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"access_token": token, "token_type": "Bearer"})
}
func (s *apiServer) me(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(claimsKey).(Claims)
	writeJSON(w, http.StatusOK, PublicUser{ID: claims.Subject, Email: claims.Email})
}

func (s *apiServer) requireAuth(next http.Handler, tokens *TokenManager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		claims, err := tokens.Validate(parts[1])
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey, claims)))
	})
}

func (s *apiServer) routes(tokens *TokenManager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/register" && r.Method == http.MethodPost:
			s.register(w, r)
		case r.URL.Path == "/login" && r.Method == http.MethodPost:
			s.login(w, r)
		case r.URL.Path == "/me" && r.Method == http.MethodGet:
			s.requireAuth(http.HandlerFunc(s.me), tokens).ServeHTTP(w, r)
		case r.URL.Path == "/register" || r.URL.Path == "/login" || r.URL.Path == "/me":
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		default:
			writeError(w, http.StatusNotFound, "not found")
		}
	})
}
