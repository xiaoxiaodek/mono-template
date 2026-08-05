package identity

import "time"

type PasswordHasher interface {
	Hash(string) (string, error)
	Compare(string, string) bool
}

type IDGenerator func(string) (string, error)

type Principal struct {
	UserID      string
	Email       string
	Roles       []string
	Permissions []string
	// JTI is an optional unique token identifier populated when a new
	// refresh token is issued so every JWT is cryptographically distinct.
	JTI string
}

type TokenClaims struct {
	UserID    string
	ExpiresAt time.Time
}

type TokenManager interface {
	SignAccessToken(Principal) (string, error)
	SignRefreshToken(Principal) (string, error)
	VerifyAccessToken(string) (TokenClaims, error)
	VerifyRefreshToken(string) (TokenClaims, error)
}
