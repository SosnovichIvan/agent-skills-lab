package config

import (
	"errors"
	"fmt"
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

func Load() (Config, error) {
	secret := strings.TrimSpace(os.Getenv("AUTH_SECRET"))
	if len([]byte(secret)) < 32 {
		return Config{}, errors.New("AUTH_SECRET must be set and contain at least 32 bytes")
	}
	issuer := strings.TrimSpace(os.Getenv("AUTH_ISSUER"))
	if issuer == "" {
		issuer = "authservice"
	}
	address := strings.TrimSpace(os.Getenv("AUTH_ADDR"))
	if address == "" {
		address = ":8080"
	}
	ttl := 15 * time.Minute
	if raw := strings.TrimSpace(os.Getenv("AUTH_TOKEN_TTL")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed <= 0 || parsed > 24*time.Hour {
			return Config{}, fmt.Errorf("invalid AUTH_TOKEN_TTL %q", raw)
		}
		ttl = parsed
	}
	return Config{Address: address, Secret: []byte(secret), Issuer: issuer, TokenTTL: ttl}, nil
}
