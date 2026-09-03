package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
func config() (string, string, string, time.Duration, error) {
	secret := os.Getenv("AUTH_SECRET")
	if len(secret) < 32 {
		return "", "", "", 0, errors.New("AUTH_SECRET must be at least 32 bytes")
	}
	issuer := env("AUTH_ISSUER", "auth-service")
	address := env("AUTH_ADDR", ":8080")
	ttlText := env("AUTH_TTL", "15m")
	ttl, err := time.ParseDuration(ttlText)
	if err != nil || ttl <= 0 {
		return "", "", "", 0, errors.New("AUTH_TTL must be a positive duration")
	}
	return secret, issuer, address, ttl, nil
}
func main() {
	secret, issuer, address, ttl, err := config()
	if err != nil {
		log.Fatal(err)
	}
	tokens := TokenManager{[]byte(secret), issuer, ttl}
	service := &AuthService{NewUserRepository(), tokens}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /register", service.registerHandler)
	mux.HandleFunc("POST /login", service.loginHandler)
	mux.Handle("GET /me", authMiddleware(tokens, http.HandlerFunc(meHandler)))
	server := &http.Server{Addr: address, Handler: mux, ReadTimeout: 10 * time.Second, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	stop, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	go func() {
		log.Printf("auth service listening on %s", address)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
		}
	}()
	<-stop.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
