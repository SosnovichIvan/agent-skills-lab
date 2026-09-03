// Package domain contains primitives shared by IAM domain services.
package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// ID is the opaque identifier used by domain entities.
type ID string

type (
	UserID         ID
	OrganizationID ID
	SessionID      ID
	RoleID         ID
	InviteID       ID
	APIKeyID       ID
)

// NewID returns a cryptographically random 128-bit identifier.
func NewID() (ID, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate domain id: %w", err)
	}
	return ID(hex.EncodeToString(raw[:])), nil
}

// MustNewID is intended for initialization paths where random generation
// failure cannot be recovered from.
func MustNewID() ID {
	id, err := NewID()
	if err != nil {
		panic(err)
	}
	return id
}

func NewUserID() (UserID, error) {
	id, err := NewID()
	return UserID(id), err
}

func NewOrganizationID() (OrganizationID, error) {
	id, err := NewID()
	return OrganizationID(id), err
}

func NewSessionID() (SessionID, error) {
	id, err := NewID()
	return SessionID(id), err
}

func NewRoleID() (RoleID, error) {
	id, err := NewID()
	return RoleID(id), err
}

func NewInviteID() (InviteID, error) {
	id, err := NewID()
	return InviteID(id), err
}

func NewAPIKeyID() (APIKeyID, error) {
	id, err := NewID()
	return APIKeyID(id), err
}
