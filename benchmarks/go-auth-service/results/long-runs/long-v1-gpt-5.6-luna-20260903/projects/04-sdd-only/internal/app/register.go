package app

import (
	"crypto/sha256"
	"errors"
	"net/http"

	"benchmark.local/iam/internal/domain"
	"benchmark.local/iam/internal/httpapi"
	"benchmark.local/iam/internal/repository"
	"benchmark.local/iam/internal/security"
)

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (a *App) register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "invalid", "method not allowed")
		return
	}
	var request registerRequest
	if err := httpapi.DecodeJSON(r, &request); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid request body")
		return
	}
	idempotencyKey := r.Header.Get("Idempotency-Key")
	storageKey := "register:" + r.RemoteAddr + ":" + idempotencyKey
	if idempotencyKey != "" {
		a.idempotencyMu.Lock()
		defer a.idempotencyMu.Unlock()
		fingerprint := registrationFingerprint(request.Email, request.Password)
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
		httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid registration data")
		return
	}
	passwordMaterial, err := security.HashPassword(request.Password)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid registration data")
		return
	}
	id, err := a.IDGenerator.NewID()
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	user := domain.User{
		ID:                id,
		Email:             email,
		PasswordMaterial:  passwordMaterial,
		EmailVerification: domain.VerificationPending,
		CreatedAt:         a.Clock.Now().UTC(),
	}
	if err := a.Users.Create(user); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			httpapi.WriteError(w, http.StatusConflict, "conflict", "user already exists")
			return
		}
		if errors.Is(err, domain.ErrInvalid) {
			httpapi.WriteError(w, http.StatusBadRequest, "invalid", "invalid registration data")
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}

	body, err := httpapi.MarshalData(user.PublicProfile())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
		return
	}
	if idempotencyKey != "" {
		_ = a.Idempotency.Put(storageKey, repository.IdempotencyRecord{Fingerprint: registrationFingerprint(request.Email, request.Password), Status: http.StatusCreated, Body: body})
	}
	httpapi.WriteBytes(w, http.StatusCreated, body)
}

func registrationFingerprint(email, password string) []byte {
	passwordHash := sha256.Sum256([]byte(password))
	hash := sha256.New()
	_, _ = hash.Write([]byte(email))
	_, _ = hash.Write(passwordHash[:])
	return hash.Sum(nil)
}

// Ensure the repository dependency remains expressed through its interface.
var _ repository.UserStore = (*repository.UserRepository)(nil)
