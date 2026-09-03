package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

type contextKey string

const claimsContextKey contextKey = "auth.claims"

type Transport struct{ service *Service }
type credentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type response struct {
	User        *Profile `json:"user,omitempty"`
	AccessToken string   `json:"access_token,omitempty"`
}
type errorBody struct {
	Error apiError `json:"error"`
}
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewHandler(service *Service) http.Handler {
	t := &Transport{service: service}
	mux := http.NewServeMux()
	mux.HandleFunc("/", t.route)
	return t.jsonMiddleware(mux)
}

func (t *Transport) route(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/register" && r.Method == http.MethodPost:
		t.register(w, r)
	case r.URL.Path == "/login" && r.Method == http.MethodPost:
		t.login(w, r)
	case r.URL.Path == "/me" && r.Method == http.MethodGet:
		t.requireAuth(http.HandlerFunc(t.me)).ServeHTTP(w, r)
	case r.URL.Path == "/register" || r.URL.Path == "/login" || r.URL.Path == "/me":
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method is not allowed")
	default:
		writeError(w, http.StatusNotFound, "not_found", "resource was not found")
	}
}

func (t *Transport) register(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	profile, err := t.service.Register(req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserExists):
			writeError(w, http.StatusConflict, "user_exists", "email is already registered")
		case errors.Is(err, ErrInvalidEmail):
			writeError(w, http.StatusBadRequest, "invalid_email", "email is invalid")
		default:
			writeError(w, http.StatusBadRequest, "invalid_password", "password must be 8 to 1024 characters")
		}
		return
	}
	writeJSON(w, http.StatusCreated, response{User: &profile})
}

func (t *Transport) login(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	token, err := t.service.Login(req.Email, req.Password, time.Now())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
		return
	}
	writeJSON(w, http.StatusOK, response{AccessToken: token})
}

func (t *Transport) me(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(claimsContextKey).(Claims)
	profile := Profile{ID: claims.Subject, Email: claims.Email}
	writeJSON(w, http.StatusOK, response{User: &profile})
}

func (t *Transport) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeError(w, http.StatusUnauthorized, "unauthorized", "valid Bearer token is required")
			return
		}
		claims, err := t.service.tokens.Validate(parts[1], time.Now())
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "valid Bearer token is required")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsContextKey, claims)))
	})
}

func (t *Transport) jsonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		next.ServeHTTP(w, r)
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return false
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must contain one JSON object")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: apiError{Code: code, Message: message}})
}
