package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/vort-ads/vort-ads-template/apps/internal/platform/config"
)

const (
	MaxPoolSize      = 10
	operationTimeout = 2 * time.Second
)

// Pinger is implemented by redis.Client.
type Pinger interface {
	Ping(context.Context) *redis.StatusCmd
}

// New creates a Redis client with explicit pool and network-operation bounds.
func New(cfg config.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		PoolSize:     MaxPoolSize,
		MinIdleConns: 1,
		PoolTimeout:  operationTimeout,
		DialTimeout:  operationTimeout,
		ReadTimeout:  operationTimeout,
		WriteTimeout: operationTimeout,
	})
}

// Ping checks whether Redis is reachable within the caller's context.
func Ping(ctx context.Context, pinger Pinger) error {
	if err := pinger.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}
	return nil
}
