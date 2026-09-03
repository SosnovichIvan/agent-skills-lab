package main

import (
	"errors"
	"os"
	"time"
)

type Config struct {
	Address  string
	Secret   []byte
	Issuer   string
	TokenTTL time.Duration
}

func loadConfig() (Config, error) {
	c := Config{Address: getenv("AUTH_ADDR", ":8080"), Issuer: getenv("AUTH_ISSUER", "auth-service"), TokenTTL: time.Hour}
	if raw := os.Getenv("AUTH_TOKEN_TTL"); raw != "" {
		ttl, err := time.ParseDuration(raw)
		if err != nil || ttl <= 0 {
			return Config{}, errors.New("AUTH_TOKEN_TTL must be a positive duration")
		}
		c.TokenTTL = ttl
	}
	secret := os.Getenv("AUTH_SECRET")
	if len([]byte(secret)) < 32 {
		return Config{}, errors.New("AUTH_SECRET must be at least 32 bytes")
	}
	c.Secret = []byte(secret)
	if c.Issuer == "" {
		return Config{}, errors.New("AUTH_ISSUER must not be empty")
	}
	return c, nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

var errInvalidCredentials = errors.New("invalid email or password")
var errEmailExists = errors.New("email already registered")
