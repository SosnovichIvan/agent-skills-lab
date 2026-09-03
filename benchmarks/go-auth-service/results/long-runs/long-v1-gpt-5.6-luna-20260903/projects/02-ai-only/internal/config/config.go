// Package config contains process configuration and its environment loader.
package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Address    string
	Issuer     string
	HMACSecret string

	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	RateLimitRequests int
	RateLimitWindow   time.Duration
	LoginMaxFailures  int
	LoginLockDuration time.Duration

	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// Load reads configuration from the process environment and validates it.
func Load() (Config, error) { return LoadFromEnv(os.Getenv) }

// LoadFromEnv is the injectable form of Load for callers with their own
// environment source.
func LoadFromEnv(getenv func(string) string) (Config, error) {
	if getenv == nil {
		return Config{}, errors.New("config: nil environment lookup")
	}

	c := Config{
		Address:           valueOr(getenv("IAM_ADDR"), ":8080"),
		Issuer:            valueOr(getenv("IAM_ISSUER"), "benchmark.local/iam"),
		HMACSecret:        getenv("IAM_HMAC_SECRET"),
		AccessTokenTTL:    durationOr(getenv("IAM_ACCESS_TOKEN_TTL"), 15*time.Minute),
		RefreshTokenTTL:   durationOr(getenv("IAM_REFRESH_TOKEN_TTL"), 30*24*time.Hour),
		RateLimitRequests: intOr(getenv("IAM_RATE_LIMIT_REQUESTS"), 60),
		RateLimitWindow:   durationOr(getenv("IAM_RATE_LIMIT_WINDOW"), time.Minute),
		LoginMaxFailures:  intOr(getenv("IAM_LOGIN_MAX_FAILURES"), 5),
		LoginLockDuration: durationOr(getenv("IAM_LOGIN_LOCK_DURATION"), 15*time.Minute),
		ReadTimeout:       durationOr(getenv("IAM_READ_TIMEOUT"), 5*time.Second),
		WriteTimeout:      durationOr(getenv("IAM_WRITE_TIMEOUT"), 10*time.Second),
		IdleTimeout:       durationOr(getenv("IAM_IDLE_TIMEOUT"), 60*time.Second),
		ShutdownTimeout:   durationOr(getenv("IAM_SHUTDOWN_TIMEOUT"), 10*time.Second),
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func durationOr(value string, fallback time.Duration) time.Duration {
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return -1
	}
	return duration
}

func intOr(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return -1
	}
	return n
}

// Validate checks configurations constructed directly by callers.
func (c Config) Validate() error {
	if len([]byte(c.HMACSecret)) < 32 {
		return errors.New("config: HMAC secret must be at least 32 bytes")
	}
	if strings.TrimSpace(c.Issuer) == "" {
		return errors.New("config: issuer must not be empty")
	}
	if c.Address == "" {
		return errors.New("config: address must not be empty")
	}
	if c.AccessTokenTTL <= 0 || c.RefreshTokenTTL <= 0 {
		return errors.New("config: token TTLs must be greater than zero")
	}
	if c.RateLimitRequests <= 0 || c.RateLimitWindow <= 0 {
		return errors.New("config: rate limit values must be greater than zero")
	}
	if c.LoginMaxFailures <= 0 || c.LoginLockDuration <= 0 {
		return errors.New("config: login lock values must be greater than zero")
	}
	if c.ReadTimeout <= 0 || c.WriteTimeout <= 0 || c.IdleTimeout <= 0 || c.ShutdownTimeout <= 0 {
		return errors.New("config: server timeouts must be greater than zero")
	}
	return nil
}
