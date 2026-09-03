package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr     string
	Secret   []byte
	Issuer   string
	TokenTTL time.Duration
}

func loadConfig() (Config, error) {
	secret := os.Getenv("AUTH_SECRET")
	if len(secret) < 32 {
		return Config{}, errors.New("AUTH_SECRET must be at least 32 bytes")
	}
	issuer, addr := os.Getenv("AUTH_ISSUER"), os.Getenv("AUTH_ADDR")
	if issuer == "" {
		issuer = "go-auth-service"
	}
	if addr == "" {
		addr = ":8080"
	}
	ttl := 15 * time.Minute
	if raw := os.Getenv("AUTH_TOKEN_TTL"); raw != "" {
		v, e := time.ParseDuration(raw)
		if e != nil || v <= 0 {
			return Config{}, fmt.Errorf("invalid AUTH_TOKEN_TTL: %w", e)
		}
		ttl = v
	}
	if raw := os.Getenv("AUTH_TOKEN_TTL_SECONDS"); raw != "" {
		v, e := strconv.Atoi(raw)
		if e != nil || v <= 0 {
			return Config{}, errors.New("invalid AUTH_TOKEN_TTL_SECONDS")
		}
		ttl = time.Duration(v) * time.Second
	}
	return Config{addr, []byte(secret), issuer, ttl}, nil
}
