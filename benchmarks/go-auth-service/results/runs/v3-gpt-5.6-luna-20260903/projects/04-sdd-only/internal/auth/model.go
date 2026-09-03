package auth

import "time"

type User struct {
	ID           string
	Email        string
	PasswordSalt []byte
	PasswordHash []byte
}

type Profile struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (u User) PublicProfile() Profile { return Profile{ID: u.ID, Email: u.Email} }

type Claims struct {
	Subject   string
	Email     string
	Issuer    string
	IssuedAt  time.Time
	ExpiresAt time.Time
	JWTID     string
}
