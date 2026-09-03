package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"authservice/internal/model"
	"authservice/internal/service"
	"authservice/internal/token"
)

type contextKey string

const claimsKey contextKey = "auth-claims"

type Server struct {
	auth   *service.Auth
	tokens *token.Manager
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func New(auth *service.Auth, tokens *token.Manager) *Server {
	return &Server{auth: auth, tokens: tokens}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /register", s.register)
	mux.HandleFunc("POST /login", s.login)
	mux.Handle("GET /me", s.requireAuth(http.HandlerFunc(s.me)))
	return mux
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if !decode(w, r, &input) {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must contain valid JSON")
		return
	}
	profile, err := s.auth.Register(input.Email, input.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidEmail), errors.Is(err, service.ErrInvalidPassword):
			writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		case errors.Is(err, service.ErrDuplicate):
			writeError(w, http.StatusConflict, "email_exists", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		}
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": profile})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if !decode(w, r, &input) {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must contain valid JSON")
		return
	}
	accessToken, err := s.auth.Login(input.Email, input.Password)
	if err != nil {
		if errors.Is(err, service.ErrCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid_credentials", err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"access_token": accessToken, "token_type": "Bearer"})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(claimsKey).(token.Claims)
	writeJSON(w, http.StatusOK, map[string]any{"user": model.Profile{ID: claims.Subject, Email: claims.Email}})
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeError(w, http.StatusUnauthorized, "unauthorized", "valid Bearer token is required")
			return
		}
		claims, err := s.tokens.Validate(parts[1], timeNow())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "valid Bearer token is required")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey, claims)))
	})
}

var timeNow = func() time.Time { return time.Now() }

func decode(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(destination); err != nil {
		return false
	}
	var extra any
	return decoder.Decode(&extra) == io.EOF
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
