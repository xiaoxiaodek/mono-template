package memory

import (
	"context"
	"sync"
	"time"

	bizidentity "github.com/vort-ads/vort-ads-template/apps/control-api/internal/biz/identity"
)

var _ bizidentity.RefreshTokenStore = (*TokenStore)(nil)

type refreshToken struct {
	userID    string
	expiresAt time.Time
}
type TokenStore struct {
	mu     sync.Mutex
	tokens map[string]refreshToken
}

func NewTokenStore() *TokenStore { return &TokenStore{tokens: make(map[string]refreshToken)} }
func (s *TokenStore) Save(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[tokenHash] = refreshToken{userID: userID, expiresAt: expiresAt}
	return nil
}
func (s *TokenStore) Exists(ctx context.Context, tokenHash string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	token, exists := s.tokens[tokenHash]
	if !exists {
		return false, nil
	}
	if !token.expiresAt.After(time.Now()) {
		delete(s.tokens, tokenHash)
		return false, nil
	}
	return true, nil
}
func (s *TokenStore) Rotate(ctx context.Context, oldHash, userID, newHash string, expiresAt time.Time) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, exists := s.tokens[oldHash]
	if !exists {
		return false, nil
	}
	if !old.expiresAt.After(time.Now()) {
		delete(s.tokens, oldHash)
		return false, nil
	}
	if old.userID != userID {
		return false, nil
	}
	delete(s.tokens, oldHash)
	s.tokens[newHash] = refreshToken{userID: userID, expiresAt: expiresAt}
	return true, nil
}
