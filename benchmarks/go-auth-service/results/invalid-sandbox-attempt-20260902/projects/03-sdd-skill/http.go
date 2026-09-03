package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

type apiServer struct{ auth *AuthService }
type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}
type apiError struct {
	Error string `json:"error"`
}

func (a *apiServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /register", a.register)
	mux.HandleFunc("POST /login", a.login)
	mux.Handle("GET /me", a.authMiddleware(http.HandlerFunc(a.me)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/register" && r.URL.Path != "/login" && r.URL.Path != "/me" {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		if (r.URL.Path == "/me" && r.Method != http.MethodGet) ||
			(r.URL.Path != "/me" && r.Method != http.MethodPost) {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		mux.ServeHTTP(w, r)
	})
}
func decodeCredentials(r *http.Request) (credentials, error) {
	var input credentials
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := decoder.Decode(&input); err != nil {
		return input, errors.New("invalid JSON body")
	}
	return input, nil
}
func (a *apiServer) register(w http.ResponseWriter, r *http.Request) {
	input, err := decodeCredentials(r)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	user, err := a.auth.Register(input.Email, input.Password)
	if err != nil {
		status := 400
		if errors.Is(err, errEmailExists) {
			status = 409
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, user)
}
func (a *apiServer) login(w http.ResponseWriter, r *http.Request) {
	input, err := decodeCredentials(r)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	token, err := a.auth.Login(input.Email, input.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	writeJSON(w, 200, tokenResponse{AccessToken: token, TokenType: "Bearer"})
}
func (a *apiServer) me(w http.ResponseWriter, r *http.Request) {
	claims, _ := claimsFromContext(r.Context())
	writeJSON(w, 200, PublicUser{ID: claims.Subject, Email: claims.Email})
}
func (a *apiServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fields := strings.Fields(r.Header.Get("Authorization"))
		if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
			writeError(w, 401, "authorization required")
			return
		}
		claims, err := a.auth.tokens.Validate(fields[1])
		if err != nil {
			writeError(w, 401, "invalid access token")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey, claims)))
	})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, apiError{Error: message})
}
