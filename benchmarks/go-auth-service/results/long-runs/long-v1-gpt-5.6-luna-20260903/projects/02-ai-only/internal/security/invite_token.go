package security

import "crypto/rand"

const InviteTokenSize = 32

func NewInviteToken() (string, error) {
	raw := make([]byte, InviteTokenSize)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return encodeOpaque(raw), nil
}

func HashInviteToken(token string) []byte { return HashRefreshToken(token) }
