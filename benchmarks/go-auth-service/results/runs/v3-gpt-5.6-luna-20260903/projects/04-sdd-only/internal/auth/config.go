package auth

import (
	"errors"
	"os"
	"strings"
	"time"
)

type Config struct {
	Address  string
	Secret   []byte
	Issuer   string
	TokenTTL time.Duration
}

func LoadConfig() (Config, error) {
	c := Config{Address: envOr("AUTH_ADDRESS", ":8080"), Issuer: envOr("AUTH_ISSUER", "auth-service"), TokenTTL: time.Hour}
	secret := os.Getenv("AUTH_SECRET")
	if len([]byte(secret)) < 32 {
		return Config{}, errors.New("AUTH_SECRET must contain at least 32 bytes")
	}
	c.Secret = []byte(secret)
	if raw := os.Getenv("AUTH_TOKEN_TTL"); raw != "" {
		ttl, err := time.ParseDuration(raw)
		if err != nil || ttl <= 0 {
			return Config{}, errors.New("AUTH_TOKEN_TTL must be a positive duration")
		}
		c.TokenTTL = ttl
	}
	if c.Address == "" || strings.TrimSpace(c.Issuer) == "" {
		return Config{}, errors.New("AUTH_ADDRESS and AUTH_ISSUER must not be empty")
	}
	return c, nil
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
