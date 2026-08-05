package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const redisFixedWindowScript = `
local current = redis.call("INCR", KEYS[1])
local ttl = redis.call("PTTL", KEYS[1])
if current == 1 or ttl < 0 then
  redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
if current <= tonumber(ARGV[1]) then
  return 1
end
return 0
`

const maxLuaSafeInteger = int64(1<<53 - 1)

type redisEvaler interface {
	Eval(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd
}

var _ redisEvaler = (*redis.Client)(nil)

// RedisFixedWindowRateLimiter uses one atomic Lua invocation per decision.
// Each counter receives an explicit TTL on creation, bounding its lifetime.
type RedisFixedWindowRateLimiter struct {
	client   redisEvaler
	prefix   string
	limit    int64
	windowMS int64
}

func NewRedisFixedWindowRateLimiter(
	client redisEvaler,
	prefix string,
	limit int64,
	window time.Duration,
) *RedisFixedWindowRateLimiter {
	if client == nil || prefix == "" || limit <= 0 || window <= 0 {
		panic("redis fixed-window rate limiter requires a client, prefix, positive limit, and positive window")
	}
	if limit > maxLuaSafeInteger {
		panic("redis fixed-window rate limiter limit exceeds Lua's safe integer range")
	}
	windowMS := window.Milliseconds()
	if windowMS <= 0 {
		panic("redis fixed-window rate limiter window must be at least one millisecond")
	}
	return &RedisFixedWindowRateLimiter{
		client:   client,
		prefix:   prefix,
		limit:    limit,
		windowMS: windowMS,
	}
}

func (l *RedisFixedWindowRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	result, err := l.client.Eval(
		ctx,
		redisFixedWindowScript,
		[]string{l.prefix + ":" + key},
		l.limit,
		l.windowMS,
	).Int64()
	if err != nil {
		return false, fmt.Errorf("evaluate Redis rate limit: %w", err)
	}
	return result == 1, nil
}
