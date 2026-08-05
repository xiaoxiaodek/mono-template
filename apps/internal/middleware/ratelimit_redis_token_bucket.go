package middleware

import (
	"context"
	"fmt"
)

const (
	tokenScale       = int64(1_000_000)
	minimumIdleTTLMS = int64(60_000)
)

const redisTokenBucketScript = `
local server_time = redis.call("TIME")
local now_ms = tonumber(server_time[1]) * 1000 + math.floor(tonumber(server_time[2]) / 1000)
local rate_per_second = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local token_scale = 1000000
local capacity = burst * token_scale

local state = redis.call("HMGET", KEYS[1], "tokens_micro", "last_ms")
local tokens_micro = tonumber(state[1])
local last_ms = tonumber(state[2])
if tokens_micro == nil or last_ms == nil then
  tokens_micro = capacity
else
  local elapsed_ms = math.max(0, now_ms - last_ms)
  local fill_ms = math.ceil(burst * 1000 / rate_per_second)
  tokens_micro = math.max(0, math.min(capacity, tokens_micro))
  if elapsed_ms >= fill_ms then
    tokens_micro = capacity
  else
    local refill_micro = elapsed_ms * rate_per_second * 1000
    local missing_micro = capacity - tokens_micro
    if refill_micro >= missing_micro then
      tokens_micro = capacity
    else
      tokens_micro = tokens_micro + refill_micro
    end
  end
end

local allowed = 0
if tokens_micro >= token_scale then
  tokens_micro = tokens_micro - token_scale
  allowed = 1
end

redis.call("HSET", KEYS[1], "tokens_micro", math.floor(tokens_micro), "last_ms", now_ms)
redis.call("PEXPIRE", KEYS[1], ARGV[3])
return allowed
`

// RedisTokenBucketRateLimiter stores a token bucket in a Redis hash and uses
// Redis server time, so all pods observe one clock and one atomic state update.
type RedisTokenBucketRateLimiter struct {
	client    redisEvaler
	prefix    string
	rate      int64
	burst     int64
	idleTTLMS int64
}

func NewRedisTokenBucketRateLimiter(
	client redisEvaler,
	prefix string,
	ratePerSecond int64,
	burst int64,
) *RedisTokenBucketRateLimiter {
	maxTokens := maxLuaSafeInteger / tokenScale
	if client == nil || prefix == "" || ratePerSecond <= 0 || burst <= 0 {
		panic("redis token bucket rate limiter requires a client, prefix, positive rate, and positive burst")
	}
	if ratePerSecond > maxTokens || burst > maxTokens {
		panic("redis token bucket rate or burst exceeds Lua's safe integer range")
	}

	fillMS := tokenBucketFillMilliseconds(ratePerSecond, burst)
	idleTTLMS := fillMS * 2
	if idleTTLMS < minimumIdleTTLMS {
		idleTTLMS = minimumIdleTTLMS
	}
	return &RedisTokenBucketRateLimiter{
		client:    client,
		prefix:    prefix,
		rate:      ratePerSecond,
		burst:     burst,
		idleTTLMS: idleTTLMS,
	}
}

func tokenBucketFillMilliseconds(ratePerSecond, burst int64) int64 {
	wholeSeconds := burst / ratePerSecond
	remainder := burst % ratePerSecond
	fillMS := wholeSeconds * 1000
	if remainder != 0 {
		fillMS += (remainder*1000 + ratePerSecond - 1) / ratePerSecond
	}
	return fillMS
}

func (l *RedisTokenBucketRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	result, err := l.client.Eval(
		ctx,
		redisTokenBucketScript,
		[]string{l.prefix + ":" + key},
		l.rate,
		l.burst,
		l.idleTTLMS,
	).Int64()
	if err != nil {
		return false, fmt.Errorf("evaluate Redis token bucket: %w", err)
	}
	return result == 1, nil
}
