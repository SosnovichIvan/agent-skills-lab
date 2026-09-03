// Package app owns the top-level dependency wiring for the IAM process.
package app

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"benchmark.local/iam/internal/config"
	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/repository"
	"benchmark.local/iam/internal/security"
)

const MaxRequestBodyBytes int64 = 1 << 20

type App struct {
	Config          config.Config
	Clock           domain.Clock
	Users           *repository.UserRepository
	Sessions        *repository.SessionRepository
	Resets          *repository.PasswordResetRepository
	Verifications   *repository.EmailVerificationRepository
	Organizations   *repository.OrganizationRepository
	Memberships     *repository.MembershipRepository
	Invites         *repository.InviteRepository
	Roles           *repository.RoleRepository
	RoleAssignments *repository.RoleAssignmentRepository
	APIKeys         *repository.APIKeyRepository
	Audit           *repository.AuditRepository
	Tokens          *security.AccessTokenIssuer
	LoginGuard      *loginGuard
	OwnershipMu     sync.Mutex
	RateLimiter     *rateLimiter
	Idempotency     *idempotencyStore
	Logger          *slog.Logger
	Metrics         operationMetrics
}

type DataEnvelope struct {
	Data any `json:"data"`
}

type ErrorDetails struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

type ErrorEnvelope struct {
	Error ErrorDetails `json:"error"`
}

func New(cfg config.Config) (*App, error) {
	return NewWithClock(cfg, domain.SystemClock{})
}

func NewWithClock(cfg config.Config, clock domain.Clock) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if clock == nil {
		return nil, domain.Invalid("clock must not be nil")
	}
	tokens, err := security.NewAccessTokenIssuer([]byte(cfg.HMACSecret), cfg.Issuer, cfg.AccessTokenTTL, clock)
	if err != nil {
		return nil, domain.Invalid("access token configuration is invalid")
	}
	return &App{Config: cfg, Clock: clock, Users: repository.NewUserRepository(), Sessions: repository.NewSessionRepository(), Resets: repository.NewPasswordResetRepository(), Verifications: repository.NewEmailVerificationRepository(), Organizations: repository.NewOrganizationRepository(), Memberships: repository.NewMembershipRepository(), Invites: repository.NewInviteRepository(), Roles: repository.NewRoleRepository(), RoleAssignments: repository.NewRoleAssignmentRepository(), APIKeys: repository.NewAPIKeyRepository(), Audit: repository.NewAuditRepository(), Tokens: tokens, LoginGuard: newLoginGuard(), RateLimiter: newRateLimiter(), Idempotency: newIdempotencyStore(), Logger: slog.New(slog.NewJSONHandler(os.Stdout, nil))}, nil
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /v1/register", a.Idempotency.handler("register", http.HandlerFunc(a.register)))
	mux.HandleFunc("POST /v1/login", a.login)
	mux.HandleFunc("POST /v1/token/refresh", a.refresh)
	mux.Handle("GET /v1/me", a.requireAuth(http.HandlerFunc(a.me)))
	mux.Handle("POST /v1/logout", a.requireAuth(http.HandlerFunc(a.logout)))
	mux.Handle("POST /v1/logout-all", a.requireAuth(http.HandlerFunc(a.logoutAll)))
	mux.Handle("GET /v1/sessions", a.requireAuth(http.HandlerFunc(a.sessions)))
	mux.Handle("DELETE /v1/sessions/{sessionID}", a.requireAuth(http.HandlerFunc(a.deleteSession)))
	mux.Handle("POST /v1/password/change", a.requireAuth(http.HandlerFunc(a.changePassword)))
	mux.HandleFunc("POST /v1/password-reset/request", a.passwordResetRequest)
	mux.HandleFunc("POST /v1/password-reset/confirm", a.passwordResetConfirm)
	mux.Handle("POST /v1/email-verification/request", a.requireAuth(http.HandlerFunc(a.emailVerificationRequest)))
	mux.HandleFunc("POST /v1/email-verification/confirm", a.emailVerificationConfirm)
	mux.Handle("POST /v1/organizations", a.requireAuth(http.HandlerFunc(a.createOrganization)))
	mux.Handle("GET /v1/organizations", a.requireAuth(http.HandlerFunc(a.listOrganizations)))
	mux.Handle("POST /v1/organizations/{orgID}/invites", a.requireAuth(a.requirePermission(domain.PermissionMemberManage, a.Idempotency.handler("invite", http.HandlerFunc(a.createInvite)))))
	mux.Handle("POST /v1/invites/{token}/accept", a.requireAuth(http.HandlerFunc(a.acceptInvite)))
	mux.Handle("GET /v1/organizations/{orgID}/members", a.requireAuth(a.requirePermission(domain.PermissionMemberRead, http.HandlerFunc(a.listMembers))))
	mux.Handle("PATCH /v1/organizations/{orgID}/members/{userID}", a.requireAuth(a.requirePermission(domain.PermissionMemberManage, http.HandlerFunc(a.updateMember))))
	mux.Handle("DELETE /v1/organizations/{orgID}/members/{userID}", a.requireAuth(a.requirePermission(domain.PermissionMemberManage, http.HandlerFunc(a.deleteMember))))
	mux.Handle("POST /v1/organizations/{orgID}/ownership-transfer", a.requireAuth(a.requirePermission(domain.PermissionOrgManage, http.HandlerFunc(a.transferOwnership))))
	mux.Handle("PUT /v1/organizations/{orgID}/members/{userID}/roles/{roleID}", a.requireAuth(a.requirePermission(domain.PermissionRoleManage, http.HandlerFunc(a.assignRole))))
	mux.Handle("DELETE /v1/organizations/{orgID}/members/{userID}/roles/{roleID}", a.requireAuth(a.requirePermission(domain.PermissionRoleManage, http.HandlerFunc(a.removeRole))))
	mux.Handle("POST /v1/organizations/{orgID}/api-keys", a.requireAuth(a.requirePermission(domain.PermissionAPIKeyManage, http.HandlerFunc(a.createAPIKey))))
	mux.Handle("GET /v1/organizations/{orgID}/api-keys", a.requireAuth(a.requirePermission(domain.PermissionAPIKeyRead, http.HandlerFunc(a.listAPIKeys))))
	mux.Handle("DELETE /v1/organizations/{orgID}/api-keys/{keyID}", a.requireAuth(a.requirePermission(domain.PermissionAPIKeyManage, http.HandlerFunc(a.revokeAPIKey))))
	mux.Handle("GET /v1/organizations/{orgID}/audit", a.requireAuth(a.requirePermission(domain.PermissionAuditRead, http.HandlerFunc(a.listAudit))))
	mux.HandleFunc("GET /metrics", a.metrics)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeData(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		writeData(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			if id, err := domain.NewID(); err == nil {
				requestID = string(id)
			} else {
				requestID = "unknown"
			}
		}
		w.Header().Set("X-Request-ID", requestID)
		a.Metrics.requests.Add(1)
		logRequest(a.Logger, r, requestID)
		if !a.RateLimiter.allow("ip:"+clientIP(r), a.Clock.Now().UTC(), a.Config.RateLimitRequests, a.Config.RateLimitWindow) {
			a.Metrics.errors.Add(1)
			writeError(w, r, http.StatusTooManyRequests, "rate_limited", "too many requests")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodyBytes)
		mux.ServeHTTP(w, r)
	})
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (a *App) register(w http.ResponseWriter, r *http.Request) {
	var request registerRequest
	if err := decodeJSON(r, &request); err != nil {
		if _, tooLarge := err.(*http.MaxBytesError); tooLarge {
			writeError(w, r, http.StatusRequestEntityTooLarge, "body_too_large", "request body is too large")
			return
		}
		writeError(w, r, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}
	email, err := domain.NormalizeEmail(request.Email)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_email", "email is invalid")
		return
	}
	material, err := security.HashPassword(request.Password)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_password", "password does not meet policy")
		return
	}
	id, err := domain.NewUserID()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to create user")
		return
	}
	now := a.Clock.Now()
	user := domain.User{
		ID:                id,
		Email:             domain.Email(email),
		EmailVerification: domain.EmailUnverified,
		PasswordMaterial:  material,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := a.Users.Create(user); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			writeError(w, r, http.StatusConflict, "conflict", "email is already registered")
			return
		}
		writeError(w, r, http.StatusBadRequest, "invalid_request", "user could not be created")
		return
	}
	a.audit("", user.ID, "user.registered", string(user.ID))
	writeData(w, http.StatusCreated, user.PublicProfile())
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if err := decodeJSON(r, &request); err != nil {
		if _, tooLarge := err.(*http.MaxBytesError); tooLarge {
			writeError(w, r, http.StatusRequestEntityTooLarge, "body_too_large", "request body is too large")
			return
		}
		writeUnauthorized(w, r)
		return
	}
	identity := strings.ToLower(strings.TrimSpace(request.Email))
	loginNow := a.Clock.Now().UTC()
	if a.LoginGuard.locked(identity, loginNow) {
		writeUnauthorized(w, r)
		return
	}
	email, err := domain.NormalizeEmail(request.Email)
	if err != nil {
		a.LoginGuard.recordFailedLogin(identity, loginNow, a.Config.LoginMaxFailures, a.Config.LoginLockDuration)
		writeUnauthorized(w, r)
		return
	}
	user, err := a.Users.FindByEmail(domain.Email(email))
	if err != nil || !security.VerifyPassword(user.PasswordMaterial, request.Password) {
		a.LoginGuard.recordFailedLogin(identity, loginNow, a.Config.LoginMaxFailures, a.Config.LoginLockDuration)
		writeUnauthorized(w, r)
		return
	}
	a.LoginGuard.resetFailedLoginCounter(identity)
	a.audit("", user.ID, "user.logged_in", string(user.ID))
	token, err := a.Tokens.Issue(user.ID, user.Email)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to create access token")
		return
	}
	refreshToken, err := security.NewRefreshToken()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to create refresh token")
		return
	}
	sessionID, err := domain.NewSessionID()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to create session")
		return
	}
	familyID, err := domain.NewID()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to create session family")
		return
	}
	now := a.Clock.Now().UTC()
	if err := a.Sessions.Create(domain.Session{
		ID:               sessionID,
		UserID:           user.ID,
		RefreshTokenHash: security.HashRefreshToken(refreshToken),
		ExpiresAt:        now.Add(a.Config.RefreshTokenTTL),
		FamilyID:         familyID,
		CreatedAt:        now,
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to create session")
		return
	}
	writeData(w, http.StatusOK, map[string]any{
		"access_token":  token,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    int64(a.Config.AccessTokenTTL / time.Second),
	})
}

func (a *App) refresh(w http.ResponseWriter, r *http.Request) {
	var request refreshRequest
	if err := decodeJSON(r, &request); err != nil {
		if _, tooLarge := err.(*http.MaxBytesError); tooLarge {
			writeError(w, r, http.StatusRequestEntityTooLarge, "body_too_large", "request body is too large")
			return
		}
		writeUnauthorized(w, r)
		return
	}
	if request.RefreshToken == "" {
		writeUnauthorized(w, r)
		return
	}
	now := a.Clock.Now().UTC()
	oldHash := security.HashRefreshToken(request.RefreshToken)
	oldSession, err := a.Sessions.FindByRefreshTokenHash(oldHash)
	if err != nil {
		writeUnauthorized(w, r)
		return
	}
	if oldSession.Revoked {
		a.Sessions.RevokeFamily(oldSession.FamilyID)
		writeUnauthorized(w, r)
		return
	}
	if !now.Before(oldSession.ExpiresAt) {
		writeUnauthorized(w, r)
		return
	}
	user, err := a.Users.Get(oldSession.UserID)
	if err != nil {
		writeUnauthorized(w, r)
		return
	}
	accessToken, err := a.Tokens.Issue(user.ID, user.Email)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to create access token")
		return
	}
	refreshToken, err := security.NewRefreshToken()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to create refresh token")
		return
	}
	newSessionID, err := domain.NewSessionID()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to create session")
		return
	}
	replacement := domain.Session{
		ID:               newSessionID,
		UserID:           oldSession.UserID,
		RefreshTokenHash: security.HashRefreshToken(refreshToken),
		ExpiresAt:        now.Add(a.Config.RefreshTokenTTL),
		FamilyID:         oldSession.FamilyID,
		CreatedAt:        now,
	}
	if err := a.Sessions.Rotate(oldHash, replacement); err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			writeUnauthorized(w, r)
			return
		}
		if errors.Is(err, domain.ErrTokenReuse) {
			a.Sessions.RevokeFamily(oldSession.FamilyID)
			writeUnauthorized(w, r)
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", "unable to rotate refresh token")
		return
	}
	a.audit("", user.ID, "session.rotated", string(replacement.ID))
	writeData(w, http.StatusOK, map[string]any{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    int64(a.Config.AccessTokenTTL / time.Second),
	})
}

func writeUnauthorized(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, http.StatusUnauthorized, "unauthorized", "invalid credentials")
}

func decodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func writeData(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(DataEnvelope{Data: data})
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorEnvelope{Error: ErrorDetails{
		Code: code, Message: message, RequestID: r.Header.Get("X-Request-ID"),
	}})
}
