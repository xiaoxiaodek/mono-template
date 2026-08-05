package identity

import "errors"

var (
	ErrInvalidEmail         = errors.New("invalid email")
	ErrPasswordHashRequired = errors.New("password hash required")
	ErrUserNotFound         = errors.New("user not found")
	ErrEmailTaken           = errors.New("email already taken")
	ErrInvalidCredentials   = errors.New("invalid credentials")
)
