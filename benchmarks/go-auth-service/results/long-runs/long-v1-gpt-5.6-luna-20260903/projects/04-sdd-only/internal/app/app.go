// Package app assembles the application-level dependencies.
package app

import (
	"net/http"
	"sync"

	"benchmark.local/iam/internal/config"
	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/httpapi"
	"benchmark.local/iam/internal/repository"
	"benchmark.local/iam/internal/security"
)

// Dependencies are the process-level dependencies passed into domain code.
type Dependencies struct {
	Clock              domain.Clock
	IDGenerator        domain.IDGenerator
	Users              repository.UserStore
	Sessions           repository.SessionStore
	PasswordResets     repository.PasswordResetStore
	EmailVerifications repository.EmailVerificationStore
	Organizations      repository.OrganizationStore
	Memberships        repository.MembershipStore
	Invites            repository.InviteStore
	Roles              repository.RoleStore
	APIKeys            repository.APIKeyStore
	Idempotency        repository.IdempotencyStore
	Audits             repository.AuditStore
	AccessToken        *security.AccessTokenService
	LoginGuard         *security.LoginGuard
	RateLimiter        *httpapi.RateLimiter
	Metrics            *httpapi.Metrics
}

// App is the root of the process dependency graph. Feature services and
// repositories are added here by subsequent implementation steps.
type App struct {
	Config             config.Config
	Clock              domain.Clock
	IDGenerator        domain.IDGenerator
	Users              repository.UserStore
	Sessions           repository.SessionStore
	PasswordResets     repository.PasswordResetStore
	EmailVerifications repository.EmailVerificationStore
	Organizations      repository.OrganizationStore
	Memberships        repository.MembershipStore
	Invites            repository.InviteStore
	Roles              repository.RoleStore
	APIKeys            repository.APIKeyStore
	Idempotency        repository.IdempotencyStore
	Audits             repository.AuditStore
	AccessToken        *security.AccessTokenService
	LoginGuard         *security.LoginGuard
	RateLimiter        *httpapi.RateLimiter
	Metrics            *httpapi.Metrics
	organizationMu     sync.Mutex
	idempotencyMu      sync.Mutex
	handler            http.Handler
}

// New constructs an application after configuration has been validated.
func New(cfg config.Config) (*App, error) {
	return NewWithDependencies(cfg, Dependencies{
		Clock:              domain.RealClock{},
		IDGenerator:        domain.CryptoIDGenerator{},
		Users:              repository.NewUserRepository(),
		Sessions:           repository.NewSessionRepository(),
		PasswordResets:     repository.NewPasswordResetRepository(),
		EmailVerifications: repository.NewEmailVerificationRepository(),
		Organizations:      repository.NewOrganizationRepository(),
		Memberships:        repository.NewMembershipRepository(),
		Invites:            repository.NewInviteRepository(),
		Roles:              repository.NewRoleRepository(),
		APIKeys:            repository.NewAPIKeyRepository(),
		Idempotency:        repository.NewIdempotencyRepository(),
		Audits:             repository.NewAuditRepository(),
	})
}

// NewWithDependencies constructs an application with explicit domain
// dependencies, which keeps business logic deterministic in callers and
// hidden integration tests.
func NewWithDependencies(cfg config.Config, deps Dependencies) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if deps.Clock == nil {
		return nil, domain.NewError(domain.KindInvalid, "clock dependency is required")
	}
	if deps.IDGenerator == nil {
		return nil, domain.NewError(domain.KindInvalid, "ID generator dependency is required")
	}
	if deps.Users == nil {
		return nil, domain.NewError(domain.KindInvalid, "user repository dependency is required")
	}
	if deps.Sessions == nil {
		return nil, domain.NewError(domain.KindInvalid, "session repository dependency is required")
	}
	if deps.PasswordResets == nil {
		return nil, domain.NewError(domain.KindInvalid, "password reset repository dependency is required")
	}
	if deps.EmailVerifications == nil {
		return nil, domain.NewError(domain.KindInvalid, "email verification repository dependency is required")
	}
	if deps.Organizations == nil {
		return nil, domain.NewError(domain.KindInvalid, "organization repository dependency is required")
	}
	if deps.Memberships == nil {
		return nil, domain.NewError(domain.KindInvalid, "membership repository dependency is required")
	}
	if deps.Invites == nil {
		return nil, domain.NewError(domain.KindInvalid, "invite repository dependency is required")
	}
	if deps.Roles == nil {
		return nil, domain.NewError(domain.KindInvalid, "role repository dependency is required")
	}
	if deps.APIKeys == nil {
		return nil, domain.NewError(domain.KindInvalid, "API key repository dependency is required")
	}
	if deps.Idempotency == nil {
		return nil, domain.NewError(domain.KindInvalid, "idempotency repository dependency is required")
	}
	if deps.Audits == nil {
		return nil, domain.NewError(domain.KindInvalid, "audit repository dependency is required")
	}
	if deps.AccessToken == nil {
		var err error
		deps.AccessToken, err = security.NewAccessTokenService(
			[]byte(cfg.HMACSecret), cfg.Issuer, deps.Clock, deps.IDGenerator, cfg.AccessTokenTTL,
		)
		if err != nil {
			return nil, domain.WrapError(domain.KindInvalid, "access token dependency is invalid", err)
		}
	}
	if deps.LoginGuard == nil {
		var err error
		deps.LoginGuard, err = security.NewLoginGuard(cfg.LoginMaxFailures, cfg.LoginLockDuration)
		if err != nil {
			return nil, domain.WrapError(domain.KindInvalid, "login guard dependency is invalid", err)
		}
	}
	if deps.RateLimiter == nil {
		deps.RateLimiter = httpapi.NewRateLimiter(cfg.RateLimitRequests, cfg.RateLimitWindow)
	}
	if deps.Metrics == nil {
		deps.Metrics = &httpapi.Metrics{}
	}
	mux := http.NewServeMux()
	application := &App{
		Config:             cfg,
		Clock:              deps.Clock,
		IDGenerator:        deps.IDGenerator,
		Users:              deps.Users,
		Sessions:           deps.Sessions,
		PasswordResets:     deps.PasswordResets,
		EmailVerifications: deps.EmailVerifications,
		Organizations:      deps.Organizations,
		Memberships:        deps.Memberships,
		Invites:            deps.Invites,
		Roles:              deps.Roles,
		APIKeys:            deps.APIKeys,
		Idempotency:        deps.Idempotency,
		Audits:             deps.Audits,
		AccessToken:        deps.AccessToken,
		LoginGuard:         deps.LoginGuard,
		RateLimiter:        deps.RateLimiter,
		Metrics:            deps.Metrics,
		handler:            httpapi.RequestIDMiddleware(httpapi.LoggingMiddleware(httpapi.MetricsMiddleware(deps.Metrics, httpapi.LimitJSONBody(httpapi.RateLimitMiddleware(deps.RateLimiter, mux))))),
	}
	mux.HandleFunc("/v1/register", application.register)
	mux.HandleFunc("/v1/login", application.login)
	mux.HandleFunc("/v1/token/refresh", application.refresh)
	authenticated := func(handler http.Handler) http.Handler {
		return httpapi.BearerMiddleware(application.AccessToken, httpapi.RateLimitMiddleware(application.RateLimiter, handler))
	}
	mux.Handle("/v1/me", authenticated(http.HandlerFunc(application.me)))
	mux.Handle("/v1/logout", authenticated(http.HandlerFunc(application.logout)))
	mux.Handle("/v1/logout-all", authenticated(http.HandlerFunc(application.logoutAll)))
	mux.Handle("/v1/sessions", authenticated(http.HandlerFunc(application.sessions)))
	mux.Handle("/v1/sessions/", authenticated(http.HandlerFunc(application.sessionByID)))
	mux.Handle("/v1/password/change", authenticated(http.HandlerFunc(application.passwordChange)))
	mux.HandleFunc("/v1/password-reset/request", application.passwordResetRequest)
	mux.HandleFunc("/v1/password-reset/confirm", application.passwordResetConfirm)
	mux.Handle("/v1/email-verification/request", authenticated(http.HandlerFunc(application.emailVerificationRequest)))
	mux.HandleFunc("/v1/email-verification/confirm", application.emailVerificationConfirm)
	mux.Handle("/v1/organizations", authenticated(http.HandlerFunc(application.organizations)))
	organizationHandler := application.organizationPermissionMiddleware(http.HandlerFunc(application.organizationSubresource))
	organizationHandler = httpapi.RateLimitMiddleware(application.RateLimiter, organizationHandler)
	organizationHandler = httpapi.BearerMiddleware(application.AccessToken, organizationHandler)
	mux.Handle("/v1/organizations/", organizationHandler)
	mux.Handle("/v1/invites/", authenticated(http.HandlerFunc(application.acceptInvite)))
	mux.HandleFunc("/healthz", application.health)
	mux.HandleFunc("/readyz", application.ready)
	mux.HandleFunc("/metrics", application.metrics)
	return application, nil
}

// Handler returns the application's HTTP handler.
func (a *App) Handler() http.Handler { return a.handler }
