// Package config contains the process configuration and its validation.
package config

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAddress           = ":8080"
	defaultIssuer            = "iam"
	defaultAccessTTL         = 15 * time.Minute
	defaultRefreshTTL        = 30 * 24 * time.Hour
	defaultPasswordResetTTL  = time.Hour
	defaultReadTimeout       = 10 * time.Second
	defaultWriteTimeout      = 10 * time.Second
	defaultIdleTimeout       = 60 * time.Second
	defaultShutdownGrace     = 10 * time.Second
	defaultLoginLockDuration = 15 * time.Minute
)

// Config is the immutable configuration used to construct the service.
// HMACSecret is kept as bytes so callers do not need to repeatedly convert it.
type Config struct {
	Address           string
	Issuer            string
	HMACSecret        []byte
	AccessTTL         time.Duration
	RefreshTTL        time.Duration
	PasswordResetTTL  time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownGrace     time.Duration
	LoginRateLimit    int
	LoginLockDuration time.Duration
	APIKeyRateLimit   int
	RateLimit         int
	RateLimitWindow   time.Duration
	IdempotencyTTL    time.Duration
}

// Load reads IAM_* environment variables and validates the resulting config.
// IAM_HMAC_SECRET is mandatory and must contain at least 32 bytes.
func Load() (Config, error) {
	secret := os.Getenv("IAM_HMAC_SECRET")
	if secret == "" {
		return Config{}, errors.New("IAM_HMAC_SECRET is required")
	}

	c := Config{
		Address:           envString("IAM_ADDRESS", defaultAddress),
		Issuer:            envString("IAM_ISSUER", defaultIssuer),
		HMACSecret:        []byte(secret),
		AccessTTL:         envDuration("IAM_ACCESS_TTL", defaultAccessTTL),
		RefreshTTL:        envDuration("IAM_REFRESH_TTL", defaultRefreshTTL),
		PasswordResetTTL:  envDuration("IAM_PASSWORD_RESET_TTL", defaultPasswordResetTTL),
		ReadTimeout:       envDuration("IAM_READ_TIMEOUT", defaultReadTimeout),
		WriteTimeout:      envDuration("IAM_WRITE_TIMEOUT", defaultWriteTimeout),
		IdleTimeout:       envDuration("IAM_IDLE_TIMEOUT", defaultIdleTimeout),
		ShutdownGrace:     envDuration("IAM_SHUTDOWN_GRACE", defaultShutdownGrace),
		LoginRateLimit:    envInt("IAM_LOGIN_RATE_LIMIT", 10),
		LoginLockDuration: envDuration("IAM_LOGIN_LOCK_DURATION", defaultLoginLockDuration),
		APIKeyRateLimit:   envInt("IAM_APIKEY_RATE_LIMIT", 60),
		RateLimit:         envInt("IAM_RATE_LIMIT", 60),
		RateLimitWindow:   envDuration("IAM_RATE_LIMIT_WINDOW", time.Minute),
		IdempotencyTTL:    envDuration("IAM_IDEMPOTENCY_TTL", 24*time.Hour),
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Validate rejects unsafe or unusable runtime settings.
func (c Config) Validate() error {
	if strings.TrimSpace(c.Address) == "" {
		return errors.New("address must not be empty")
	}
	if strings.TrimSpace(c.Issuer) == "" {
		return errors.New("issuer must not be empty")
	}
	if len(c.HMACSecret) < 32 {
		return errors.New("HMAC secret must be at least 32 bytes")
	}
	for name, value := range map[string]time.Duration{
		"access TTL": c.AccessTTL, "refresh TTL": c.RefreshTTL,
		"login lock duration": c.LoginLockDuration,
		"read timeout":        c.ReadTimeout, "write timeout": c.WriteTimeout,
		"idle timeout": c.IdleTimeout, "shutdown grace": c.ShutdownGrace,
	} {
		if value <= 0 {
			return fmt.Errorf("%s must be positive", name)
		}
	}
	if c.RefreshTTL < c.AccessTTL {
		return errors.New("refresh TTL must not be shorter than access TTL")
	}
	if c.LoginRateLimit <= 0 || c.APIKeyRateLimit <= 0 || c.RateLimit <= 0 {
		return errors.New("rate limits must be positive")
	}
	if c.RateLimitWindow <= 0 || c.IdempotencyTTL <= 0 {
		return errors.New("rate limit window and idempotency TTL must be positive")
	}
	return nil
}

// GenerateSecret returns a cryptographically random secret suitable for local setup.
func GenerateSecret(size int) ([]byte, error) {
	if size < 32 {
		return nil, errors.New("secret size must be at least 32 bytes")
	}
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}
	return b, nil
}

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return -1
	}
	return duration
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	result, err := strconv.Atoi(value)
	if err != nil {
		return -1
	}
	return result
}
