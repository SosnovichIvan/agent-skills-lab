package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"authservice/internal/repository"
	"authservice/internal/service"
	"authservice/internal/token"
)

type Handler struct {
	app    *service.Service
	tokens *token.Issuer
}

func New(app *service.Service, tokens *token.Issuer) http.Handler {
	h := &Handler{app, tokens}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /register", h.register)
	mux.HandleFunc("POST /login", h.login)
	mux.Handle("GET /me", h.auth(http.HandlerFunc(h.me)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/register" && r.URL.Path != "/login" && r.URL.Path != "/me" {
			writeError(w, http.StatusNotFound, "not_found", "resource not found")
			return
		}
		if (r.URL.Path == "/register" || r.URL.Path == "/login") && r.Method != http.MethodPost || r.URL.Path == "/me" && r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method is not allowed")
			return
		}
		mux.ServeHTTP(w, r)
	})
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, 400, "invalid_request", "request body must be valid JSON")
		return false
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		writeError(w, 400, "invalid_request", "request body must contain one JSON object")
		return false
	}
	return true
}
func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !decode(w, r, &in) {
		return
	}
	u, err := h.app.Register(in.Email, in.Password)
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			writeError(w, 409, "email_exists", "email is already registered")
		} else {
			writeError(w, 400, "invalid_request", err.Error())
		}
		return
	}
	writeJSON(w, 201, map[string]any{"data": map[string]string{"id": u.ID, "email": u.Email}})
}
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !decode(w, r, &in) {
		return
	}
	t, err := h.app.Login(in.Email, in.Password)
	if err != nil {
		writeError(w, 401, "invalid_credentials", "invalid email or password")
		return
	}
	writeJSON(w, 200, map[string]any{"data": map[string]string{"access_token": t, "token_type": "Bearer"}})
}
func (h *Handler) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := token.Bearer(r.Header.Get("Authorization"))
		if err != nil {
			writeError(w, 401, "unauthorized", "missing or invalid bearer token")
			return
		}
		sub, err := h.tokens.Validate(raw)
		if err != nil {
			writeError(w, 401, "unauthorized", "invalid or expired token")
			return
		}
		r = r.WithContext(withSubject(r.Context(), sub))
		next.ServeHTTP(w, r)
	})
}

type contextKey string

const subjectKey contextKey = "subject"

func withSubject(ctx context.Context, subject string) context.Context {
	return context.WithValue(ctx, subjectKey, subject)
}
func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	id, ok := r.Context().Value(subjectKey).(string)
	if !ok {
		writeError(w, 401, "unauthorized", "authentication required")
		return
	}
	u, err := h.app.User(id)
	if err != nil {
		writeError(w, 401, "unauthorized", "user no longer exists")
		return
	}
	writeJSON(w, 200, map[string]any{"data": map[string]string{"id": u.ID, "email": u.Email}})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
