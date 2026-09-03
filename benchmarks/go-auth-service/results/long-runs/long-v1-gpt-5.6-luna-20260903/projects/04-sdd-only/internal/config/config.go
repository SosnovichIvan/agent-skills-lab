// Package config contains the process configuration for the IAM service.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAddress           = ":8080"
	defaultIssuer            = "benchmark.local/iam"
	defaultAccessTokenTTL    = 15 * time.Minute
	defaultRefreshTokenTTL   = 30 * 24 * time.Hour
	defaultPasswordResetTTL  = 30 * time.Minute
	defaultEmailVerifyTTL    = 24 * time.Hour
	defaultInviteTTL         = 7 * 24 * time.Hour
	defaultReadTimeout       = 5 * time.Second
	defaultWriteTimeout      = 10 * time.Second
	defaultIdleTimeout       = 60 * time.Second
	defaultShutdownTimeout   = 10 * time.Second
	defaultLoginMaxFailures  = 5
	defaultLoginLockDuration = 15 * time.Minute
	defaultRateLimitRequests = 60
	defaultRateLimitWindow   = time.Minute
)

// Config contains values needed to construct the service and its HTTP server.
// Secret values are kept as strings here so process wiring can pass them to
// security services without introducing a second configuration source.
type Config struct {
	Address    string
	Issuer     string
	HMACSecret string

	AccessTokenTTL       time.Duration
	RefreshTokenTTL      time.Duration
	PasswordResetTTL     time.Duration
	EmailVerificationTTL time.Duration
	InviteTTL            time.Duration

	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration

	LoginMaxFailures  int
	LoginLockDuration time.Duration
	RateLimitRequests int
	RateLimitWindow   time.Duration
}

// Load reads the process configuration from environment variables.
func Load() (Config, error) {
	c := Config{
		Address:              envOr("IAM_ADDRESS", defaultAddress),
		Issuer:               envOr("IAM_ISSUER", defaultIssuer),
		HMACSecret:           os.Getenv("IAM_HMAC_SECRET"),
		AccessTokenTTL:       defaultAccessTokenTTL,
		RefreshTokenTTL:      defaultRefreshTokenTTL,
		PasswordResetTTL:     defaultPasswordResetTTL,
		EmailVerificationTTL: defaultEmailVerifyTTL,
		InviteTTL:            defaultInviteTTL,
		ReadTimeout:          defaultReadTimeout,
		WriteTimeout:         defaultWriteTimeout,
		IdleTimeout:          defaultIdleTimeout,
		ShutdownTimeout:      defaultShutdownTimeout,
		LoginMaxFailures:     defaultLoginMaxFailures,
		LoginLockDuration:    defaultLoginLockDuration,
		RateLimitRequests:    defaultRateLimitRequests,
		RateLimitWindow:      defaultRateLimitWindow,
	}

	var err error
	if c.AccessTokenTTL, err = durationEnv("IAM_ACCESS_TOKEN_TTL", c.AccessTokenTTL); err != nil {
		return Config{}, err
	}
	if c.RefreshTokenTTL, err = durationEnv("IAM_REFRESH_TOKEN_TTL", c.RefreshTokenTTL); err != nil {
		return Config{}, err
	}
	if c.PasswordResetTTL, err = durationEnv("IAM_PASSWORD_RESET_TTL", c.PasswordResetTTL); err != nil {
		return Config{}, err
	}
	if c.EmailVerificationTTL, err = durationEnv("IAM_EMAIL_VERIFICATION_TTL", c.EmailVerificationTTL); err != nil {
		return Config{}, err
	}
	if c.InviteTTL, err = durationEnv("IAM_INVITE_TTL", c.InviteTTL); err != nil {
		return Config{}, err
	}
	if c.ReadTimeout, err = durationEnv("IAM_READ_TIMEOUT", c.ReadTimeout); err != nil {
		return Config{}, err
	}
	if c.WriteTimeout, err = durationEnv("IAM_WRITE_TIMEOUT", c.WriteTimeout); err != nil {
		return Config{}, err
	}
	if c.IdleTimeout, err = durationEnv("IAM_IDLE_TIMEOUT", c.IdleTimeout); err != nil {
		return Config{}, err
	}
	if c.ShutdownTimeout, err = durationEnv("IAM_SHUTDOWN_TIMEOUT", c.ShutdownTimeout); err != nil {
		return Config{}, err
	}
	if c.LoginLockDuration, err = durationEnv("IAM_LOGIN_LOCK_DURATION", c.LoginLockDuration); err != nil {
		return Config{}, err
	}
	if c.RateLimitWindow, err = durationEnv("IAM_RATE_LIMIT_WINDOW", c.RateLimitWindow); err != nil {
		return Config{}, err
	}
	if raw := os.Getenv("IAM_RATE_LIMIT_REQUESTS"); raw != "" {
		c.RateLimitRequests, err = strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("IAM_RATE_LIMIT_REQUESTS: %w", err)
		}
	}
	if raw := os.Getenv("IAM_LOGIN_MAX_FAILURES"); raw != "" {
		c.LoginMaxFailures, err = strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("IAM_LOGIN_MAX_FAILURES: %w", err)
		}
	}

	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Validate checks security-sensitive configuration and values used by the
// HTTP server. In particular, a short HMAC secret is never accepted.
func (c Config) Validate() error {
	if strings.TrimSpace(c.Address) == "" {
		return errors.New("address must not be empty")
	}
	if strings.TrimSpace(c.Issuer) == "" {
		return errors.New("issuer must not be empty")
	}
	if len([]byte(c.HMACSecret)) < 32 {
		return errors.New("HMAC secret must be at least 32 bytes")
	}
	for name, value := range map[string]time.Duration{
		"access token TTL":       c.AccessTokenTTL,
		"refresh token TTL":      c.RefreshTokenTTL,
		"password reset TTL":     c.PasswordResetTTL,
		"email verification TTL": c.EmailVerificationTTL,
		"invite TTL":             c.InviteTTL,
		"read timeout":           c.ReadTimeout,
		"write timeout":          c.WriteTimeout,
		"idle timeout":           c.IdleTimeout,
		"shutdown timeout":       c.ShutdownTimeout,
		"login lock duration":    c.LoginLockDuration,
	} {
		if value <= 0 {
			return fmt.Errorf("%s must be positive", name)
		}
	}
	if c.LoginMaxFailures <= 0 {
		return errors.New("login max failures must be positive")
	}
	if c.RateLimitRequests <= 0 {
		return errors.New("rate limit requests must be positive")
	}
	if c.RateLimitWindow <= 0 {
		return errors.New("rate limit window must be positive")
	}
	return nil
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func durationEnv(name string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return value, nil
}
