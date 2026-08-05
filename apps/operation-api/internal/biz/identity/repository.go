package identity

import (
	"context"
	"time"
)

type UserRepository interface {
	Create(context.Context, User) error
	Delete(context.Context, string) error
	FindByEmail(context.Context, string) (User, error)
	FindByID(context.Context, string) (User, error)
}

type RefreshTokenStore interface {
	Save(context.Context, string, string, time.Time) error
	Rotate(context.Context, string, string, string, time.Time) (bool, error)
}

type RegistrationUnitOfWork interface {
	CreateUserWithRefreshToken(context.Context, User, string, time.Time) error
}
