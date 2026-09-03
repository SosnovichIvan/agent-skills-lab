package domain

import (
	"crypto/rand"
	"encoding/hex"
)

// ID is an opaque identifier used by domain entities.
type ID string

func (id ID) String() string { return string(id) }

// IDGenerator allows domain services to receive ID generation as a
// dependency, while the production implementation uses crypto/rand.
type IDGenerator interface {
	NewID() (ID, error)
}

// CryptoIDGenerator generates 128-bit, URL-safe hexadecimal identifiers.
type CryptoIDGenerator struct{}

func (CryptoIDGenerator) NewID() (ID, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return ID(hex.EncodeToString(raw[:])), nil
}

// NewID generates one identifier using the production generator.
func NewID() (ID, error) { return CryptoIDGenerator{}.NewID() }
