// Package app wires process configuration to the HTTP server dependencies.
package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
)

// App is the top-level HTTP application. Domain dependencies are added here as
// they are introduced, keeping construction in one place.
type App struct {
	cfg                config.Config
	handler            http.Handler
	users              *repository.UserRepository
	sessions           *repository.SessionRepository
	passwordResets     *repository.PasswordResetRepository
	emailVerifications *repository.EmailVerificationRepository
	organizations      *repository.OrganizationRepository
	memberships        *repository.MembershipRepository
	roles              *repository.RoleRepository
	invites            *repository.InviteRepository
	apiKeys            *repository.APIKeyRepository
	audit              *repository.AuditRepository
	clock              domain.Clock
	loginGuard         *loginGuard
	rateLimiter        *rateLimiter
	idempotency        *idempotencyStore
	metrics            *metrics
	logger             *slog.Logger
	organizationMu     sync.Mutex
}

const maxJSONBodySize int64 = 1 << 20

// New constructs an application from validated configuration.
func New(cfg config.Config) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	metrics := &metrics{}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	mux.HandleFunc("GET /healthz", health)
	mux.HandleFunc("GET /readyz", ready)
	mux.HandleFunc("GET /metrics", metricsHandler(metrics))
	users := repository.NewUserRepository()
	sessions := repository.NewSessionRepository()
	passwordResets := repository.NewPasswordResetRepository()
	emailVerifications := repository.NewEmailVerificationRepository()
	organizations := repository.NewOrganizationRepository()
	memberships := repository.NewMembershipRepository()
	roles := repository.NewRoleRepository()
	invites := repository.NewInviteRepository()
	apiKeys := repository.NewAPIKeyRepository()
	audit := repository.NewAuditRepository()
	application := &App{cfg: cfg, users: users, sessions: sessions, passwordResets: passwordResets, emailVerifications: emailVerifications, organizations: organizations, memberships: memberships, roles: roles, invites: invites, apiKeys: apiKeys, audit: audit, clock: domain.RealClock{}, loginGuard: newLoginGuard(), rateLimiter: newRateLimiter(cfg.RateLimit, cfg.RateLimitWindow), idempotency: newIdempotencyStore(cfg.IdempotencyTTL), metrics: metrics, logger: logger}
	mux.HandleFunc("POST /v1/register", application.register)
	mux.HandleFunc("POST /v1/login", application.login)
	mux.HandleFunc("POST /v1/token/refresh", application.refresh)
	mux.Handle("GET /v1/me", application.bearerMiddleware(http.HandlerFunc(application.me)))
	mux.Handle("POST /v1/logout-all", application.bearerMiddleware(http.HandlerFunc(application.logoutAll)))
	mux.Handle("POST /v1/password/change", application.bearerMiddleware(http.HandlerFunc(application.changePassword)))
	mux.HandleFunc("POST /v1/password-reset/request", application.passwordResetRequest)
	mux.HandleFunc("POST /v1/password-reset/confirm", application.passwordResetConfirm)
	mux.HandleFunc("POST /v1/email-verification/request", application.emailVerificationRequest)
	mux.HandleFunc("POST /v1/email-verification/confirm", application.emailVerificationConfirm)
	mux.Handle("GET /v1/sessions", application.bearerMiddleware(http.HandlerFunc(application.listSessions)))
	mux.Handle("DELETE /v1/sessions/{sessionID}", application.bearerMiddleware(http.HandlerFunc(application.deleteSession)))
	mux.Handle("POST /v1/organizations", application.bearerMiddleware(http.HandlerFunc(application.createOrganization)))
	mux.Handle("GET /v1/organizations", application.bearerMiddleware(http.HandlerFunc(application.listOrganizations)))
	mux.Handle("GET /v1/organizations/{orgID}/members", application.organizationMiddleware(domain.PermissionMemberRead, http.HandlerFunc(application.listMembers)))
	mux.Handle("PATCH /v1/organizations/{orgID}/members/{userID}", application.organizationMiddleware(domain.PermissionMemberManage, http.HandlerFunc(application.updateMember)))
	mux.Handle("DELETE /v1/organizations/{orgID}/members/{userID}", application.organizationMiddleware(domain.PermissionMemberManage, http.HandlerFunc(application.deleteMember)))
	mux.Handle("POST /v1/organizations/{orgID}/invites", application.organizationMiddleware(domain.PermissionMemberManage, http.HandlerFunc(application.createInvite)))
	mux.Handle("POST /v1/invites/{token}/accept", application.bearerMiddleware(http.HandlerFunc(application.acceptInvite)))
	mux.Handle("POST /v1/organizations/{orgID}/ownership-transfer", application.organizationMiddleware(domain.PermissionOrgManage, http.HandlerFunc(application.transferOwnership)))
	mux.Handle("POST /v1/organizations/{orgID}/roles", application.organizationMiddleware(domain.PermissionRoleManage, http.HandlerFunc(application.createRole)))
	mux.Handle("GET /v1/organizations/{orgID}/roles", application.organizationMiddleware(domain.PermissionRoleRead, http.HandlerFunc(application.listRoles)))
	mux.Handle("PUT /v1/organizations/{orgID}/members/{userID}/roles/{roleID}", application.organizationMiddleware(domain.PermissionRoleManage, http.HandlerFunc(application.assignRole)))
	mux.Handle("DELETE /v1/organizations/{orgID}/members/{userID}/roles/{roleID}", application.organizationMiddleware(domain.PermissionRoleManage, http.HandlerFunc(application.removeRole)))
	mux.Handle("POST /v1/organizations/{orgID}/api-keys", application.organizationMiddleware(domain.PermissionAPIKeyManage, http.HandlerFunc(application.createAPIKey)))
	mux.Handle("GET /v1/organizations/{orgID}/api-keys", application.organizationMiddleware(domain.PermissionAPIKeyRead, http.HandlerFunc(application.listAPIKeys)))
	mux.Handle("DELETE /v1/organizations/{orgID}/api-keys/{keyID}", application.organizationMiddleware(domain.PermissionAPIKeyManage, http.HandlerFunc(application.revokeAPIKey)))
	mux.Handle("GET /v1/organizations/{orgID}/audit", application.organizationMiddleware(domain.PermissionAuditRead, http.HandlerFunc(application.listAudit)))
	mux.HandleFunc("POST /v1/logout", application.logout)
	application.handler = withRequestID(application.rateLimitMiddleware(application.observabilityMiddleware(mux)))
	return application, nil
}

// Handler returns the HTTP handler used by cmd/iamd.
func (a *App) Handler() http.Handler { return a.handler }

// Server returns a configured net/http server.
func (a *App) Server() *http.Server {
	return &http.Server{
		Addr:         a.cfg.Address,
		Handler:      a.handler,
		ReadTimeout:  a.cfg.ReadTimeout,
		WriteTimeout: a.cfg.WriteTimeout,
		IdleTimeout:  a.cfg.IdleTimeout,
	}
}

func health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, dataEnvelope{Data: map[string]string{"status": "ok"}})
}

func ready(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, dataEnvelope{Data: map[string]string{"status": "ready"}})
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

type dataEnvelope struct {
	Data any `json:"data"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func (a *App) register(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	w.Header().Set("X-Request-ID", requestID)
	idemKey, fingerprint, idemErr := idempotencyRequest(r)
	var idemEntry *idempotencyEntry
	if idemErr != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid idempotency request", requestID)
		return
	}
	if idemKey != "" {
		idemStorageKey := "register:" + clientIP(r) + ":" + idemKey
		idemEntry, body, status, err := a.idempotency.begin(idemStorageKey, fingerprint, a.clock.Now().UTC())
		if err != nil {
			writeError(w, http.StatusConflict, "idempotency_conflict", err.Error(), requestID)
			return
		}
		if body != nil {
			replay(w, status, body)
			return
		}
		defer a.idempotency.fail(idemStorageKey, idemEntry)
		r = r.WithContext(context.WithValue(r.Context(), idempotencyKeyContext{}, idemStorageKey))
	}
	var input registerRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body", requestID)
		return
	}
	email, err := domain.ValidateEmail(input.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid email", requestID)
		return
	}
	material, err := domain.HashPassword(input.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error(), requestID)
		return
	}
	id, err := domain.NewUserID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	now := a.clock.Now().UTC()
	user := domain.User{
		ID:                     id,
		Email:                  email,
		EmailVerificationState: domain.EmailVerificationPending,
		PasswordMaterial:       material,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if err := a.users.Create(user); err != nil {
		if domain.IsKind(err, domain.ErrConflict) {
			writeError(w, http.StatusConflict, "conflict", "email already exists", requestID)
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	response := dataEnvelope{Data: user.PublicProfile()}
	if idemEntry != nil {
		key := r.Context().Value(idempotencyKeyContext{}).(string)
		a.idempotency.complete(key, idemEntry, http.StatusCreated, marshalResponse(response))
	}
	writeJSON(w, http.StatusCreated, response)
}

// login authenticates an account and issues a short-lived access token. Every
// credential failure uses the same response, including an unknown or invalid
// email, so callers cannot use this endpoint to enumerate accounts.
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	w.Header().Set("X-Request-ID", requestID)
	var input registerRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body", requestID)
		return
	}

	identity := domain.NormalizeEmail(input.Email)
	if a.loginGuard.isLocked(identity, a.clock.Now().UTC()) {
		writeUnauthorized(w, requestID)
		return
	}
	email, err := domain.ValidateEmail(input.Email)
	if err != nil {
		a.loginGuard.failure(identity, a.clock.Now().UTC(), a.cfg.LoginRateLimit, a.cfg.LoginLockDuration)
		writeUnauthorized(w, requestID)
		return
	}
	user, err := a.users.GetByEmail(email)
	if err != nil || !domain.VerifyPassword(input.Password, user.PasswordMaterial) {
		a.loginGuard.failure(identity, a.clock.Now().UTC(), a.cfg.LoginRateLimit, a.cfg.LoginLockDuration)
		writeUnauthorized(w, requestID)
		return
	}
	a.loginGuard.success(identity)

	issuer := domain.AccessTokenIssuer{
		Secret: a.cfg.HMACSecret,
		Issuer: a.cfg.Issuer,
		TTL:    a.cfg.AccessTTL,
		Clock:  a.clock,
	}
	token, claims, err := issuer.Issue(string(user.ID), user.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	refreshToken, err := domain.NewRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	familyID, err := domain.NewTokenFamilyID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	sessionID, err := domain.NewSessionID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	now := a.clock.Now().UTC()
	if err := a.sessions.Create(domain.Session{ID: sessionID, UserID: user.ID, TokenFamilyID: familyID, CreatedAt: now, ExpiresAt: now.Add(a.cfg.RefreshTTL)}, refreshToken); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	a.recordUserAudit(r, user.ID, "session.created")
	writeJSON(w, http.StatusOK, dataEnvelope{Data: loginResponse{
		AccessToken:  token,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    claims.Expiry - claims.Issued,
	}})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (a *App) refresh(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	w.Header().Set("X-Request-ID", requestID)
	var input refreshRequest
	if err := decodeJSON(r, &input); err != nil || !domain.ValidRefreshToken(input.RefreshToken) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid refresh token", requestID)
		return
	}
	old, err := a.sessions.GetByRefreshToken(input.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid refresh token", requestID)
		return
	}
	now := a.clock.Now().UTC()
	if old.ExpiresAt.IsZero() || !now.Before(old.ExpiresAt) || old.Revoked {
		if old.Revoked {
			a.sessions.RevokeFamily(old.TokenFamilyID)
		}
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid refresh token", requestID)
		return
	}
	user, err := a.users.GetByID(old.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid refresh token", requestID)
		return
	}
	access, claims, err := (domain.AccessTokenIssuer{Secret: a.cfg.HMACSecret, Issuer: a.cfg.Issuer, TTL: a.cfg.AccessTTL, Clock: a.clock}).Issue(string(user.ID), user.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	newRefresh, err := domain.NewRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	newID, err := domain.NewSessionID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	_, err = a.sessions.Rotate(input.RefreshToken, domain.Session{ID: newID, UserID: old.UserID, TokenFamilyID: old.TokenFamilyID, CreatedAt: now, ExpiresAt: old.ExpiresAt}, newRefresh)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid refresh token", requestID)
		return
	}
	a.recordUserAudit(r, old.UserID, "session.rotated")
	writeJSON(w, http.StatusOK, dataEnvelope{Data: loginResponse{AccessToken: access, RefreshToken: newRefresh, TokenType: "Bearer", ExpiresIn: claims.Expiry - claims.Issued}})
}

// logout revokes the session represented by a refresh token. A well-formed
// but already unknown token is treated as success, and the token is never
// echoed in a response or error.
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	w.Header().Set("X-Request-ID", requestID)
	var input refreshRequest
	if err := decodeJSON(r, &input); err != nil || !domain.ValidRefreshToken(input.RefreshToken) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid refresh token", requestID)
		return
	}
	if session, err := a.sessions.GetByRefreshToken(input.RefreshToken); err == nil {
		_ = a.sessions.Revoke(session.ID)
		a.recordUserAudit(r, session.UserID, "session.revoked")
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: map[string]string{"status": "logged_out"}})
}

func (a *App) logoutAll(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	w.Header().Set("X-Request-ID", requestID)
	claims, ok := ClaimsFromContext(r.Context())
	if !ok || strings.TrimSpace(claims.Subject) == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid access token", requestID)
		return
	}
	a.sessions.RevokeByUser(domain.UserID(claims.Subject))
	a.recordUserAudit(r, domain.UserID(claims.Subject), "sessions.revoked")
	writeJSON(w, http.StatusOK, dataEnvelope{Data: map[string]string{"status": "logged_out"}})
}

type passwordChangeRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type passwordResetRequest struct {
	Email string `json:"email"`
}

type emailVerificationRequest struct {
	Email string `json:"email"`
}

type emailVerificationConfirmRequest struct {
	Token string `json:"token"`
}

func (a *App) emailVerificationRequest(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	w.Header().Set("X-Request-ID", requestID)
	var input emailVerificationRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body", requestID)
		return
	}
	token, err := domain.NewEmailVerificationToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	if email, validationErr := domain.ValidateEmail(input.Email); validationErr == nil {
		if user, lookupErr := a.users.GetByEmail(email); lookupErr == nil && !user.EmailVerified {
			ttl := a.cfg.PasswordResetTTL
			if ttl <= 0 {
				ttl = time.Hour
			}
			_ = a.emailVerifications.Create(user.ID, token, a.clock.Now().UTC().Add(ttl))
		}
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: map[string]string{
		"status": "if_account_exists_verification_instructions_sent", "verification_token": token,
	}})
}

func (a *App) emailVerificationConfirm(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	w.Header().Set("X-Request-ID", requestID)
	var input emailVerificationConfirmRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body", requestID)
		return
	}
	if !domain.ValidEmailVerificationToken(input.Token) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid email verification token", requestID)
		return
	}
	userID, err := a.emailVerifications.Consume(input.Token, a.clock.Now().UTC())
	if err != nil {
		record, lookupErr := a.emailVerifications.Get(input.Token)
		if lookupErr == nil && record.Used {
			if user, userErr := a.users.GetByID(record.UserID); userErr == nil && user.EmailVerified {
				writeJSON(w, http.StatusOK, dataEnvelope{Data: map[string]string{"status": "email_verified"}})
				return
			}
		}
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid email verification token", requestID)
		return
	}
	user, err := a.users.GetByID(userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid email verification token", requestID)
		return
	}
	if !user.EmailVerified {
		user.EmailVerified = true
		user.EmailVerificationState = domain.EmailVerificationVerified
		user.UpdatedAt = a.clock.Now().UTC()
		if err := a.users.Update(user); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
			return
		}
		a.recordUserAudit(r, user.ID, "email.verified")
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: map[string]string{"status": "email_verified"}})
}

type passwordResetConfirmRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (a *App) passwordResetRequest(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	w.Header().Set("X-Request-ID", requestID)
	var input passwordResetRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body", requestID)
		return
	}
	// A token is generated for every request so the response shape is identical
	// for known and unknown addresses, preventing account enumeration.
	token, err := domain.NewPasswordResetToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	resetTTL := a.cfg.PasswordResetTTL
	if resetTTL <= 0 {
		resetTTL = time.Hour
	}
	if email, validationErr := domain.ValidateEmail(input.Email); validationErr == nil {
		if user, lookupErr := a.users.GetByEmail(email); lookupErr == nil {
			_ = a.passwordResets.Create(user.ID, token, a.clock.Now().UTC().Add(resetTTL))
		}
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: map[string]string{
		"status": "if_account_exists_reset_instructions_sent", "reset_token": token,
	}})
}

func (a *App) passwordResetConfirm(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	w.Header().Set("X-Request-ID", requestID)
	var input passwordResetConfirmRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body", requestID)
		return
	}
	if !domain.ValidPasswordResetToken(input.Token) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid password reset token", requestID)
		return
	}
	if err := domain.ValidatePassword(input.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error(), requestID)
		return
	}
	userID, err := a.passwordResets.Consume(input.Token, a.clock.Now().UTC())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid password reset token", requestID)
		return
	}
	user, err := a.users.GetByID(userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid password reset token", requestID)
		return
	}
	material, err := domain.HashPassword(input.NewPassword)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error(), requestID)
		return
	}
	user.PasswordMaterial = material
	user.UpdatedAt = a.clock.Now().UTC()
	if err := a.users.Update(user); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	a.sessions.RevokeByUser(userID)
	a.recordUserAudit(r, userID, "password.reset")
	writeJSON(w, http.StatusOK, dataEnvelope{Data: map[string]string{"status": "password_reset"}})
}

func (a *App) changePassword(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	w.Header().Set("X-Request-ID", requestID)
	claims, ok := ClaimsFromContext(r.Context())
	if !ok || strings.TrimSpace(claims.Subject) == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid access token", requestID)
		return
	}

	var input passwordChangeRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body", requestID)
		return
	}
	if err := domain.ValidatePassword(input.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error(), requestID)
		return
	}

	userID := domain.UserID(claims.Subject)
	user, err := a.users.GetByID(userID)
	if err != nil || !domain.VerifyPassword(input.CurrentPassword, user.PasswordMaterial) {
		writeUnauthorized(w, requestID)
		return
	}
	material, err := domain.HashPassword(input.NewPassword)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error(), requestID)
		return
	}
	user.PasswordMaterial = material
	user.UpdatedAt = a.clock.Now().UTC()
	if err := a.users.Update(user); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
		return
	}
	a.sessions.RevokeByUser(userID)
	for _, membership := range a.memberships.ListByUser(userID) {
		a.recordAudit(r, membership.OrganizationID, "password.changed", string(userID), nil)
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: map[string]string{"status": "password_changed"}})
}

func (a *App) listSessions(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	w.Header().Set("X-Request-ID", requestID)
	claims, ok := ClaimsFromContext(r.Context())
	if !ok || strings.TrimSpace(claims.Subject) == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid access token", requestID)
		return
	}
	sessions := a.sessions.ListByUser(domain.UserID(claims.Subject))
	if sessions == nil {
		sessions = []domain.Session{}
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: sessions})
}

func (a *App) deleteSession(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	w.Header().Set("X-Request-ID", requestID)
	claims, ok := ClaimsFromContext(r.Context())
	if !ok || strings.TrimSpace(claims.Subject) == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid access token", requestID)
		return
	}
	requestedID := domain.SessionID(r.PathValue("sessionID"))
	session, err := a.sessions.GetByID(requestedID)
	// Do not distinguish a missing session from a session belonging to another
	// account. This both enforces the tenant boundary and avoids an identifier
	// oracle.
	if err != nil || session.UserID != domain.UserID(claims.Subject) {
		writeError(w, http.StatusNotFound, "not_found", "session not found", requestID)
		return
	}
	if err := a.sessions.Revoke(requestedID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "session not found", requestID)
		return
	}
	a.recordUserAudit(r, domain.UserID(claims.Subject), "session.revoked")
	writeJSON(w, http.StatusOK, dataEnvelope{Data: map[string]string{"status": "logged_out"}})
}

type accessTokenClaimsContextKey struct{}
type apiKeyScopesContextKey struct{}
type apiKeyOrganizationContextKey struct{}

// ClaimsFromContext returns the claims attached by bearerMiddleware. It is
// intentionally the only way handlers need to know how authentication state
// is represented in a request context.
func ClaimsFromContext(ctx context.Context) (domain.AccessTokenClaims, bool) {
	claims, ok := ctx.Value(accessTokenClaimsContextKey{}).(domain.AccessTokenClaims)
	return claims, ok
}

func apiKeyScopesFromContext(ctx context.Context) ([]domain.Permission, bool) {
	scopes, ok := ctx.Value(apiKeyScopesContextKey{}).([]domain.Permission)
	return scopes, ok
}
func apiKeyOrganizationFromContext(ctx context.Context) (domain.OrganizationID, bool) {
	orgID, ok := ctx.Value(apiKeyOrganizationContextKey{}).(domain.OrganizationID)
	return orgID, ok
}

func (a *App) bearerMiddleware(next http.Handler) http.Handler {
	validator := domain.AccessTokenValidator{
		Secret: a.cfg.HMACSecret,
		Issuer: a.cfg.Issuer,
		Clock:  a.clock,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		credential := strings.TrimSpace(r.Header.Get("X-API-Key"))
		parts := strings.Fields(strings.TrimSpace(r.Header.Get("Authorization")))
		if credential == "" && len(parts) == 2 && strings.EqualFold(parts[0], "ApiKey") {
			credential = parts[1]
		}
		if credential != "" {
			if !domain.ValidAPIKey(credential) {
				writeError(w, http.StatusUnauthorized, "unauthorized", "invalid access token", requestID(r))
				return
			}
			key, err := a.apiKeys.GetByKey(domain.HashAPIKey(credential))
			if err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized", "invalid access token", requestID(r))
				return
			}
			if !a.rateLimiter.allow("identity:"+string(key.CreatedBy), a.clock.Now().UTC()) {
				w.Header().Set("Retry-After", "60")
				writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests", requestID(r))
				return
			}
			claims := domain.AccessTokenClaims{Subject: string(key.CreatedBy)}
			ctx := context.WithValue(r.Context(), accessTokenClaimsContextKey{}, claims)
			ctx = context.WithValue(ctx, apiKeyScopesContextKey{}, key.Scopes)
			ctx = context.WithValue(ctx, apiKeyOrganizationContextKey{}, key.OrganizationID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid access token", requestID(r))
			return
		}
		claims, err := validator.Validate(parts[1])
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid access token", requestID(r))
			return
		}
		if !a.rateLimiter.allow("identity:"+claims.Subject, a.clock.Now().UTC()) {
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests", requestID(r))
			return
		}
		ctx := context.WithValue(r.Context(), accessTokenClaimsContextKey{}, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// organizationMiddleware authenticates the request, resolves membership from
// the current repository state, and checks the current role's permissions.
// Organization identity always comes from the URL, so a token cannot cross a
// tenant boundary by carrying authorization data of its own.
func (a *App) organizationMiddleware(permission domain.Permission, next http.Handler) http.Handler {
	return a.bearerMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := requestID(r)
		claims, ok := ClaimsFromContext(r.Context())
		orgID := domain.OrganizationID(r.PathValue("orgID"))
		if !ok || strings.TrimSpace(claims.Subject) == "" || strings.TrimSpace(string(orgID)) == "" {
			writeError(w, http.StatusForbidden, "forbidden", "organization membership required", requestID)
			return
		}
		membership, err := a.memberships.Get(orgID, domain.UserID(claims.Subject))
		if err != nil || !a.memberHasPermission(membership, permission) {
			writeError(w, http.StatusForbidden, "forbidden", "organization permission required", requestID)
			return
		}
		if scopes, isAPIKey := apiKeyScopesFromContext(r.Context()); isAPIKey {
			keyOrg, _ := apiKeyOrganizationFromContext(r.Context())
			if keyOrg != orgID {
				writeError(w, http.StatusForbidden, "forbidden", "organization permission required", requestID)
				return
			}
			allowed := false
			for _, scope := range scopes {
				if scope == permission {
					allowed = true
					break
				}
			}
			if !allowed {
				writeError(w, http.StatusForbidden, "forbidden", "api key scope required", requestID)
				return
			}
			// The key is resolved by bearerMiddleware and its organization is
			// checked here through the key owner membership and URL tenant.
		}
		next.ServeHTTP(w, r)
	}))
}

func (a *App) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid access token", requestID(r))
		return
	}
	user, err := a.users.GetByID(domain.UserID(claims.Subject))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid access token", requestID(r))
		return
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: user.PublicProfile()})
}

func writeUnauthorized(w http.ResponseWriter, requestID string) {
	writeError(w, http.StatusUnauthorized, "unauthorized", "invalid credentials", requestID)
}

func decodeJSON(r *http.Request, destination any) error {
	limited := &countingReader{reader: io.LimitReader(r.Body, maxJSONBodySize+1)}
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if limited.count > maxJSONBodySize {
		return errors.New("request body too large")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	if limited.count > maxJSONBodySize {
		return errors.New("request body too large")
	}
	return nil
}

type countingReader struct {
	reader io.Reader
	count  int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.count += int64(n)
	return n, err
}

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := requestID(r)
		r.Header.Set("X-Request-ID", id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

func requestID(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-Request-ID")); value != "" {
		return value
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return hex.EncodeToString(raw[:])
	}
	return "unknown"
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message, requestID string) {
	writeJSON(w, status, errorEnvelope{Error: errorBody{Code: code, Message: message, RequestID: requestID}})
}
