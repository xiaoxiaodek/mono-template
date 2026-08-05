package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vort-ads/vort-ads-template/internal/platform/config"
)

type stubPinger struct{ err error }

func (p stubPinger) Ping(context.Context) error { return p.err }

func TestNewConfiguresPoolBounds(t *testing.T) {
	pool, err := New(context.Background(), config.DatabaseConfig{
		DSN:             "postgres://user:pass@127.0.0.1:1/example?sslmode=disable", // #nosec G101 -- unreachable test fixture.
		MaxOpenConns:    12,
		MaxIdleConns:    3,
		ConnMaxLifetime: 7 * time.Minute,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(pool.Close)

	got := pool.Config()
	if got.MaxConns != 12 || got.MinConns != 3 {
		t.Fatalf("pool bounds = max %d min %d, want max 12 min 3", got.MaxConns, got.MinConns)
	}
	if got.MaxConnLifetime != 7*time.Minute {
		t.Fatalf("MaxConnLifetime = %s, want 7m", got.MaxConnLifetime)
	}
}

func TestPingReturnsDependencyError(t *testing.T) {
	want := errors.New("database unavailable")
	if got := Ping(context.Background(), stubPinger{err: want}); !errors.Is(got, want) {
		t.Fatalf("Ping error = %v, want %v", got, want)
	}
}
