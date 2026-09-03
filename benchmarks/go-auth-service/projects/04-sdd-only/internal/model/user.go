package model

import "time"

type User struct {
	ID           string
	Email        string
	PasswordSalt []byte
	PasswordHash []byte
	CreatedAt    time.Time
}

type Profile struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}
