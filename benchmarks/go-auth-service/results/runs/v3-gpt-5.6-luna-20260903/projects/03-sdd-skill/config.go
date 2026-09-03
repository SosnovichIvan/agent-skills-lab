package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Secret    []byte
	Issuer    string
	Addr      string
	TokenTTL  time.Duration
	ReadLimit int64
}

func loadConfig() (Config, error) {
	secret := os.Getenv("AUTH_SECRET")
	if len([]byte(secret)) < 32 {
		return Config{}, errors.New("AUTH_SECRET must be at least 32 bytes")
	}
	issuer := os.Getenv("AUTH_ISSUER")
	if issuer == "" {
		issuer = "auth-service"
	}
	addr := os.Getenv("AUTH_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	ttl := 15 * time.Minute
	if raw := os.Getenv("AUTH_TOKEN_TTL"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed <= 0 {
			return Config{}, fmt.Errorf("invalid AUTH_TOKEN_TTL: %q", raw)
		}
		ttl = parsed
	}
	limit := int64(1 << 20)
	if raw := os.Getenv("AUTH_BODY_LIMIT"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			return Config{}, fmt.Errorf("invalid AUTH_BODY_LIMIT: %q", raw)
		}
		limit = parsed
	}
	return Config{Secret: []byte(secret), Issuer: issuer, Addr: addr, TokenTTL: ttl, ReadLimit: limit}, nil
}
