// Package domain contains shared IAM domain primitives.
package domain

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
)

// ID is the common representation of an entity identifier. Values are UUID
// version 4 strings, generated with crypto/rand.
type ID string

type UserID ID
type OrganizationID ID
type SessionID ID
type InviteID ID
type RoleID ID
type APIKeyID ID
type TokenFamilyID ID

// NewID returns a new random UUID version 4 identifier.
func NewID() (ID, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	encoded := make([]byte, 36)
	hex.Encode(encoded[0:8], value[0:4])
	hex.Encode(encoded[9:13], value[4:6])
	hex.Encode(encoded[14:18], value[6:8])
	hex.Encode(encoded[19:23], value[8:10])
	hex.Encode(encoded[24:36], value[10:16])
	encoded[8], encoded[13], encoded[18], encoded[23] = '-', '-', '-', '-'
	return ID(encoded), nil
}

func newTypedID[T ~string](newValue func() (ID, error)) (T, error) {
	id, err := newValue()
	return T(id), err
}

func NewUserID() (UserID, error) { return newTypedID[UserID](NewID) }

func NewOrganizationID() (OrganizationID, error) { return newTypedID[OrganizationID](NewID) }

func NewSessionID() (SessionID, error) { return newTypedID[SessionID](NewID) }

func NewInviteID() (InviteID, error) { return newTypedID[InviteID](NewID) }

func NewRoleID() (RoleID, error) { return newTypedID[RoleID](NewID) }

func NewAPIKeyID() (APIKeyID, error) { return newTypedID[APIKeyID](NewID) }

func NewTokenFamilyID() (TokenFamilyID, error) { return newTypedID[TokenFamilyID](NewID) }

// ParseID validates the textual UUID representation before converting it to ID.
func ParseID(value string) (ID, error) {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return "", errors.New("invalid id")
	}
	var raw [16]byte
	if _, err := hex.Decode(raw[:4], []byte(value[0:8])); err != nil {
		return "", errors.New("invalid id")
	}
	if _, err := hex.Decode(raw[4:6], []byte(value[9:13])); err != nil {
		return "", errors.New("invalid id")
	}
	if _, err := hex.Decode(raw[6:8], []byte(value[14:18])); err != nil {
		return "", errors.New("invalid id")
	}
	if _, err := hex.Decode(raw[8:10], []byte(value[19:23])); err != nil {
		return "", errors.New("invalid id")
	}
	if _, err := hex.Decode(raw[10:16], []byte(value[24:36])); err != nil {
		return "", errors.New("invalid id")
	}
	return ID(value), nil
}
