package main

import "time"

const minPasswordLength = 12

type User struct {
	ID           string
	Email        string
	PasswordSalt []byte
	PasswordHash []byte
}

type PublicUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (u User) Public() PublicUser { return PublicUser{ID: u.ID, Email: u.Email} }

type Claims struct {
	Subject string
	Email   string
	Issuer  string
	Issued  time.Time
	Expires time.Time
	JTI     string
}
