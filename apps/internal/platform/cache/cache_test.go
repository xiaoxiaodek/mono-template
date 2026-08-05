package cache

import (
	"context"
	"errors"
	"testing"

	"github.com/redis/go-redis/v9"

	"github.com/vort-ads/vort-ads-template/apps/internal/platform/config"
)

type stubPinger struct {
	result *redis.StatusCmd
	err    error
}

func (p stubPinger) Ping(context.Context) *redis.StatusCmd {
	cmd := p.result
	if cmd == nil {
		cmd = redis.NewStatusCmd(context.Background())
	}
	cmd.SetErr(p.err)
	return cmd
}

func TestNewConfiguresBoundedPoolAndOperationTimeouts(t *testing.T) {
	client := New(config.RedisConfig{Addr: "127.0.0.1:6379", Password: "secret"})
	t.Cleanup(func() { _ = client.Close() })

	options := client.Options()
	if options.PoolSize <= 0 || options.PoolSize > MaxPoolSize {
		t.Fatalf("PoolSize = %d, want 1..%d", options.PoolSize, MaxPoolSize)
	}
	if options.DialTimeout <= 0 || options.ReadTimeout <= 0 || options.WriteTimeout <= 0 {
		t.Fatalf("timeouts must be bounded: dial=%s read=%s write=%s", options.DialTimeout, options.ReadTimeout, options.WriteTimeout)
	}
	if options.Password != "secret" {
		t.Fatalf("Password not applied")
	}
}

func TestPingReturnsDependencyError(t *testing.T) {
	want := errors.New("redis unavailable")
	if got := Ping(context.Background(), stubPinger{err: want}); !errors.Is(got, want) {
		t.Fatalf("Ping error = %v, want %v", got, want)
	}
}
