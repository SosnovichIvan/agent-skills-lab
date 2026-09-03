package transport

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"authservice/internal/repository"
	"authservice/internal/service"
)

type Handler struct {
	auth   *service.Auth
	logger *slog.Logger
}
type request struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type response struct {
	Data any `json:"data,omitempty"`
}
type errorResponse struct {
	Error errorBody `json:"error"`
}
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func New(auth *service.Auth, logger *slog.Logger) http.Handler {
	h := &Handler{auth: auth, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /register", h.register)
	mux.HandleFunc("POST /login", h.login)
	mux.Handle("GET /me", h.requireAuth(http.HandlerFunc(h.me)))
	return h.recoverer(h.logging(mux))
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req request
	if !readJSON(w, r, &req) {
		return
	}
	user, err := h.auth.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, repository.ErrUserExists) {
			writeError(w, http.StatusConflict, "email_exists", "email is already registered")
			return
		}
		if errors.Is(err, service.ErrInvalidEmail) || strings.HasPrefix(err.Error(), "password ") {
			writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, response{Data: map[string]string{"id": user.ID, "email": user.Email}})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req request
	if !readJSON(w, r, &req) {
		return
	}
	tok, _, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}
	writeJSON(w, http.StatusOK, response{Data: map[string]string{"access_token": tok, "token_type": "Bearer"}})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(claimsKey{}).(struct{ ID, Email string })
	writeJSON(w, http.StatusOK, response{Data: map[string]string{"id": claims.ID, "email": claims.Email}})
}

type claimsKey struct{}

func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeError(w, http.StatusUnauthorized, "unauthorized", "valid Bearer token required")
			return
		}
		claims, err := h.auth.ValidateToken(parts[1])
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "valid Bearer token required")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey{}, struct{ ID, Email string }{claims.Subject, claims.Email})))
	})
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if r.Header.Get("Content-Type") != "application/json" {
		writeError(w, http.StatusBadRequest, "invalid_json", "Content-Type must be application/json")
		return false
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: errorBody{code, message}})
}
func (h *Handler) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		h.logger.Info("request", "method", r.Method, "path", r.URL.Path)
	})
}
func (h *Handler) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
