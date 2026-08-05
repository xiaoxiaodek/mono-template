package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	redisclient "github.com/redis/go-redis/v9"

	bizidentity "github.com/vort-ads/vort-ads-template/apps/operation-api/internal/biz/identity"
)

const refreshTokenKeyPrefix = "identity:refresh:"

const rotateTokenScript = `
local current = redis.call('GET', KEYS[1])
if not current or current ~= ARGV[1] then
    return 0
end
local ttl = tonumber(ARGV[2])
if not ttl or ttl <= 0 then
    return 0
end
redis.call('DEL', KEYS[1])
redis.call('SET', KEYS[2], ARGV[1], 'PX', ttl)
return 1
`

var ErrTokenAlreadyExpired = errors.New("refresh token already expired")

type Client interface {
	Set(context.Context, string, any, time.Duration) *redisclient.StatusCmd
	Eval(context.Context, string, []string, ...any) *redisclient.Cmd
}

type TokenStore struct {
	client Client
	now    func() time.Time
}

var _ bizidentity.RefreshTokenStore = (*TokenStore)(nil)

func NewTokenStore(client Client) *TokenStore {
	return &TokenStore{client: client, now: time.Now}
}

func (s *TokenStore) Save(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	ttl, err := s.ttl(expiresAt)
	if err != nil {
		return err
	}
	if err := s.client.Set(ctx, tokenKey(tokenHash), userID, ttl).Err(); err != nil {
		return fmt.Errorf("save refresh token: %w", err)
	}
	return nil
}

func (s *TokenStore) Rotate(ctx context.Context, oldHash, userID, newHash string, expiresAt time.Time) (bool, error) {
	ttl, err := s.ttl(expiresAt)
	if err != nil {
		return false, err
	}
	ttlMilliseconds := ttl.Milliseconds()
	if ttlMilliseconds == 0 {
		ttlMilliseconds = 1
	}
	result, err := s.client.Eval(
		ctx,
		rotateTokenScript,
		[]string{tokenKey(oldHash), tokenKey(newHash)},
		userID,
		ttlMilliseconds,
	).Int64()
	if err != nil {
		return false, fmt.Errorf("rotate refresh token: %w", err)
	}
	return result == 1, nil
}

func (s *TokenStore) ttl(expiresAt time.Time) (time.Duration, error) {
	ttl := expiresAt.Sub(s.now())
	if ttl <= 0 {
		return 0, ErrTokenAlreadyExpired
	}
	return ttl, nil
}

func tokenKey(hash string) string {
	return refreshTokenKeyPrefix + hash
}
