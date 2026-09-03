package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type api struct{ service *authService }
type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type response struct {
	Data  any       `json:"data,omitempty"`
	Error *apiError `json:"error,omitempty"`
}
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (a *api) routes() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("POST /register", a.register)
	m.HandleFunc("POST /login", a.login)
	m.Handle("GET /me", a.auth(http.HandlerFunc(a.me)))
	return m
}
func decode(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(dst); e != nil {
		return errors.New("invalid JSON body")
	}
	return nil
}
func writeJSON(w http.ResponseWriter, status int, data any, e *apiError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response{data, e})
}
func fail(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, nil, &apiError{code, msg})
}
func (a *api) register(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if decode(w, r, &in) != nil {
		fail(w, 400, "invalid_request", "invalid JSON body")
		return
	}
	u, e := a.service.register(in.Email, in.Password)
	if errors.Is(e, errUserExists) {
		fail(w, 409, "email_exists", "email is already registered")
		return
	}
	if e != nil {
		fail(w, 400, "invalid_request", e.Error())
		return
	}
	writeJSON(w, 201, map[string]string{"id": u.ID, "email": u.Email}, nil)
}
func (a *api) login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if decode(w, r, &in) != nil {
		fail(w, 400, "invalid_request", "invalid JSON body")
		return
	}
	t, e := a.service.login(in.Email, in.Password)
	if e != nil {
		fail(w, 401, "invalid_credentials", "invalid email or password")
		return
	}
	writeJSON(w, 200, map[string]string{"access_token": t, "token_type": "Bearer"}, nil)
}
func (a *api) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.Fields(r.Header.Get("Authorization"))
		if len(p) != 2 || !strings.EqualFold(p[0], "Bearer") {
			fail(w, 401, "unauthorized", "Bearer token required")
			return
		}
		c, e := a.service.tokens.verify(p[1], time.Now().UTC())
		if e != nil {
			fail(w, 401, "unauthorized", "invalid access token")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey{}, c.Subject)))
	})
}
func (a *api) me(w http.ResponseWriter, r *http.Request) {
	id, _ := r.Context().Value(userIDKey{}).(string)
	u, e := a.service.users.findByID(id)
	if e != nil {
		fail(w, 401, "unauthorized", "user not found")
		return
	}
	writeJSON(w, 200, map[string]string{"id": u.ID, "email": u.Email}, nil)
}

type userIDKey struct{}
