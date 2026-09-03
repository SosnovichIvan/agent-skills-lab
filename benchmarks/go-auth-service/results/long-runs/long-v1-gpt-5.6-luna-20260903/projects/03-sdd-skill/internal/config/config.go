// Package config contains the process configuration for the IAM service.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config is the configuration shared by the service dependencies.
type Config struct {
	Address               string
	Issuer                string
	HMACSecret            string
	AccessTTL             time.Duration
	RefreshTTL            time.Duration
	PasswordResetTTL      time.Duration
	EmailVerificationTTL  time.Duration
	InviteTTL             time.Duration
	ReadTimeout           time.Duration
	WriteTimeout          time.Duration
	IdleTimeout           time.Duration
	ShutdownTimeout       time.Duration
	LoginRateLimit        int
	LoginFailureThreshold int
	LoginLockDuration     time.Duration
	RegisterRateLimit     int
	InviteRateLimit       int
	RateLimitWindow       time.Duration
}

// Load reads configuration from environment variables and validates it.
func Load() (Config, error) {
	c := Config{
		Address:               envOr("IAM_ADDR", ":8080"),
		Issuer:                envOr("IAM_ISSUER", "benchmark.local/iam"),
		HMACSecret:            os.Getenv("IAM_HMAC_SECRET"),
		AccessTTL:             durationOr("IAM_ACCESS_TTL", time15Minutes),
		RefreshTTL:            durationOr("IAM_REFRESH_TTL", 30*24*time.Hour),
		PasswordResetTTL:      durationOr("IAM_PASSWORD_RESET_TTL", 15*time.Minute),
		EmailVerificationTTL:  durationOr("IAM_EMAIL_VERIFICATION_TTL", 24*time.Hour),
		InviteTTL:             durationOr("IAM_INVITE_TTL", 7*24*time.Hour),
		ReadTimeout:           durationOr("IAM_READ_TIMEOUT", 5*time.Second),
		WriteTimeout:          durationOr("IAM_WRITE_TIMEOUT", 10*time.Second),
		IdleTimeout:           durationOr("IAM_IDLE_TIMEOUT", 60*time.Second),
		ShutdownTimeout:       durationOr("IAM_SHUTDOWN_TIMEOUT", 10*time.Second),
		LoginRateLimit:        intOr("IAM_LOGIN_RATE_LIMIT", 10),
		LoginFailureThreshold: intOr("IAM_LOGIN_FAILURE_THRESHOLD", 5),
		LoginLockDuration:     durationOr("IAM_LOGIN_LOCK_DURATION", 15*time.Minute),
		RegisterRateLimit:     intOr("IAM_REGISTER_RATE_LIMIT", 5),
		InviteRateLimit:       intOr("IAM_INVITE_RATE_LIMIT", 10),
		RateLimitWindow:       durationOr("IAM_RATE_LIMIT_WINDOW", time.Minute),
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

const time15Minutes = 15 * time.Minute

// Validate checks security-sensitive and server timing settings.
func (c Config) Validate() error {
	if len([]byte(c.HMACSecret)) < 32 {
		return errors.New("IAM_HMAC_SECRET must be at least 32 bytes")
	}
	if c.Address == "" {
		return errors.New("IAM_ADDR must not be empty")
	}
	if c.Issuer == "" {
		return errors.New("IAM_ISSUER must not be empty")
	}
	for name, value := range map[string]time.Duration{
		"access TTL": c.AccessTTL, "refresh TTL": c.RefreshTTL,
		"password reset TTL":     c.PasswordResetTTL,
		"email verification TTL": c.EmailVerificationTTL,
		"invite TTL":             c.InviteTTL,
		"read timeout":           c.ReadTimeout, "write timeout": c.WriteTimeout,
		"idle timeout": c.IdleTimeout, "shutdown timeout": c.ShutdownTimeout,
	} {
		if value <= 0 {
			return fmt.Errorf("%s must be greater than zero", name)
		}
	}
	if c.LoginRateLimit <= 0 || c.LoginFailureThreshold <= 0 || c.LoginLockDuration <= 0 || c.RegisterRateLimit <= 0 || c.InviteRateLimit <= 0 || c.RateLimitWindow <= 0 {
		return errors.New("rate limits must be greater than zero")
	}
	return nil
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func durationOr(name string, fallback time.Duration) time.Duration {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return -1
	}
	return parsed
}

func intOr(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return -1
	}
	return parsed
}
