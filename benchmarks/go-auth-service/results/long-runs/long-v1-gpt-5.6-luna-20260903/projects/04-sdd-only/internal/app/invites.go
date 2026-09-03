package app

import (
	"crypto/sha256"
	"errors"
	"net/http"
	"strings"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/httpapi"
	"benchmark.local/iam/internal/repository"
	"benchmark.local/iam/internal/security"
)

type inviteCreateRequest struct {
	Email string `json:"email"`
}

func (a *App) createInvite(w http.ResponseWriter, r *http.Request, organizationID domain.ID) {
	claims, _ := httpapi.ClaimsFromContext(r.Context())
	var request inviteCreateRequest
	if err := httpapi.DecodeJSON(r, &request); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid request body")
		return
	}
	idempotencyKey := r.Header.Get("Idempotency-Key")
	fingerprint := inviteFingerprint(request.Email)
	storageKey := "invite:" + string(claims.Subject) + ":" + string(organizationID) + ":" + idempotencyKey
	if idempotencyKey != "" {
		a.idempotencyMu.Lock()
		defer a.idempotencyMu.Unlock()
		if record, exists := a.Idempotency.Get(storageKey); exists {
			if string(record.Fingerprint) != string(fingerprint) {
				httpapi.WriteError(w, http.StatusConflict, "conflict", "idempotency key reused with different request")
				return
			}
			httpapi.WriteBytes(w, record.Status, record.Body)
			return
		}
	}
	email, err := security.NormalizeEmail(request.Email)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid invite email")
		return
	}
	tokenValue, err := security.NewInviteToken()
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	tokenID, err := a.IDGenerator.NewID()
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if err := a.Invites.Create(domain.Invite{
		ID: tokenID, OrganizationID: organizationID, InvitedEmail: email,
		TokenHash: security.HashOpaqueToken(tokenValue),
		ExpiresAt: a.Clock.Now().UTC().Add(a.Config.InviteTTL),
	}); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if err := a.appendAudit(organizationID, domain.ID(claims.Subject), "invite.created", map[string]string{"invited_email": email}); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	body, err := httpapi.MarshalData(map[string]string{"invite_token": tokenValue})
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if idempotencyKey != "" {
		_ = a.Idempotency.Put("invite:"+string(organizationID)+":"+idempotencyKey, repository.IdempotencyRecord{Fingerprint: fingerprint, Status: http.StatusCreated, Body: body})
	}
	httpapi.WriteBytes(w, http.StatusCreated, body)
}

func inviteFingerprint(email string) []byte {
	hash := sha256.Sum256([]byte(email))
	return append([]byte(nil), hash[:]...)
}

func (a *App) acceptInvite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "invalid", "method not allowed")
		return
	}
	claims, ok := httpapi.ClaimsFromContext(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}
	tokenValue := strings.TrimPrefix(r.URL.Path, "/v1/invites/")
	if tokenValue == "" || strings.Contains(tokenValue, "/") {
		writeUnauthorized(w)
		return
	}
	tokenHash := security.HashOpaqueToken(tokenValue)
	invite, err := a.Invites.FindByTokenHash(tokenHash)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	user, err := a.Users.Get(domain.ID(claims.Subject))
	if err != nil || user.Email != invite.InvitedEmail {
		writeUnauthorized(w)
		return
	}
	invite, err = a.Invites.Consume(tokenHash, a.Clock.Now().UTC())
	if err != nil {
		writeUnauthorized(w)
		return
	}
	err = a.Memberships.Create(domain.Membership{
		OrganizationID: invite.OrganizationID, UserID: user.ID, Role: "viewer", CreatedAt: a.Clock.Now().UTC(),
	})
	if err != nil && !errors.Is(err, domain.ErrConflict) {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if err := a.appendAudit(invite.OrganizationID, user.ID, "invite.accepted", nil); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	httpapi.WriteData(w, http.StatusOK, map[string]bool{"accepted": true})
}

func (a *App) listMembers(w http.ResponseWriter, organizationID domain.ID) {
	memberships := a.Memberships.ListByOrganization(organizationID)
	profiles := make([]domain.PublicProfile, 0, len(memberships))
	for _, membership := range memberships {
		user, err := a.Users.Get(membership.UserID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				continue
			}
			httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			return
		}
		profiles = append(profiles, user.PublicProfile())
	}
	httpapi.WriteData(w, http.StatusOK, profiles)
}
