// Package app assembles the process-level service dependencies.
package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"benchmark.local/iam/internal/config"
	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/repository"
)

const maxJSONBodyBytes int64 = 1 << 20
const defaultPasswordResetTTL = 15 * time.Minute
const defaultEmailVerificationTTL = 24 * time.Hour
const defaultInviteTTL = 7 * 24 * time.Hour
const defaultLoginFailureThreshold = 5
const defaultLoginLockDuration = 15 * time.Minute
const defaultRateLimitWindow = time.Minute

var errRequestBodyTooLarge = errors.New("request body too large")

// App is the assembled IAM application. Domain dependencies are added here as
// their packages are introduced by subsequent implementation tasks.
type App struct {
	Config             config.Config
	Clock              domain.Clock
	Users              *repository.UserRepository
	Organizations      *repository.OrganizationRepository
	Memberships        *repository.MembershipRepository
	Roles              *repository.RoleRepository
	Sessions           *repository.SessionRepository
	PasswordResets     *repository.PasswordResetRepository
	EmailVerifications *repository.EmailVerificationRepository
	Invites            *repository.InviteRepository
	APIKeys            *repository.APIKeyRepository
	Audit              *repository.AuditRepository
	AccessTokens       *domain.AccessTokenIssuer
	AccessValidator    *domain.AccessTokenValidator
	dummyPassword      domain.PasswordMaterial
	loginFailures      loginFailureTracker
	rateLimiter        requestRateLimiter
	idempotency        idempotencyStore
	organizationMu     sync.Mutex
	logger             *log.Logger
	requests           atomic.Uint64
	requestErrors      atomic.Uint64
	Handler            http.Handler
}

// loginFailureTracker keeps temporary lockouts keyed by normalized identity.
// It is deliberately process-local, like the in-memory user repository.
type loginFailureTracker struct {
	mu      sync.Mutex
	entries map[string]loginFailureEntry
}

type loginFailureEntry struct {
	failures    int
	lockedUntil time.Time
}

type requestRateLimiter struct {
	mu      sync.Mutex
	entries map[string]rateEntry
}
type rateEntry struct {
	started time.Time
	count   int
}

func (l *requestRateLimiter) allow(key string, limit int, window time.Duration, now time.Time) bool {
	if limit <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.entries == nil {
		l.entries = make(map[string]rateEntry)
	}
	e := l.entries[key]
	if e.started.IsZero() || !now.Before(e.started.Add(window)) {
		e = rateEntry{started: now}
	}
	if e.count >= limit {
		l.entries[key] = e
		return false
	}
	e.count++
	l.entries[key] = e
	return true
}

type idempotencyRecord struct {
	fingerprint string
	status      int
	header      http.Header
	body        []byte
}
type idempotencyStore struct {
	mu      sync.Mutex
	records map[string]idempotencyRecord
}

func (t *loginFailureTracker) locked(identity string, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	entry, ok := t.entries[identity]
	if !ok {
		return false
	}
	if !entry.lockedUntil.IsZero() && now.Before(entry.lockedUntil) {
		return true
	}
	if !entry.lockedUntil.IsZero() {
		delete(t.entries, identity)
	}
	return false
}

func (t *loginFailureTracker) failed(identity string, now time.Time, threshold int, duration time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.entries == nil {
		t.entries = make(map[string]loginFailureEntry)
	}
	entry := t.entries[identity]
	entry.failures++
	if entry.failures >= threshold {
		entry.lockedUntil = now.Add(duration)
	}
	t.entries[identity] = entry
}

func (t *loginFailureTracker) succeeded(identity string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, identity)
}

// New builds the application from validated configuration.
func New(cfg config.Config) *App {
	return NewWithClock(cfg, domain.RealClock{})
}

// NewWithClock assembles the application with an explicit time dependency.
// Tests and domain services can therefore use a deterministic clock.
func NewWithClock(cfg config.Config, clock domain.Clock) *App {
	if clock == nil {
		clock = domain.RealClock{}
	}
	if cfg.PasswordResetTTL <= 0 {
		cfg.PasswordResetTTL = defaultPasswordResetTTL
	}
	if cfg.EmailVerificationTTL <= 0 {
		cfg.EmailVerificationTTL = defaultEmailVerificationTTL
	}
	if cfg.InviteTTL <= 0 {
		cfg.InviteTTL = defaultInviteTTL
	}
	if cfg.LoginFailureThreshold <= 0 {
		cfg.LoginFailureThreshold = defaultLoginFailureThreshold
	}
	if cfg.LoginLockDuration <= 0 {
		cfg.LoginLockDuration = defaultLoginLockDuration
	}
	if cfg.InviteRateLimit <= 0 {
		cfg.InviteRateLimit = 10
	}
	if cfg.RateLimitWindow <= 0 {
		cfg.RateLimitWindow = defaultRateLimitWindow
	}
	app := &App{
		Config:             cfg,
		Clock:              clock,
		Users:              repository.NewUserRepository(),
		Organizations:      repository.NewOrganizationRepository(),
		Memberships:        repository.NewMembershipRepository(),
		Roles:              repository.NewRoleRepository(),
		Sessions:           repository.NewSessionRepository(),
		PasswordResets:     repository.NewPasswordResetRepository(),
		EmailVerifications: repository.NewEmailVerificationRepository(),
		Invites:            repository.NewInviteRepository(),
		APIKeys:            repository.NewAPIKeyRepository(),
		Audit:              repository.NewAuditRepository(),
		loginFailures:      loginFailureTracker{entries: make(map[string]loginFailureEntry)},
		rateLimiter:        requestRateLimiter{entries: make(map[string]rateEntry)},
		idempotency:        idempotencyStore{records: make(map[string]idempotencyRecord)},
	}
	app.AccessTokens, _ = domain.NewAccessTokenIssuer(
		[]byte(cfg.HMACSecret), cfg.Issuer, cfg.AccessTTL, clock,
	)
	app.AccessValidator, _ = domain.NewAccessTokenValidator(
		[]byte(cfg.HMACSecret), cfg.Issuer, clock,
	)
	// Verify failed logins for unknown identities against the same shape of
	// password record as a real user, without retaining a user's password.
	app.dummyPassword, _ = domain.HashPassword("invalid-login-password")
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", app.handleHealth)
	mux.HandleFunc("/readyz", app.handleReady)
	mux.HandleFunc("/metrics", app.handleMetrics)
	mux.HandleFunc("/v1/register", app.handleRegister)
	mux.HandleFunc("/v1/login", app.handleLogin)
	mux.HandleFunc("/v1/token/refresh", app.handleRefresh)
	mux.Handle("/v1/logout", app.BearerMiddleware(http.HandlerFunc(app.handleLogout)))
	mux.Handle("/v1/logout-all", app.BearerMiddleware(http.HandlerFunc(app.handleLogoutAll)))
	mux.Handle("/v1/sessions", app.BearerMiddleware(http.HandlerFunc(app.handleSessions)))
	mux.Handle("/v1/sessions/", app.BearerMiddleware(http.HandlerFunc(app.handleSession)))
	mux.Handle("/v1/password/change", app.BearerMiddleware(http.HandlerFunc(app.handlePasswordChange)))
	mux.HandleFunc("/v1/password-reset/request", app.handlePasswordResetRequest)
	mux.HandleFunc("/v1/password-reset/confirm", app.handlePasswordResetConfirm)
	mux.HandleFunc("/v1/email-verification/request", app.handleEmailVerificationRequest)
	mux.HandleFunc("/v1/email-verification/confirm", app.handleEmailVerificationConfirm)
	mux.Handle("/v1/me", app.BearerMiddleware(http.HandlerFunc(app.handleMe)))
	mux.Handle("/v1/organizations", app.BearerMiddleware(http.HandlerFunc(app.handleOrganizations)))
	mux.Handle("/v1/organizations/", app.BearerMiddleware(app.PermissionMiddleware(http.HandlerFunc(app.handleOrganizationSubresource))))
	mux.Handle("/v1/invites/", app.BearerMiddleware(http.HandlerFunc(app.handleInviteAccept)))
	app.logger = log.New(os.Stderr, "", 0)
	app.Handler = app.requestLoggingMiddleware(app.idempotencyMiddleware(app.rateLimitMiddleware(mux)))
	return app
}

type requestIDContextKey struct{}

type requestIDResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *requestIDResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *requestIDResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(data)
}

func newRequestID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "request-unknown"
	}
	return hex.EncodeToString(value[:])
}

func validRequestID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if !(char == '-' || char == '_' || char == '.' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9') {
			return false
		}
	}
	return true
}

func requestIDFromWriter(w http.ResponseWriter) string {
	if w == nil {
		return ""
	}
	return w.Header().Get("X-Request-ID")
}

func (a *App) requestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if !validRequestID(requestID) {
			requestID = newRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), requestIDContextKey{}, requestID)
		wrapped := &requestIDResponseWriter{ResponseWriter: w}
		started := time.Now()
		a.requests.Add(1)
		next.ServeHTTP(wrapped, r.WithContext(ctx))
		status := wrapped.status
		if status == 0 {
			status = http.StatusOK
		}
		if status >= 400 {
			a.requestErrors.Add(1)
		}
		entry := map[string]any{
			"event":       "http_request",
			"request_id":  requestID,
			"method":      r.Method,
			"path":        safeLogPath(r.URL.Path),
			"status":      status,
			"duration_ms": time.Since(started).Milliseconds(),
		}
		encoded, _ := json.Marshal(entry)
		if a.logger != nil {
			a.logger.Print(string(encoded))
		}
	})
}

func safeLogPath(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for index := range parts {
		if index > 0 && parts[index-1] == "invites" {
			parts[index] = "[redacted]"
		}
	}
	if len(parts) == 0 {
		return "/"
	}
	return "/" + strings.Join(parts, "/")
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: map[string]string{"status": "ok"}})
}

func (a *App) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
		return
	}
	if a == nil || a.Users == nil || a.AccessValidator == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "not_ready", "service is not ready")
		return
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: map[string]string{"status": "ready"}})
}

func (a *App) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "iam_http_requests_total %d\niam_http_errors_total %d\n", a.requests.Load(), a.requestErrors.Load())
}

type accessTokenClaimsContextKey struct{}
type apiKeyAuthContextKey struct{}
type apiKeyAuth struct {
	OrganizationID domain.OrganizationID
	Scopes         []domain.Permission
}

// AccessTokenClaimsFromContext returns the verified access-token claims put in
// the request context by BearerMiddleware.
func AccessTokenClaimsFromContext(ctx context.Context) (domain.AccessTokenClaims, bool) {
	claims, ok := ctx.Value(accessTokenClaimsContextKey{}).(domain.AccessTokenClaims)
	return claims, ok
}

// BearerMiddleware validates the Authorization bearer token and passes its
// claims to the downstream handler through the request context. Authentication
// failures use one response regardless of how the credential was malformed.
func (a *App) BearerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a == nil || a.AccessValidator == nil {
			writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid access token")
			return
		}
		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			token = strings.TrimSpace(r.Header.Get("X-API-Key"))
			ok = token != ""
		}
		if !ok {
			writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid access token")
			return
		}
		claims, err := a.AccessValidator.Validate(token)
		if err != nil {
			if a.APIKeys == nil {
				writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid access token")
				return
			}
			key, keyErr := a.APIKeys.Authenticate(token)
			if keyErr != nil {
				writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid access token")
				return
			}
			ctx := context.WithValue(r.Context(), accessTokenClaimsContextKey{}, domain.AccessTokenClaims{Sub: string(key.UserID)})
			ctx = context.WithValue(ctx, apiKeyAuthContextKey{}, apiKeyAuth{OrganizationID: key.OrganizationID, Scopes: key.Scopes})
			if !a.allowRequest(r, string(key.UserID)) {
				writeRateLimitError(w)
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		if !a.allowRequest(r, claims.Sub) {
			writeRateLimitError(w)
			return
		}
		ctx := context.WithValue(r.Context(), accessTokenClaimsContextKey{}, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requestIP(r *http.Request) string {
	if r == nil {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	if value := strings.TrimSpace(r.RemoteAddr); value != "" {
		return value
	}
	return "unknown"
}

func (a *App) requestLimit(r *http.Request) int {
	if strings.HasPrefix(r.URL.Path, "/v1/register") {
		return a.Config.RegisterRateLimit
	}
	if strings.HasPrefix(r.URL.Path, "/v1/organizations/") && strings.HasSuffix(r.URL.Path, "/invites") {
		return a.Config.InviteRateLimit
	}
	return a.Config.LoginRateLimit
}

func (a *App) allowRequest(r *http.Request, identity string) bool {
	limit := a.requestLimit(r)
	window := a.Config.RateLimitWindow
	if window <= 0 {
		window = time.Minute
	}
	now := a.Clock.Now()
	return a.rateLimiter.allow("ip:"+requestIP(r), limit, window, now) && a.rateLimiter.allow("identity:"+identity, limit, window, now)
}

func (a *App) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Authentication middleware applies the identity bucket after validating
		// the credential. Public requests still get an identity-independent
		// bucket, preventing a client from bypassing the IP limit with aliases.
		if !strings.HasPrefix(r.URL.Path, "/v1/") || strings.HasPrefix(r.URL.Path, "/v1/login") || strings.HasPrefix(r.URL.Path, "/v1/register") {
			if !a.rateLimiter.allow("ip:"+requestIP(r), a.requestLimit(r), a.Config.RateLimitWindow, a.Clock.Now()) {
				writeRateLimitError(w)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func writeRateLimitError(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "60")
	writeAPIError(w, http.StatusTooManyRequests, "rate_limited", "rate limit exceeded")
}

type bufferedResponseWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func newBufferedResponseWriter() *bufferedResponseWriter {
	return &bufferedResponseWriter{header: make(http.Header)}
}
func (w *bufferedResponseWriter) Header() http.Header { return w.header }
func (w *bufferedResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *bufferedResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(data)
}

func (a *App) idempotencyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if r.Method != http.MethodPost || key == "" || !(r.URL.Path == "/v1/register" || (strings.HasPrefix(r.URL.Path, "/v1/organizations/") && strings.HasSuffix(r.URL.Path, "/invites"))) {
			next.ServeHTTP(w, r)
			return
		}
		if len(key) > 200 {
			writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "invalid idempotency key")
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxJSONBodyBytes+1))
		if err != nil || int64(len(body)) > maxJSONBodyBytes {
			writeAPIError(w, http.StatusRequestEntityTooLarge, string(domain.ErrorInvalid), "request body too large")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		digest := sha256.Sum256(append(append([]byte(r.Method+" "+r.URL.Path+"\n"), body...), []byte("\n"+r.Header.Get("Authorization"))...))
		fingerprint := fmt.Sprintf("%x", digest[:])
		recordKey := r.URL.Path + "\x00" + key
		a.idempotency.mu.Lock()
		defer a.idempotency.mu.Unlock()
		if record, ok := a.idempotency.records[recordKey]; ok {
			if record.fingerprint != fingerprint {
				writeAPIError(w, http.StatusConflict, string(domain.ErrorConflict), "idempotency key reused with different request")
				return
			}
			for name, values := range record.header {
				for _, value := range values {
					w.Header().Add(name, value)
				}
			}
			w.WriteHeader(record.status)
			_, _ = w.Write(record.body)
			return
		}
		buffer := newBufferedResponseWriter()
		buffer.Header().Set("X-Request-ID", requestIDFromWriter(w))
		next.ServeHTTP(buffer, r)
		for name, values := range buffer.header {
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}
		status := buffer.status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_, _ = w.Write(buffer.body.Bytes())
		if status >= 200 && status < 300 {
			a.idempotency.records[recordKey] = idempotencyRecord{fingerprint: fingerprint, status: status, header: buffer.header.Clone(), body: append([]byte(nil), buffer.body.Bytes()...)}
		}
	})
}

// PermissionMiddleware enforces the tenant boundary and resolves permissions
// from the current membership and role repositories. Authorization data is
// intentionally not taken from access-token claims.
func (a *App) PermissionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := AccessTokenClaimsFromContext(r.Context())
		if !ok || a == nil || a.Memberships == nil || a.Organizations == nil {
			writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid access token")
			return
		}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/organizations/"), "/")
		if len(parts) < 2 || parts[0] == "" {
			writeAPIError(w, http.StatusNotFound, string(domain.ErrorNotFound), "organization resource not found")
			return
		}
		orgID := domain.OrganizationID(parts[0])
		if keyAuth, isKey := r.Context().Value(apiKeyAuthContextKey{}).(apiKeyAuth); isKey && keyAuth.OrganizationID != orgID {
			writeAPIError(w, http.StatusForbidden, string(domain.ErrorUnauthorized), "organization scope violation")
			return
		}
		if _, err := a.Organizations.GetByID(orgID); err != nil {
			writeAPIError(w, http.StatusNotFound, string(domain.ErrorNotFound), "organization not found")
			return
		}
		membership, err := a.Memberships.GetByOrganizationAndUser(orgID, domain.UserID(claims.Sub))
		if err != nil {
			writeAPIError(w, http.StatusForbidden, string(domain.ErrorUnauthorized), "organization membership required")
			return
		}
		permission, required := organizationPermission(parts, r.Method)
		if keyAuth, isKey := r.Context().Value(apiKeyAuthContextKey{}).(apiKeyAuth); isKey && required && !hasPermission(keyAuth.Scopes, permission) {
			writeAPIError(w, http.StatusForbidden, string(domain.ErrorUnauthorized), "api key scope required")
			return
		}
		if required && !a.membershipHasPermission(membership, permission) {
			writeAPIError(w, http.StatusForbidden, string(domain.ErrorUnauthorized), "organization permission required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func organizationPermission(parts []string, method string) (domain.Permission, bool) {
	if len(parts) < 2 {
		return "", false
	}
	switch parts[1] {
	case "members":
		if len(parts) == 2 && method == http.MethodGet {
			return domain.PermissionMemberRead, true
		}
		if len(parts) == 5 && parts[3] == "roles" && (method == http.MethodPut || method == http.MethodDelete) {
			return domain.PermissionRoleManage, true
		}
		if len(parts) == 3 && (method == http.MethodPatch || method == http.MethodDelete) {
			return domain.PermissionMemberManage, true
		}
	case "invites":
		if len(parts) == 2 && method == http.MethodPost {
			return domain.PermissionMemberManage, true
		}
	case "ownership-transfer":
		if len(parts) == 2 && method == http.MethodPost {
			return domain.PermissionOrgManage, true
		}
	case "roles":
		if len(parts) == 2 && method == http.MethodPost {
			return domain.PermissionRoleManage, true
		}
	case "api-keys":
		if len(parts) == 3 && method == http.MethodDelete {
			return domain.PermissionAPIKeyManage, true
		}
		if len(parts) == 2 && (method == http.MethodGet || method == http.MethodPost) {
			if method == http.MethodGet {
				return domain.PermissionAPIKeyRead, true
			}
			return domain.PermissionAPIKeyManage, true
		}
	case "audit":
		if len(parts) == 2 && method == http.MethodGet {
			return domain.PermissionAuditRead, true
		}
	}
	return "", false
}

func hasPermission(scopes []domain.Permission, wanted domain.Permission) bool {
	for _, scope := range scopes {
		if scope == wanted {
			return true
		}
	}
	return false
}

func (a *App) membershipHasPermission(membership domain.Membership, wanted domain.Permission) bool {
	if a == nil || a.Roles == nil {
		return false
	}
	permissions := make(map[domain.Permission]struct{})
	for _, roleID := range membership.RoleIDs {
		role, err := a.Roles.GetByID(roleID)
		if err != nil || role.OrganizationID != membership.OrganizationID {
			continue
		}
		for _, permission := range role.Permissions {
			permissions[permission] = struct{}{}
		}
	}
	// Role is retained for compatibility with memberships created before role
	// assignments. Once RoleIDs exist they are authoritative; otherwise a
	// legacy role must not silently broaden a custom role's permissions.
	if len(membership.RoleIDs) == 0 {
		if rolePermissions, ok := domain.BuiltInRolePermissions(membership.Role); ok {
			for _, permission := range rolePermissions {
				permissions[permission] = struct{}{}
			}
		}
	}
	_, ok := permissions[wanted]
	return ok
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
		return
	}
	claims, ok := AccessTokenClaimsFromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid access token")
		return
	}
	user, err := a.Users.GetByID(domain.UserID(claims.Sub))
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid access token")
		return
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: user.PublicProfile()})
}

type organizationCreateRequest struct {
	Name string `json:"name"`
}

// handleOrganizations serves the collection endpoint. Organization identity
// is always generated by the service; the request cannot supply a tenant ID.
func (a *App) handleOrganizations(w http.ResponseWriter, r *http.Request) {
	claims, ok := AccessTokenClaimsFromContext(r.Context())
	if !ok || a.Users == nil {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid access token")
		return
	}
	userID := domain.UserID(claims.Sub)
	if _, err := a.Users.GetByID(userID); err != nil {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid access token")
		return
	}

	switch r.Method {
	case http.MethodPost:
		a.createOrganization(w, r, userID)
	case http.MethodGet:
		a.listOrganizations(w, userID)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
	}
}

func (a *App) createOrganization(w http.ResponseWriter, r *http.Request, ownerID domain.UserID) {
	if a.Organizations == nil || a.Memberships == nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create organization")
		return
	}
	var input organizationCreateRequest
	if err := decodeJSONBody(w, r, &input); err != nil {
		if errors.Is(err, errRequestBodyTooLarge) {
			writeAPIError(w, http.StatusRequestEntityTooLarge, string(domain.ErrorInvalid), "request body too large")
			return
		}
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "invalid request body")
		return
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "organization name is required")
		return
	}
	organizationID, err := domain.NewOrganizationID()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create organization")
		return
	}
	now := a.Clock.Now()
	organization := domain.Organization{ID: organizationID, Name: name, OwnerID: ownerID, CreatedAt: now, UpdatedAt: now}
	if err := a.Organizations.Create(organization); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create organization")
		return
	}
	membershipID, err := domain.NewMembershipID()
	if err != nil {
		_ = a.Organizations.Delete(organizationID)
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create organization")
		return
	}
	membership := domain.Membership{
		ID: membershipID, OrganizationID: organizationID, UserID: ownerID,
		Role: "owner", CreatedAt: now, UpdatedAt: now,
	}
	if err := a.Memberships.Create(membership); err != nil {
		_ = a.Organizations.Delete(organizationID)
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create organization")
		return
	}
	if a.Roles != nil {
		for _, roleName := range []string{domain.RoleOwner, domain.RoleAdmin, domain.RoleViewer} {
			roleID, roleErr := domain.NewRoleID()
			role, createErr := domain.Role{}, roleErr
			if createErr == nil {
				role, createErr = domain.NewBuiltInRole(roleID, organizationID, roleName, now)
			}
			if createErr != nil || a.Roles.Create(role) != nil {
				_ = a.Memberships.Delete(membershipID)
				_ = a.Organizations.Delete(organizationID)
				writeAPIError(w, http.StatusInternalServerError, "internal", "could not create organization")
				return
			}
			if roleName == domain.RoleOwner {
				membership.RoleIDs = append(membership.RoleIDs, roleID)
				if updateErr := a.Memberships.Update(domain.Membership{ID: membershipID, OrganizationID: organizationID, UserID: ownerID, Role: domain.RoleOwner, RoleIDs: membership.RoleIDs, CreatedAt: now, UpdatedAt: now}); updateErr != nil {
					_ = a.Memberships.Delete(membershipID)
					_ = a.Organizations.Delete(organizationID)
					writeAPIError(w, http.StatusInternalServerError, "internal", "could not create organization")
					return
				}
			}
		}
	}
	a.recordAudit(organizationID, ownerID, "organization.created", string(organizationID), map[string]string{"name": name})
	writeJSON(w, http.StatusCreated, dataEnvelope{Data: organization})
}

func (a *App) listOrganizations(w http.ResponseWriter, userID domain.UserID) {
	if a.Organizations == nil || a.Memberships == nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not list organizations")
		return
	}
	memberships := a.Memberships.ListByUser(userID)
	organizations := make([]domain.Organization, 0, len(memberships))
	for _, membership := range memberships {
		organization, err := a.Organizations.GetByID(membership.OrganizationID)
		if err == nil {
			organizations = append(organizations, organization)
		}
	}
	sort.Slice(organizations, func(i, j int) bool { return organizations[i].ID < organizations[j].ID })
	writeJSON(w, http.StatusOK, dataEnvelope{Data: organizations})
}

type organizationInviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role,omitempty"`
}

type roleCreateRequest struct {
	Name        string              `json:"name"`
	Permissions []domain.Permission `json:"permissions"`
}

type apiKeyCreateRequest struct {
	Name   string              `json:"name"`
	Scopes []domain.Permission `json:"scopes"`
}

type apiKeyCreateResponse struct {
	ID             domain.APIKeyID       `json:"id"`
	OrganizationID domain.OrganizationID `json:"organization_id"`
	UserID         domain.UserID         `json:"created_by"`
	Name           string                `json:"name"`
	Scopes         []domain.Permission   `json:"scopes"`
	CreatedAt      time.Time             `json:"created_at"`
	Key            string                `json:"key"`
}

type inviteResponse struct {
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
	Token     string    `json:"token"`
}

type memberResponse struct {
	ID             domain.MembershipID   `json:"id"`
	OrganizationID domain.OrganizationID `json:"organization_id"`
	UserID         domain.UserID         `json:"user_id"`
	Role           string                `json:"role"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
	User           domain.PublicProfile  `json:"user"`
}

type memberUpdateRequest struct {
	Role string `json:"role"`
}

type ownershipTransferRequest struct {
	UserID domain.UserID `json:"user_id"`
}

func (a *App) handleOrganizationSubresource(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/organizations/"), "/")
	if len(parts) < 2 || len(parts) > 5 || parts[0] == "" {
		writeAPIError(w, http.StatusNotFound, string(domain.ErrorNotFound), "organization resource not found")
		return
	}
	orgID := domain.OrganizationID(parts[0])
	claims, ok := AccessTokenClaimsFromContext(r.Context())
	if !ok || a.Users == nil || a.Memberships == nil {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid access token")
		return
	}
	userID := domain.UserID(claims.Sub)
	if _, err := a.Users.GetByID(userID); err != nil {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid access token")
		return
	}
	if _, err := a.Organizations.GetByID(orgID); err != nil {
		writeAPIError(w, http.StatusNotFound, string(domain.ErrorNotFound), "organization not found")
		return
	}
	if _, err := a.Memberships.GetByOrganizationAndUser(orgID, userID); err != nil {
		writeAPIError(w, http.StatusForbidden, string(domain.ErrorUnauthorized), "organization membership required")
		return
	}
	switch parts[1] {
	case "members":
		if len(parts) == 5 && parts[2] != "" && parts[3] == "roles" && parts[4] != "" {
			switch r.Method {
			case http.MethodPut:
				a.assignRole(w, orgID, userID, domain.UserID(parts[2]), domain.RoleID(parts[4]))
			case http.MethodDelete:
				a.revokeRole(w, orgID, userID, domain.UserID(parts[2]), domain.RoleID(parts[4]))
			default:
				writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
			}
			return
		}
		if len(parts) == 2 {
			if r.Method != http.MethodGet {
				writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
				return
			}
			a.listMembers(w, orgID)
			return
		}
		if parts[2] == "" {
			writeAPIError(w, http.StatusNotFound, string(domain.ErrorNotFound), "member not found")
			return
		}
		targetID := domain.UserID(parts[2])
		switch r.Method {
		case http.MethodPatch:
			a.updateMember(w, r, orgID, userID, targetID)
		case http.MethodDelete:
			a.deleteMember(w, r, orgID, userID, targetID)
		default:
			writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
		}
	case "invites":
		if len(parts) != 2 {
			writeAPIError(w, http.StatusNotFound, string(domain.ErrorNotFound), "organization resource not found")
			return
		}
		if r.Method != http.MethodPost {
			writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
			return
		}
		a.createInvite(w, r, orgID)
	case "ownership-transfer":
		if len(parts) != 2 || r.Method != http.MethodPost {
			writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
			return
		}
		a.transferOwnership(w, r, orgID, userID)
	case "roles":
		if len(parts) != 2 || r.Method != http.MethodPost {
			writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
			return
		}
		a.createRole(w, r, orgID, userID)
	case "api-keys":
		switch {
		case len(parts) == 2 && r.Method == http.MethodPost:
			a.createAPIKey(w, r, orgID, userID)
		case len(parts) == 2 && r.Method == http.MethodGet:
			a.listAPIKeys(w, orgID)
		case len(parts) == 3 && parts[2] != "" && r.Method == http.MethodDelete:
			a.revokeAPIKey(w, orgID, domain.APIKeyID(parts[2]))
		default:
			writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
		}
	case "audit":
		if len(parts) != 2 || r.Method != http.MethodGet {
			writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
			return
		}
		a.listAudit(w, r, orgID)
	default:
		writeAPIError(w, http.StatusNotFound, string(domain.ErrorNotFound), "organization resource not found")
	}
}

var validAPIKeyScopes = map[domain.Permission]struct{}{
	domain.PermissionOrgRead: {}, domain.PermissionOrgManage: {},
	domain.PermissionMemberRead: {}, domain.PermissionMemberManage: {},
	domain.PermissionRoleRead: {}, domain.PermissionRoleManage: {},
	domain.PermissionAPIKeyRead: {}, domain.PermissionAPIKeyManage: {},
	domain.PermissionAuditRead: {},
}

func validateAPIKeyScopes(scopes []domain.Permission) bool {
	if len(scopes) == 0 {
		return false
	}
	seen := make(map[domain.Permission]struct{}, len(scopes))
	for _, scope := range scopes {
		if _, ok := validAPIKeyScopes[scope]; !ok {
			return false
		}
		if _, duplicate := seen[scope]; duplicate {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}

func (a *App) createAPIKey(w http.ResponseWriter, r *http.Request, orgID domain.OrganizationID, userID domain.UserID) {
	if a.APIKeys == nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create api key")
		return
	}
	var input apiKeyCreateRequest
	if err := decodeJSONBody(w, r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "invalid request body")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || !validateAPIKeyScopes(input.Scopes) {
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "api key name and scopes are required")
		return
	}
	if a.Memberships == nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create api key")
		return
	}
	membership, err := a.Memberships.GetByOrganizationAndUser(orgID, userID)
	if err != nil {
		writeAPIError(w, http.StatusForbidden, string(domain.ErrorUnauthorized), "organization membership required")
		return
	}
	for _, scope := range input.Scopes {
		if !a.membershipHasPermission(membership, scope) {
			writeAPIError(w, http.StatusForbidden, string(domain.ErrorUnauthorized), "api key scope exceeds actor permissions")
			return
		}
	}
	id, err := domain.NewAPIKeyID()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create api key")
		return
	}
	raw, err := domain.NewAPIKey()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create api key")
		return
	}
	now := a.Clock.Now()
	key := domain.APIKey{ID: id, OrganizationID: orgID, UserID: userID, Name: input.Name, Scopes: append([]domain.Permission(nil), input.Scopes...), SecretHash: domain.HashAPIKey(raw), CreatedAt: now}
	if err := a.APIKeys.Create(key); err != nil {
		writeAPIError(w, http.StatusConflict, string(domain.ErrorConflict), "api key already exists")
		return
	}
	a.recordAudit(orgID, userID, "api_key.created", string(id), map[string]string{"name": input.Name})
	writeJSON(w, http.StatusCreated, dataEnvelope{Data: apiKeyCreateResponse{ID: key.ID, OrganizationID: key.OrganizationID, UserID: key.UserID, Name: key.Name, Scopes: key.Scopes, CreatedAt: key.CreatedAt, Key: raw}})
}

func (a *App) listAPIKeys(w http.ResponseWriter, orgID domain.OrganizationID) {
	if a.APIKeys == nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not list api keys")
		return
	}
	keys := a.APIKeys.ListByOrganization(orgID)
	sort.Slice(keys, func(i, j int) bool { return keys[i].ID < keys[j].ID })
	writeJSON(w, http.StatusOK, dataEnvelope{Data: keys})
}

func (a *App) revokeAPIKey(w http.ResponseWriter, orgID domain.OrganizationID, keyID domain.APIKeyID) {
	if a.APIKeys == nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not revoke api key")
		return
	}
	key, err := a.APIKeys.GetByID(keyID)
	if err != nil || key.OrganizationID != orgID {
		writeAPIError(w, http.StatusNotFound, string(domain.ErrorNotFound), "api key not found")
		return
	}
	if err := a.APIKeys.Revoke(keyID, a.Clock.Now()); err != nil {
		writeAPIError(w, http.StatusNotFound, string(domain.ErrorNotFound), "api key not found")
		return
	}
	key, _ = a.APIKeys.GetByID(keyID)
	a.recordAudit(orgID, key.UserID, "api_key.revoked", string(keyID), nil)
	writeJSON(w, http.StatusOK, dataEnvelope{Data: key})
}

func (a *App) recordAudit(orgID domain.OrganizationID, actorID domain.UserID, action, target string, details map[string]string) {
	if a == nil || a.Audit == nil {
		return
	}
	_, _ = a.Audit.Append(domain.AuditEvent{OrganizationID: orgID, ActorID: actorID, Action: action, Target: target, Details: redactDetails(details), CreatedAt: a.Clock.Now()})
}

func (a *App) recordUserAudits(userID domain.UserID, action, target string, details map[string]string) {
	if a == nil || a.Memberships == nil {
		return
	}
	for _, membership := range a.Memberships.ListByUser(userID) {
		a.recordAudit(membership.OrganizationID, userID, action, target, details)
	}
}

func redactDetails(details map[string]string) map[string]string {
	if len(details) == 0 {
		return nil
	}
	result := make(map[string]string, len(details))
	for key, value := range details {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "password") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "authorization") || lower == "key" || strings.Contains(lower, "refresh") {
			continue
		}
		result[key] = value
	}
	return result
}

type auditListResponse struct {
	Events     []domain.AuditEvent `json:"events"`
	NextCursor string              `json:"next_cursor,omitempty"`
}

func (a *App) listAudit(w http.ResponseWriter, r *http.Request, orgID domain.OrganizationID) {
	if a.Audit == nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not list audit events")
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "invalid audit limit")
			return
		}
		limit = parsed
	}
	if limit > 100 {
		limit = 100
	}
	events, next, err := a.Audit.ListByOrganization(orgID, r.URL.Query().Get("cursor"), limit)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "invalid audit cursor")
		return
	}
	for i := range events {
		events[i].Details = redactDetails(events[i].Details)
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: auditListResponse{Events: events, NextCursor: next}})
}

func (a *App) assignRole(w http.ResponseWriter, orgID domain.OrganizationID, actorID, targetID domain.UserID, roleID domain.RoleID) {
	a.organizationMu.Lock()
	defer a.organizationMu.Unlock()
	if a.Roles == nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not assign role")
		return
	}
	actor, err := a.Memberships.GetByOrganizationAndUser(orgID, actorID)
	if err != nil || !a.membershipHasPermission(actor, domain.PermissionRoleManage) {
		writeAPIError(w, http.StatusForbidden, string(domain.ErrorUnauthorized), "role management permission required")
		return
	}
	role, err := a.Roles.GetByID(roleID)
	if err != nil || role.OrganizationID != orgID {
		writeAPIError(w, http.StatusNotFound, string(domain.ErrorNotFound), "role not found")
		return
	}
	membership, err := a.Memberships.GetByOrganizationAndUser(orgID, targetID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, string(domain.ErrorNotFound), "member not found")
		return
	}
	for _, assigned := range membership.RoleIDs {
		if assigned == roleID {
			writeJSON(w, http.StatusOK, dataEnvelope{Data: membership})
			return
		}
	}
	membership.RoleIDs = append(membership.RoleIDs, roleID)
	if role.Name == domain.RoleOwner {
		membership.Role = domain.RoleOwner
	}
	membership.UpdatedAt = a.Clock.Now()
	if err := a.Memberships.Update(membership); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not assign role")
		return
	}
	a.recordAudit(orgID, actorID, "member.role_assigned", string(targetID), map[string]string{"role_id": string(roleID)})
	writeJSON(w, http.StatusOK, dataEnvelope{Data: membership})
}

func (a *App) revokeRole(w http.ResponseWriter, orgID domain.OrganizationID, actorID, targetID domain.UserID, roleID domain.RoleID) {
	a.organizationMu.Lock()
	defer a.organizationMu.Unlock()
	if a.Roles == nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not revoke role")
		return
	}
	actor, err := a.Memberships.GetByOrganizationAndUser(orgID, actorID)
	if err != nil || !a.membershipHasPermission(actor, domain.PermissionRoleManage) {
		writeAPIError(w, http.StatusForbidden, string(domain.ErrorUnauthorized), "role management permission required")
		return
	}
	role, err := a.Roles.GetByID(roleID)
	if err != nil || role.OrganizationID != orgID {
		writeAPIError(w, http.StatusNotFound, string(domain.ErrorNotFound), "role not found")
		return
	}
	membership, err := a.Memberships.GetByOrganizationAndUser(orgID, targetID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, string(domain.ErrorNotFound), "member not found")
		return
	}
	index := -1
	for i, assigned := range membership.RoleIDs {
		if assigned == roleID {
			index = i
			break
		}
	}
	if index < 0 {
		writeAPIError(w, http.StatusNotFound, string(domain.ErrorNotFound), "role assignment not found")
		return
	}
	if role.Name == domain.RoleOwner {
		ownerCount := 0
		for _, member := range a.Memberships.ListByOrganization(orgID) {
			// RoleIDs is the source of truth for assignments. Keep the legacy
			// Role field as a compatibility fallback for memberships created
			// before role assignments were introduced.
			if member.Role == domain.RoleOwner || hasRole(member.RoleIDs, roleID) {
				ownerCount++
			}
		}
		if ownerCount <= 1 {
			writeAPIError(w, http.StatusConflict, string(domain.ErrorConflict), "last owner cannot lose owner role")
			return
		}
	}
	membership.RoleIDs = append(membership.RoleIDs[:index], membership.RoleIDs[index+1:]...)
	if role.Name == domain.RoleOwner {
		membership.Role = domain.RoleViewer
	}
	membership.UpdatedAt = a.Clock.Now()
	if err := a.Memberships.Update(membership); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not revoke role")
		return
	}
	a.recordAudit(orgID, actorID, "member.role_revoked", string(targetID), map[string]string{"role_id": string(roleID)})
	w.WriteHeader(http.StatusNoContent)
}

func hasRole(roleIDs []domain.RoleID, roleID domain.RoleID) bool {
	for _, assigned := range roleIDs {
		if assigned == roleID {
			return true
		}
	}
	return false
}

func (a *App) createRole(w http.ResponseWriter, r *http.Request, orgID domain.OrganizationID, actorID domain.UserID) {
	if a.Roles == nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create role")
		return
	}
	actor, err := a.Memberships.GetByOrganizationAndUser(orgID, actorID)
	if err != nil || !a.membershipHasPermission(actor, domain.PermissionRoleManage) {
		writeAPIError(w, http.StatusForbidden, string(domain.ErrorUnauthorized), "role management permission required")
		return
	}
	var input roleCreateRequest
	if err := decodeJSONBody(w, r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "invalid request body")
		return
	}
	name := strings.ToLower(strings.TrimSpace(input.Name))
	if name == "" {
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "role name is required")
		return
	}
	if _, builtIn := domain.BuiltInRolePermissions(name); builtIn {
		writeAPIError(w, http.StatusConflict, string(domain.ErrorConflict), "built-in role cannot be customized")
		return
	}
	roleID, err := domain.NewRoleID()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create role")
		return
	}
	now := a.Clock.Now()
	role := domain.Role{ID: roleID, OrganizationID: orgID, Name: name, Permissions: input.Permissions, CreatedAt: now, UpdatedAt: now}
	if err := a.Roles.Create(role); err != nil {
		if domain.IsConflict(err) {
			writeAPIError(w, http.StatusConflict, string(domain.ErrorConflict), "role already exists")
			return
		}
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "invalid role")
		return
	}
	a.recordAudit(orgID, actorID, "role.created", string(roleID), map[string]string{"name": name})
	writeJSON(w, http.StatusCreated, dataEnvelope{Data: role})
}

func (a *App) createInvite(w http.ResponseWriter, r *http.Request, orgID domain.OrganizationID) {
	var input organizationInviteRequest
	if err := decodeJSONBody(w, r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "invalid request body")
		return
	}
	email, err := domain.ValidateEmail(input.Email)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "invalid email address")
		return
	}
	role := strings.TrimSpace(input.Role)
	if role == "" {
		role = "viewer"
	}
	raw, err := domain.NewInviteToken()
	if err != nil || a.Invites == nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create invite")
		return
	}
	expiresAt := a.Clock.Now().Add(a.Config.InviteTTL)
	if err := a.Invites.Create(domain.InviteToken{TokenHash: domain.HashInviteToken(raw), OrganizationID: orgID, Email: email, Role: role, ExpiresAt: expiresAt}); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create invite")
		return
	}
	claims, _ := AccessTokenClaimsFromContext(r.Context())
	a.recordAudit(orgID, domain.UserID(claims.Sub), "invite.created", "", map[string]string{"email": email, "role": role})
	writeJSON(w, http.StatusCreated, dataEnvelope{Data: inviteResponse{Email: email, Role: role, ExpiresAt: expiresAt, Token: raw}})
}

func (a *App) updateMember(w http.ResponseWriter, r *http.Request, orgID domain.OrganizationID, actorID, targetID domain.UserID) {
	var input memberUpdateRequest
	if err := decodeJSONBody(w, r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "invalid request body")
		return
	}
	role := strings.TrimSpace(input.Role)
	if role == "" || role == "owner" {
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "invalid member role")
		return
	}

	a.organizationMu.Lock()
	defer a.organizationMu.Unlock()
	actor, err := a.Memberships.GetByOrganizationAndUser(orgID, actorID)
	if err != nil || !a.membershipHasPermission(actor, domain.PermissionMemberManage) {
		writeAPIError(w, http.StatusForbidden, string(domain.ErrorUnauthorized), "member management permission required")
		return
	}
	target, err := a.Memberships.GetByOrganizationAndUser(orgID, targetID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, string(domain.ErrorNotFound), "member not found")
		return
	}
	if target.Role == "owner" {
		writeAPIError(w, http.StatusConflict, string(domain.ErrorConflict), "owner role requires ownership transfer")
		return
	}
	target.Role = role
	target.UpdatedAt = a.Clock.Now()
	if err := a.Memberships.Update(target); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not update member")
		return
	}
	a.recordAudit(orgID, actorID, "member.updated", string(targetID), map[string]string{"role": role})
	writeJSON(w, http.StatusOK, dataEnvelope{Data: target})
}

func (a *App) deleteMember(w http.ResponseWriter, r *http.Request, orgID domain.OrganizationID, actorID, targetID domain.UserID) {
	a.organizationMu.Lock()
	defer a.organizationMu.Unlock()
	actor, err := a.Memberships.GetByOrganizationAndUser(orgID, actorID)
	if err != nil || !a.membershipHasPermission(actor, domain.PermissionMemberManage) {
		writeAPIError(w, http.StatusForbidden, string(domain.ErrorUnauthorized), "member management permission required")
		return
	}
	target, err := a.Memberships.GetByOrganizationAndUser(orgID, targetID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, string(domain.ErrorNotFound), "member not found")
		return
	}
	if target.Role == "owner" {
		writeAPIError(w, http.StatusConflict, string(domain.ErrorConflict), "owner must transfer ownership before removal")
		return
	}
	if err := a.Memberships.Delete(target.ID); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not delete member")
		return
	}
	a.recordAudit(orgID, actorID, "member.deleted", string(targetID), nil)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) transferOwnership(w http.ResponseWriter, r *http.Request, orgID domain.OrganizationID, actorID domain.UserID) {
	var input ownershipTransferRequest
	if err := decodeJSONBody(w, r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "invalid request body")
		return
	}
	if input.UserID == "" {
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "user_id is required")
		return
	}

	// A single application lock covers both repository writes. This prevents a
	// concurrent member mutation from observing an organization with no owner
	// or two owners during the transfer.
	a.organizationMu.Lock()
	defer a.organizationMu.Unlock()
	organization, err := a.Organizations.GetByID(orgID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, string(domain.ErrorNotFound), "organization not found")
		return
	}
	actor, err := a.Memberships.GetByOrganizationAndUser(orgID, actorID)
	if err != nil || actor.Role != "owner" || organization.OwnerID != actorID {
		writeAPIError(w, http.StatusForbidden, string(domain.ErrorUnauthorized), "owner permission required")
		return
	}
	target, err := a.Memberships.GetByOrganizationAndUser(orgID, input.UserID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, string(domain.ErrorNotFound), "member not found")
		return
	}
	if target.UserID == actorID {
		writeAPIError(w, http.StatusConflict, string(domain.ErrorConflict), "ownership already belongs to user")
		return
	}
	now := a.Clock.Now()
	previousOwner := actor
	previousOwner.Role = "admin"
	if a.Roles != nil {
		if ownerRole, roleErr := a.Roles.GetByOrganizationAndName(orgID, domain.RoleOwner); roleErr == nil {
			previousOwner.RoleIDs = removeRoleID(previousOwner.RoleIDs, ownerRole.ID)
		}
		if adminRole, roleErr := a.Roles.GetByOrganizationAndName(orgID, domain.RoleAdmin); roleErr == nil {
			previousOwner.RoleIDs = appendRoleID(previousOwner.RoleIDs, adminRole.ID)
		}
		if ownerRole, roleErr := a.Roles.GetByOrganizationAndName(orgID, domain.RoleOwner); roleErr == nil {
			target.RoleIDs = appendRoleID(target.RoleIDs, ownerRole.ID)
		}
	}
	previousOwner.UpdatedAt = now
	target.Role = "owner"
	target.UpdatedAt = now
	organization.OwnerID = target.UserID
	organization.UpdatedAt = now
	if err := a.Memberships.Update(previousOwner); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not transfer ownership")
		return
	}
	if err := a.Memberships.Update(target); err != nil {
		// Roll back the first write while still holding the mutation lock.
		_ = a.Memberships.Update(actor)
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not transfer ownership")
		return
	}
	if err := a.Organizations.Update(organization); err != nil {
		_ = a.Memberships.Update(actor)
		_ = a.Memberships.Update(target)
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not transfer ownership")
		return
	}
	a.recordAudit(orgID, actorID, "organization.ownership_transferred", string(target.UserID), map[string]string{"previous_owner_id": string(actorID)})
	writeJSON(w, http.StatusOK, dataEnvelope{Data: organization})
}

func appendRoleID(roleIDs []domain.RoleID, roleID domain.RoleID) []domain.RoleID {
	if hasRole(roleIDs, roleID) {
		return roleIDs
	}
	return append(roleIDs, roleID)
}

func removeRoleID(roleIDs []domain.RoleID, roleID domain.RoleID) []domain.RoleID {
	result := roleIDs[:0]
	for _, assigned := range roleIDs {
		if assigned != roleID {
			result = append(result, assigned)
		}
	}
	return result
}

func (a *App) listMembers(w http.ResponseWriter, orgID domain.OrganizationID) {
	result := make([]memberResponse, 0)
	for _, membership := range a.Memberships.ListByOrganization(orgID) {
		user, err := a.Users.GetByID(membership.UserID)
		if err != nil {
			continue
		}
		result = append(result, memberResponse{ID: membership.ID, OrganizationID: membership.OrganizationID, UserID: membership.UserID, Role: membership.Role, CreatedAt: membership.CreatedAt, UpdatedAt: membership.UpdatedAt, User: user.PublicProfile()})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UserID < result[j].UserID })
	writeJSON(w, http.StatusOK, dataEnvelope{Data: result})
}

func (a *App) handleInviteAccept(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
		return
	}
	raw := strings.TrimPrefix(r.URL.Path, "/v1/invites/")
	if raw == "" || strings.HasSuffix(raw, "/accept") == false || strings.Count(raw, "/") != 1 {
		writeAPIError(w, http.StatusNotFound, string(domain.ErrorNotFound), "invite not found")
		return
	}
	raw = strings.TrimSuffix(raw, "/accept")
	claims, ok := AccessTokenClaimsFromContext(r.Context())
	if !ok || a.Invites == nil || a.Users == nil || a.Memberships == nil {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid access token")
		return
	}
	user, err := a.Users.GetByID(domain.UserID(claims.Sub))
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid access token")
		return
	}
	invite, err := a.Invites.Get(raw)
	if err != nil || invite.Email != user.Email {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid invite")
		return
	}
	invite, err = a.Invites.Consume(raw, a.Clock.Now())
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid invite")
		return
	}
	membershipID, err := domain.NewMembershipID()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not accept invite")
		return
	}
	membership := domain.Membership{ID: membershipID, OrganizationID: invite.OrganizationID, UserID: user.ID, Role: invite.Role, CreatedAt: a.Clock.Now(), UpdatedAt: a.Clock.Now()}
	if err := a.Memberships.Create(membership); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			if existing, lookupErr := a.Memberships.GetByOrganizationAndUser(invite.OrganizationID, user.ID); lookupErr == nil {
				writeJSON(w, http.StatusOK, dataEnvelope{Data: existing})
				return
			}
		}
		writeAPIError(w, http.StatusConflict, string(domain.ErrorConflict), "membership already exists")
		return
	}
	a.recordAudit(invite.OrganizationID, user.ID, "invite.accepted", string(membership.ID), map[string]string{"role": membership.Role})
	writeJSON(w, http.StatusCreated, dataEnvelope{Data: membership})
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type passwordChangeRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type passwordResetRequest struct {
	Email string `json:"email"`
}

type passwordResetConfirmRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

type passwordResetResponse struct {
	Message string `json:"message"`
	Token   string `json:"token,omitempty"`
}

type emailVerificationConfirmRequest struct {
	Token string `json:"token"`
}

type emailVerificationResponse struct {
	Message string `json:"message"`
	Token   string `json:"token,omitempty"`
}

type sessionResponse struct {
	ID        domain.SessionID `json:"id"`
	UserID    domain.UserID    `json:"user_id"`
	ExpiresAt time.Time        `json:"expires_at"`
	Family    string           `json:"family"`
	Revoked   bool             `json:"revoked"`
}

type dataEnvelope struct {
	Data any `json:"data"`
}

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func (a *App) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "invalid", "method not allowed")
		return
	}

	var input registerRequest
	if err := decodeJSONBody(w, r, &input); err != nil {
		if errors.Is(err, errRequestBodyTooLarge) {
			writeAPIError(w, http.StatusRequestEntityTooLarge, string(domain.ErrorInvalid), "request body too large")
			return
		}
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "invalid request body")
		return
	}
	if input.Email == "" || input.Password == "" {
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "email and password are required")
		return
	}

	email, err := domain.ValidateEmail(input.Email)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "invalid email address")
		return
	}
	passwordMaterial, err := domain.HashPassword(input.Password)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "invalid password")
		return
	}
	userID, err := domain.NewUserID()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create user")
		return
	}
	now := a.Clock.Now()
	user := domain.User{
		ID:               userID,
		Email:            email,
		CreatedAt:        now,
		UpdatedAt:        now,
		PasswordMaterial: passwordMaterial,
	}
	if err := a.Users.Create(user); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			writeAPIError(w, http.StatusConflict, string(domain.ErrorConflict), "email already exists")
			return
		}
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "could not create user")
		return
	}

	writeJSON(w, http.StatusCreated, dataEnvelope{Data: user.PublicProfile()})
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
		return
	}

	var input loginRequest
	if err := decodeJSONBody(w, r, &input); err != nil {
		writeAuthError(w)
		return
	}

	// Keep all credential failures indistinguishable, including malformed or
	// missing email addresses. The tracker uses the same normalized identity
	// representation as the user repository.
	identity := domain.NormalizeEmail(input.Email)
	now := a.Clock.Now()
	if a.loginFailures.locked(identity, now) {
		writeAuthError(w)
		return
	}
	email, emailErr := domain.ValidateEmail(identity)
	user := domain.User{}
	var userErr error = domain.ErrNotFound
	if emailErr == nil {
		user, userErr = a.Users.GetByEmail(email)
	}
	material := a.dummyPassword
	if userErr == nil {
		material = user.PasswordMaterial
	}
	passwordOK := domain.VerifyPassword(input.Password, material)
	if emailErr != nil || userErr != nil || !passwordOK {
		a.loginFailures.failed(identity, now, a.Config.LoginFailureThreshold, a.Config.LoginLockDuration)
		writeAuthError(w)
		return
	}
	a.loginFailures.succeeded(identity)

	if a.AccessTokens == nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create access token")
		return
	}
	accessToken, err := a.AccessTokens.Create(user)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create access token")
		return
	}
	refreshToken, err := domain.NewRefreshToken()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create refresh token")
		return
	}
	sessionID, err := domain.NewSessionID()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create session")
		return
	}
	family, err := domain.NewID()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create session")
		return
	}
	if a.Sessions == nil || a.Sessions.Create(domain.Session{
		ID: sessionID, UserID: user.ID, RefreshTokenHash: domain.HashRefreshToken(refreshToken),
		ExpiresAt: a.Clock.Now().Add(a.Config.RefreshTTL), Family: string(family),
	}) != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create session")
		return
	}
	a.recordUserAudits(user.ID, "session.created", string(sessionID), nil)
	writeJSON(w, http.StatusOK, dataEnvelope{Data: loginResponse{AccessToken: accessToken, RefreshToken: refreshToken}})
}

func (a *App) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
		return
	}
	var input refreshRequest
	if err := decodeJSONBody(w, r, &input); err != nil || input.RefreshToken == "" {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid refresh token")
		return
	}
	if a.Sessions == nil || a.AccessTokens == nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not refresh token")
		return
	}
	session, err := a.Sessions.GetByRefreshToken(input.RefreshToken)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid refresh token")
		return
	}
	if session.Revoked {
		// Reuse of a rotated predecessor is a family compromise. Revoke the
		// whole family, including the replacement token that may still be
		// presented by the legitimate client.
		_ = a.Sessions.RevokeFamily(session.Family)
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid refresh token")
		return
	}
	if !a.Clock.Now().Before(session.ExpiresAt) {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid refresh token")
		return
	}
	user, err := a.Users.GetByID(session.UserID)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid refresh token")
		return
	}
	accessToken, err := a.AccessTokens.Create(user)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create access token")
		return
	}
	newRefreshToken, err := domain.NewRefreshToken()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create refresh token")
		return
	}
	newSessionID, err := domain.NewSessionID()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create session")
		return
	}
	replacement := domain.Session{
		ID: newSessionID, UserID: session.UserID, RefreshTokenHash: domain.HashRefreshToken(newRefreshToken),
		ExpiresAt: a.Clock.Now().Add(a.Config.RefreshTTL), Family: session.Family,
	}
	if err := a.Sessions.Rotate(input.RefreshToken, replacement); err != nil {
		if errors.Is(err, domain.ErrUnauthorized) || errors.Is(err, domain.ErrNotFound) {
			writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid refresh token")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not rotate refresh token")
		return
	}
	a.recordUserAudits(session.UserID, "session.rotated", string(newSessionID), nil)
	writeJSON(w, http.StatusOK, dataEnvelope{Data: loginResponse{AccessToken: accessToken, RefreshToken: newRefreshToken}})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
		return
	}
	var input refreshRequest
	if err := decodeJSONBody(w, r, &input); err != nil || input.RefreshToken == "" {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid refresh token")
		return
	}
	claims, ok := AccessTokenClaimsFromContext(r.Context())
	if !ok || a.Sessions == nil {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid access token")
		return
	}
	session, err := a.Sessions.GetByRefreshToken(input.RefreshToken)
	if err != nil || session.UserID != domain.UserID(claims.Sub) {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid refresh token")
		return
	}
	if err := a.Sessions.Revoke(session.ID); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not revoke session")
		return
	}
	a.recordUserAudits(domain.UserID(claims.Sub), "session.revoked", string(session.ID), nil)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleLogoutAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
		return
	}
	claims, ok := AccessTokenClaimsFromContext(r.Context())
	if !ok || a.Sessions == nil {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid access token")
		return
	}
	a.Sessions.RevokeAllForUser(domain.UserID(claims.Sub))
	a.recordUserAudits(domain.UserID(claims.Sub), "sessions.revoked_all", "", nil)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
		return
	}
	claims, ok := AccessTokenClaimsFromContext(r.Context())
	if !ok || a.Sessions == nil {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid access token")
		return
	}
	sessions := a.Sessions.ListByUser(domain.UserID(claims.Sub))
	result := make([]sessionResponse, 0, len(sessions))
	for _, session := range sessions {
		result = append(result, sessionResponse{ID: session.ID, UserID: session.UserID, ExpiresAt: session.ExpiresAt, Family: session.Family, Revoked: session.Revoked})
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: result})
}

func (a *App) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
		return
	}
	claims, ok := AccessTokenClaimsFromContext(r.Context())
	if !ok || a.Sessions == nil {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid access token")
		return
	}
	rawID := strings.TrimPrefix(r.URL.Path, "/v1/sessions/")
	if rawID == "" || strings.Contains(rawID, "/") {
		writeAPIError(w, http.StatusNotFound, string(domain.ErrorNotFound), "session not found")
		return
	}
	session, err := a.Sessions.GetByID(domain.SessionID(rawID))
	if err != nil || session.UserID != domain.UserID(claims.Sub) {
		writeAPIError(w, http.StatusNotFound, string(domain.ErrorNotFound), "session not found")
		return
	}
	if err := a.Sessions.Revoke(session.ID); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not revoke session")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handlePasswordChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
		return
	}
	claims, ok := AccessTokenClaimsFromContext(r.Context())
	if !ok || a.Users == nil || a.Sessions == nil {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid access token")
		return
	}

	var input passwordChangeRequest
	if err := decodeJSONBody(w, r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "invalid request body")
		return
	}
	user, err := a.Users.GetByID(domain.UserID(claims.Sub))
	if err != nil || !domain.VerifyPassword(input.CurrentPassword, user.PasswordMaterial) {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid current password")
		return
	}
	newMaterial, err := domain.HashPassword(input.NewPassword)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "invalid new password")
		return
	}
	user.PasswordMaterial = newMaterial
	user.UpdatedAt = a.Clock.Now()
	if err := a.Users.Update(user); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not change password")
		return
	}
	a.Sessions.RevokeAllForUser(user.ID)
	a.recordUserAudits(user.ID, "password.changed", string(user.ID), nil)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handlePasswordResetRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
		return
	}
	var input passwordResetRequest
	if err := decodeJSONBody(w, r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "invalid request body")
		return
	}
	// A token is returned in the same response shape for every request. Only
	// tokens for existing accounts are persisted; delivery would normally be
	// handled out of band in production.
	rawToken, err := domain.NewPasswordResetToken()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create password reset token")
		return
	}
	if email, emailErr := domain.ValidateEmail(input.Email); emailErr == nil {
		if user, userErr := a.Users.GetByEmail(email); userErr == nil && a.PasswordResets != nil {
			_ = a.PasswordResets.Create(domain.PasswordResetToken{
				TokenHash: domain.HashPasswordResetToken(rawToken), UserID: user.ID,
				ExpiresAt: a.Clock.Now().Add(a.Config.PasswordResetTTL),
			})
		}
	}
	writeJSON(w, http.StatusAccepted, dataEnvelope{Data: passwordResetResponse{
		Message: "if the account exists, password reset instructions have been sent",
		Token:   rawToken,
	}})
}

func (a *App) handlePasswordResetConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
		return
	}
	var input passwordResetConfirmRequest
	if err := decodeJSONBody(w, r, &input); err != nil || input.Token == "" {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid password reset token")
		return
	}
	if err := domain.ValidatePassword(input.NewPassword); err != nil {
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "invalid new password")
		return
	}
	if a.PasswordResets == nil || a.Users == nil || a.Sessions == nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not reset password")
		return
	}
	token, err := a.PasswordResets.Consume(input.Token, a.Clock.Now())
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid password reset token")
		return
	}
	user, err := a.Users.GetByID(token.UserID)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid password reset token")
		return
	}
	material, err := domain.HashPassword(input.NewPassword)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "invalid new password")
		return
	}
	user.PasswordMaterial = material
	user.UpdatedAt = a.Clock.Now()
	if err := a.Users.Update(user); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not reset password")
		return
	}
	a.Sessions.RevokeAllForUser(user.ID)
	a.recordUserAudits(user.ID, "password.reset", string(user.ID), nil)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleEmailVerificationRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
		return
	}
	var input passwordResetRequest
	if err := decodeJSONBody(w, r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, string(domain.ErrorInvalid), "invalid request body")
		return
	}
	rawToken, err := domain.NewEmailVerificationToken()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not create email verification token")
		return
	}
	if email, emailErr := domain.ValidateEmail(input.Email); emailErr == nil {
		if user, userErr := a.Users.GetByEmail(email); userErr == nil && !user.EmailVerified && a.EmailVerifications != nil {
			_ = a.EmailVerifications.Create(domain.EmailVerificationToken{
				TokenHash: domain.HashEmailVerificationToken(rawToken), UserID: user.ID,
				ExpiresAt: a.Clock.Now().Add(a.Config.EmailVerificationTTL),
			})
		}
	}
	writeJSON(w, http.StatusAccepted, dataEnvelope{Data: emailVerificationResponse{
		Message: "if the account exists, email verification instructions have been sent", Token: rawToken,
	}})
}

func (a *App) handleEmailVerificationConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, string(domain.ErrorInvalid), "method not allowed")
		return
	}
	var input emailVerificationConfirmRequest
	if err := decodeJSONBody(w, r, &input); err != nil || input.Token == "" {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid email verification token")
		return
	}
	if a.EmailVerifications == nil || a.Users == nil {
		writeAPIError(w, http.StatusInternalServerError, "internal", "could not verify email")
		return
	}
	token, err := a.EmailVerifications.Consume(input.Token, a.Clock.Now())
	if err != nil {
		// Once an account is verified, replaying its old confirmation is a
		// successful no-op. The repository still treats the bearer as one-time.
		if saved, lookupErr := a.EmailVerifications.Get(input.Token); lookupErr == nil {
			if user, userErr := a.Users.GetByID(saved.UserID); userErr == nil && user.EmailVerified {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid email verification token")
		return
	}
	user, err := a.Users.GetByID(token.UserID)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid email verification token")
		return
	}
	if !user.EmailVerified {
		user.EmailVerified = true
		user.UpdatedAt = a.Clock.Now()
		if err := a.Users.Update(user); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "internal", "could not verify email")
			return
		}
	}
	a.recordUserAudits(user.ID, "email.verified", string(user.ID), nil)
	w.WriteHeader(http.StatusNoContent)
}

func writeAuthError(w http.ResponseWriter) {
	writeAPIError(w, http.StatusUnauthorized, string(domain.ErrorUnauthorized), "invalid credentials")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorEnvelope{Error: apiError{Code: code, Message: message, RequestID: requestIDFromWriter(w)}})
}

// decodeJSONBody applies the transport-wide JSON request rules: bounded input,
// known fields only, and exactly one JSON value in the body.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		if errors.Is(err, http.ErrBodyReadAfterClose) {
			return errRequestBodyTooLarge
		}
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return errRequestBodyTooLarge
		}
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if errors.Is(err, http.ErrBodyReadAfterClose) {
			return errRequestBodyTooLarge
		}
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return errRequestBodyTooLarge
		}
		return errors.New("request body must contain one JSON value")
	}
	return nil
}
