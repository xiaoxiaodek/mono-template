package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vort-ads/vort-ads-template/internal/platform/config"
)

// Pinger is implemented by pgxpool.Pool and keeps dependency health checks
// easy to compose and test.
type Pinger interface {
	Ping(context.Context) error
}

// New creates a lazy PostgreSQL pool with all configured connection bounds.
// Connectivity is checked separately with Ping so callers can decide whether
// an unavailable dependency is a startup or readiness failure.
func New(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}
	poolConfig.MaxConns = cfg.MaxOpenConns
	poolConfig.MinConns = cfg.MaxIdleConns
	poolConfig.MaxConnLifetime = cfg.ConnMaxLifetime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	return pool, nil
}

// Ping checks whether PostgreSQL is reachable within the caller's context.
func Ping(ctx context.Context, pinger Pinger) error {
	if err := pinger.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	return nil
}
