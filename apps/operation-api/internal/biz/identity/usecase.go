package identity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const dummyPasswordHash = "$2y$10$H3Kn/yvALRON6tdTscY4OuoPkk9JYXERDOpPUDkRF1W20B8FXqOtG" // #nosec G101 -- not a credential

type Usecase struct {
	users        UserRepository
	tokens       RefreshTokenStore
	password     PasswordHasher
	tokenManager TokenManager
	generateID   IDGenerator
	registration RegistrationUnitOfWork
}

type RegisterInput struct{ Email, Password string }
type LoginInput struct{ Email, Password string }
type RefreshInput struct{ RefreshToken string }
type UserOutput struct {
	ID        string
	Email     string
	Roles     []string
	CreatedAt time.Time
	UpdatedAt time.Time
}
type AuthOutput struct {
	User                      UserOutput
	AccessToken, RefreshToken string
}

func NewUsecase(users UserRepository, tokens RefreshTokenStore, password PasswordHasher, tokenManager TokenManager, generateID IDGenerator, registration RegistrationUnitOfWork) *Usecase {
	return &Usecase{users: users, tokens: tokens, password: password, tokenManager: tokenManager, generateID: generateID, registration: registration}
}

func (u *Usecase) Register(ctx context.Context, input RegisterInput) (AuthOutput, error) {
	if err := ctx.Err(); err != nil {
		return AuthOutput{}, err
	}
	passwordHash, err := u.password.Hash(input.Password)
	if err != nil {
		return AuthOutput{}, err
	}
	userID, err := u.generateID("usr")
	if err != nil {
		return AuthOutput{}, err
	}
	user, err := NewUser(userID, input.Email, passwordHash, nil)
	if err != nil {
		return AuthOutput{}, err
	}
	candidate, err := u.generateTokenPair(user)
	if err != nil {
		return AuthOutput{}, err
	}
	if u.registration != nil {
		if err := u.registration.CreateUserWithRefreshToken(ctx, user, candidate.refreshHash, candidate.expiresAt); err != nil {
			return AuthOutput{}, err
		}
		return candidate.output, nil
	}
	if err := u.users.Create(ctx, user); err != nil {
		return AuthOutput{}, err
	}
	saveErr := u.tokens.Save(ctx, user.ID, candidate.refreshHash, candidate.expiresAt)
	if saveErr == nil {
		return candidate.output, nil
	}
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if deleteErr := u.users.Delete(rollbackCtx, user.ID); deleteErr != nil {
		return AuthOutput{}, errors.Join(saveErr, fmt.Errorf("rollback registered user: %w", deleteErr))
	}
	return AuthOutput{}, saveErr
}

func (u *Usecase) Login(ctx context.Context, input LoginInput) (AuthOutput, error) {
	if err := ctx.Err(); err != nil {
		return AuthOutput{}, err
	}
	user, err := u.users.FindByEmail(ctx, strings.ToLower(strings.TrimSpace(input.Email)))
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			u.password.Compare(dummyPasswordHash, input.Password)
			return AuthOutput{}, ErrInvalidCredentials
		}
		return AuthOutput{}, err
	}
	if !u.password.Compare(user.PasswordHash, input.Password) {
		return AuthOutput{}, ErrInvalidCredentials
	}
	return u.issueTokenPair(ctx, user)
}

func (u *Usecase) Refresh(ctx context.Context, input RefreshInput) (AuthOutput, error) {
	if err := ctx.Err(); err != nil {
		return AuthOutput{}, err
	}
	claims, err := u.tokenManager.VerifyRefreshToken(input.RefreshToken)
	if err != nil {
		return AuthOutput{}, ErrInvalidCredentials
	}
	user, err := u.users.FindByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return AuthOutput{}, ErrInvalidCredentials
		}
		return AuthOutput{}, err
	}
	candidate, err := u.generateTokenPair(user)
	if err != nil {
		return AuthOutput{}, err
	}
	rotated, err := u.tokens.Rotate(ctx, tokenHash(input.RefreshToken), user.ID, candidate.refreshHash, candidate.expiresAt)
	if err != nil {
		return AuthOutput{}, err
	}
	if !rotated {
		return AuthOutput{}, ErrInvalidCredentials
	}
	return candidate.output, nil
}

func (u *Usecase) Me(ctx context.Context, userID string) (UserOutput, error) {
	user, err := u.users.FindByID(ctx, userID)
	if err != nil {
		return UserOutput{}, err
	}
	return publicUser(user), nil
}

func (u *Usecase) issueTokenPair(ctx context.Context, user User) (AuthOutput, error) {
	candidate, err := u.generateTokenPair(user)
	if err != nil {
		return AuthOutput{}, err
	}
	if err := u.tokens.Save(ctx, user.ID, candidate.refreshHash, candidate.expiresAt); err != nil {
		return AuthOutput{}, err
	}
	return candidate.output, nil
}

type tokenPairCandidate struct {
	output      AuthOutput
	refreshHash string
	expiresAt   time.Time
}

func (u *Usecase) generateTokenPair(user User) (tokenPairCandidate, error) {
	principal := Principal{UserID: user.ID, Email: user.Email, Roles: append([]string(nil), user.Roles...)}
	accessToken, err := u.tokenManager.SignAccessToken(principal)
	if err != nil {
		return tokenPairCandidate{}, err
	}
	if _, err := u.tokenManager.VerifyAccessToken(accessToken); err != nil {
		return tokenPairCandidate{}, err
	}
	nonce, err := u.generateID("rft")
	if err != nil {
		return tokenPairCandidate{}, err
	}
	principal.Permissions = []string{nonce}
	refreshToken, err := u.tokenManager.SignRefreshToken(principal)
	if err != nil {
		return tokenPairCandidate{}, err
	}
	claims, err := u.tokenManager.VerifyRefreshToken(refreshToken)
	if err != nil {
		return tokenPairCandidate{}, err
	}
	if claims.ExpiresAt.IsZero() {
		return tokenPairCandidate{}, ErrInvalidCredentials
	}
	return tokenPairCandidate{output: AuthOutput{User: publicUser(user), AccessToken: accessToken, RefreshToken: refreshToken}, refreshHash: tokenHash(refreshToken), expiresAt: claims.ExpiresAt}, nil
}

func publicUser(user User) UserOutput {
	return UserOutput{ID: user.ID, Email: user.Email, Roles: append([]string(nil), user.Roles...), CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt}
}
func tokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
