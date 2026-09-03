package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Address      string
	Issuer       string
	Secret       []byte
	TokenTTL     time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

func FromEnvironment() (Config, error) {
	secret := os.Getenv("AUTH_SECRET")
	if len([]byte(secret)) < 32 {
		return Config{}, errors.New("AUTH_SECRET must be at least 32 bytes")
	}

	c := Config{
		Address:      envOr("AUTH_ADDRESS", ":8080"),
		Issuer:       envOr("AUTH_ISSUER", "auth-service"),
		Secret:       []byte(secret),
		TokenTTL:     15 * time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	if value := os.Getenv("AUTH_TOKEN_TTL"); value != "" {
		ttl, err := time.ParseDuration(value)
		if err != nil || ttl <= 0 {
			return Config{}, fmt.Errorf("AUTH_TOKEN_TTL must be a positive duration: %w", err)
		}
		c.TokenTTL = ttl
	}
	for name, target := range map[string]*time.Duration{
		"AUTH_READ_TIMEOUT":  &c.ReadTimeout,
		"AUTH_WRITE_TIMEOUT": &c.WriteTimeout,
		"AUTH_IDLE_TIMEOUT":  &c.IdleTimeout,
	} {
		if value := os.Getenv(name); value != "" {
			seconds, err := strconv.Atoi(value)
			if err != nil || seconds <= 0 {
				return Config{}, fmt.Errorf("%s must be a positive number of seconds", name)
			}
			*target = time.Duration(seconds) * time.Second
		}
	}
	return c, nil
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
