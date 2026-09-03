package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	var e apiError
	e.Error.Code, e.Error.Message = code, message
	writeJSON(w, status, e)
}
func decodeCredentials(r *http.Request) (credentials, error) {
	var c credentials
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		return c, err
	}
	return c, nil
}

func (s *AuthService) registerHandler(w http.ResponseWriter, r *http.Request) {
	c, err := decodeCredentials(r)
	if err != nil {
		writeError(w, 400, "invalid_json", "request body must be valid JSON")
		return
	}
	u, err := s.Register(c.Email, c.Password)
	if err != nil {
		code := "invalid_request"
		status := 400
		if errors.Is(err, errEmailExists) {
			code, status = "conflict", 409
		}
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"id": u.ID, "email": u.Email})
}
func (s *AuthService) loginHandler(w http.ResponseWriter, r *http.Request) {
	c, err := decodeCredentials(r)
	if err != nil {
		writeError(w, 400, "invalid_json", "request body must be valid JSON")
		return
	}
	token, _, err := s.Login(c.Email, c.Password)
	if err != nil {
		writeError(w, 401, "invalid_credentials", "invalid email or password")
		return
	}
	writeJSON(w, 200, map[string]string{"access_token": token, "token_type": "Bearer"})
}

type contextKey string

const claimsKey contextKey = "claims"

func authMiddleware(tm TokenManager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := parseBearer(r.Header.Get("Authorization"))
		if err != nil {
			writeError(w, 401, "unauthorized", "Bearer token required")
			return
		}
		claims, err := tm.Verify(raw, nowUTC())
		if err != nil {
			writeError(w, 401, "unauthorized", "invalid or expired token")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey, claims)))
	})
}
func meHandler(w http.ResponseWriter, r *http.Request) {
	c := r.Context().Value(claimsKey).(tokenClaims)
	writeJSON(w, 200, map[string]string{"id": c.Subject, "email": c.Email})
}
