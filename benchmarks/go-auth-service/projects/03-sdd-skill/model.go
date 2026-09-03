package main

import "time"

type User struct {
	ID           string
	Email        string
	Salt         []byte
	PasswordHash []byte
}

type PublicUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type Claims struct {
	Subject string
	Email   string
	Issuer  string
	Issued  time.Time
	Expires time.Time
	JTI     string
}
