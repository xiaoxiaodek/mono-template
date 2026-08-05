package security

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Principal struct {
	UserID      string
	Email       string
	Roles       []string
	Permissions []string
	// JTI is an optional unique token identifier. When set, it populates the
	// standard jti claim so every issued token is cryptographically distinct.
	JTI string
}

type Claims struct {
	UserID      string   `json:"uid"`
	Email       string   `json:"email"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	TokenType   string   `json:"typ"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewJWTManager(secret string, accessTTL time.Duration, refreshTTL time.Duration) JWTManager {
	return JWTManager{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

func (m JWTManager) SignAccessToken(principal Principal) (string, error) {
	return m.sign(principal, "access", m.accessTTL)
}

func (m JWTManager) SignRefreshToken(principal Principal) (string, error) {
	return m.sign(principal, "refresh", m.refreshTTL)
}

func (m JWTManager) VerifyAccessToken(tokenString string) (*Claims, error) {
	claims, err := m.verify(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != "access" {
		return nil, errors.New("token is not access token")
	}
	return claims, nil
}

func (m JWTManager) VerifyRefreshToken(tokenString string) (*Claims, error) {
	claims, err := m.verify(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != "refresh" {
		return nil, errors.New("token is not refresh token")
	}
	return claims, nil
}

func (m JWTManager) sign(principal Principal, tokenType string, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID:      principal.UserID,
		Email:       principal.Email,
		Roles:       principal.Roles,
		Permissions: principal.Permissions,
		TokenType:   tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        principal.JTI,
			Subject:   principal.UserID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

func (m JWTManager) verify(tokenString string) (*Claims, error) {
	claims := new(Claims)
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("unexpected signing method")
			}
			return m.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
