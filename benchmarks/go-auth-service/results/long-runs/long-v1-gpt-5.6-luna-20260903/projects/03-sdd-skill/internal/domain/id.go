// Package domain contains the small, dependency-free building blocks shared
// by the IAM domain services.
package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// ID is an opaque, globally unique domain identifier. Its value is a UUID
// formatted string, generated from cryptographically secure random bytes.
type ID string

// NewID creates a random UUID (version 4, variant 1).
func NewID() (ID, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate domain id: %w", err)
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	var encoded [36]byte
	hex.Encode(encoded[0:8], bytes[0:4])
	encoded[8] = '-'
	hex.Encode(encoded[9:13], bytes[4:6])
	encoded[13] = '-'
	hex.Encode(encoded[14:18], bytes[6:8])
	encoded[18] = '-'
	hex.Encode(encoded[19:23], bytes[8:10])
	encoded[23] = '-'
	hex.Encode(encoded[24:36], bytes[10:16])
	return ID(encoded[:]), nil
}

// ParseID validates and returns a canonical UUID-form ID.
func ParseID(value string) (ID, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return "", fmt.Errorf("invalid domain id")
	}
	var bytes [16]byte
	if _, err := hex.Decode(bytes[:], []byte(strings.ReplaceAll(value, "-", ""))); err != nil {
		return "", fmt.Errorf("invalid domain id")
	}
	return ID(value), nil
}

// Typed IDs prevent accidentally using an identifier from another aggregate.
type UserID ID
type OrganizationID ID
type MembershipID ID
type SessionID ID
type RoleID ID
type APIKeyID ID

func NewUserID() (UserID, error)                 { id, err := NewID(); return UserID(id), err }
func NewOrganizationID() (OrganizationID, error) { id, err := NewID(); return OrganizationID(id), err }
func NewMembershipID() (MembershipID, error)     { id, err := NewID(); return MembershipID(id), err }
func NewSessionID() (SessionID, error)           { id, err := NewID(); return SessionID(id), err }
func NewRoleID() (RoleID, error)                 { id, err := NewID(); return RoleID(id), err }
func NewAPIKeyID() (APIKeyID, error)             { id, err := NewID(); return APIKeyID(id), err }
