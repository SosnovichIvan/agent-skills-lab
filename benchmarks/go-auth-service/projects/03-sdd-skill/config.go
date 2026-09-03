package main

import (
	"errors"
	"os"
	"strconv"
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
	address := os.Getenv("AUTH_ADDR")
	if address == "" {
		address = ":8080"
	}
	issuer := os.Getenv("AUTH_ISSUER")
	if issuer == "" {
		issuer = "auth-service"
	}
	secret := strings.TrimSpace(os.Getenv("AUTH_SECRET"))
	if len([]byte(secret)) < 32 {
		return Config{}, errors.New("AUTH_SECRET must be at least 32 bytes")
	}
	ttl := 15 * time.Minute
	if raw := os.Getenv("AUTH_TOKEN_TTL"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds <= 0 {
			return Config{}, errors.New("AUTH_TOKEN_TTL must be a positive number of seconds")
		}
		ttl = time.Duration(seconds) * time.Second
	}
	return Config{Address: address, Secret: []byte(secret), Issuer: issuer, TokenTTL: ttl}, nil
}
