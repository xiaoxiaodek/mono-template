package identity

import (
	"net/mail"
	"strings"
	"time"
)

type User struct {
	ID           string
	Email        string
	PasswordHash string
	Roles        []string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NewUser(id string, email string, passwordHash string, roles []string) (User, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(normalizedEmail)
	if err != nil || address.Address != normalizedEmail {
		return User{}, ErrInvalidEmail
	}
	if passwordHash == "" {
		return User{}, ErrPasswordHashRequired
	}
	if len(roles) == 0 {
		roles = []string{"user"}
	}
	now := time.Now().UTC()
	return User{ID: id, Email: normalizedEmail, PasswordHash: passwordHash, Roles: append([]string(nil), roles...), CreatedAt: now, UpdatedAt: now}, nil
}
