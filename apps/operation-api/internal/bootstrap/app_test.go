package bootstrap

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	redisclient "github.com/redis/go-redis/v9"

	"github.com/vort-ads/vort-ads-template/apps/operation-api/internal/data/identity/memory"
	"github.com/vort-ads/vort-ads-template/apps/operation-api/internal/data/identity/postgres"
	identityredis "github.com/vort-ads/vort-ads-template/apps/operation-api/internal/data/identity/redis"
	"github.com/vort-ads/vort-ads-template/internal/middleware"
)

func TestSelectRefreshTokenStoreUsesRedisWhenAvailable(t *testing.T) {
	client := redisclient.NewClient(&redisclient.Options{Addr: "localhost:0"})
	t.Cleanup(func() { _ = client.Close() })

	store := selectRefreshTokenStore(nil, client, true, false)
	if _, ok := store.(*identityredis.TokenStore); !ok {
		t.Fatalf("store type = %T, want Redis token store", store)
	}
}

func TestSelectRefreshTokenStoreFallsBackWhenOptionalRedisUnavailable(t *testing.T) {
	client := redisclient.NewClient(&redisclient.Options{Addr: "localhost:0"})
	t.Cleanup(func() { _ = client.Close() })

	store := selectRefreshTokenStore(nil, client, false, false)
	if _, ok := store.(*memory.TokenStore); !ok {
		t.Fatalf("store type = %T, want memory token store", store)
	}
}

func TestSelectRefreshTokenStoreKeepsRedisWhenRequiredButUnavailable(t *testing.T) {
	client := redisclient.NewClient(&redisclient.Options{Addr: "localhost:0"})
	t.Cleanup(func() { _ = client.Close() })

	store := selectRefreshTokenStore(nil, client, false, true)
	if _, ok := store.(*identityredis.TokenStore); !ok {
		t.Fatalf("store type = %T, want Redis token store", store)
	}
}

func TestSelectRefreshTokenStorePrefersPostgresWhenPoolExists(t *testing.T) {
	client := redisclient.NewClient(&redisclient.Options{Addr: "localhost:0"})
	t.Cleanup(func() { _ = client.Close() })

	store := selectRefreshTokenStore(&pgxpool.Pool{}, client, true, true)
	if _, ok := store.(*postgres.RefreshTokenStore); !ok {
		t.Fatalf("store type = %T, want PostgreSQL token store", store)
	}
}

func TestConfigureGinModeUsesReleaseModeInProduction(t *testing.T) {
	previous := gin.Mode()
	t.Cleanup(func() { gin.SetMode(previous) })
	gin.SetMode(gin.TestMode)

	configureGinMode("prod")

	if got := gin.Mode(); got != gin.ReleaseMode {
		t.Fatalf("Gin mode = %q, want %q", got, gin.ReleaseMode)
	}
}

func TestConfigureGinModeDoesNotOverrideNonProductionMode(t *testing.T) {
	previous := gin.Mode()
	t.Cleanup(func() { gin.SetMode(previous) })
	gin.SetMode(gin.TestMode)

	configureGinMode("dev")

	if got := gin.Mode(); got != gin.TestMode {
		t.Fatalf("Gin mode = %q, want existing %q", got, gin.TestMode)
	}
}

func TestRequestTimeoutLeavesWriteTimeoutHeadroom(t *testing.T) {
	for _, writeTimeout := range []time.Duration{10 * time.Second, 2 * time.Second, time.Millisecond, 2 * time.Nanosecond} {
		requestTimeout := requestTimeoutFor(writeTimeout)
		if requestTimeout <= 0 || requestTimeout >= writeTimeout {
			t.Fatalf("request timeout = %v, want 0 < timeout < write timeout %v", requestTimeout, writeTimeout)
		}
	}
}

func TestSelectRateLimitPoliciesUsesRedisTokenBucketsWhenAvailable(t *testing.T) {
	client := redisclient.NewClient(&redisclient.Options{Addr: "localhost:0"})
	t.Cleanup(func() { _ = client.Close() })

	policies := selectRateLimitPolicies(client, true, false, false)
	for name, limiter := range map[string]any{
		"global": policies.global,
		"IP":     policies.clientIP,
		"user":   policies.user,
	} {
		if _, ok := limiter.(*middleware.RedisTokenBucketRateLimiter); !ok {
			t.Fatalf("%s limiter type = %T, want Redis token bucket", name, limiter)
		}
	}
}

func TestSelectRateLimitPoliciesUsesBoundedLocalFallbackWhenOptionalRedisUnavailable(t *testing.T) {
	policies := selectRateLimitPolicies(nil, false, false, false)
	for name, limiter := range map[string]any{
		"global": policies.global,
		"IP":     policies.clientIP,
		"user":   policies.user,
	} {
		if _, ok := limiter.(*middleware.LocalKeyedRateLimiter); !ok {
			t.Fatalf("%s limiter type = %T, want bounded local token bucket", name, limiter)
		}
	}
}

func TestSelectRateLimitPoliciesKeepsRedisFailClosedWhenRequiredButUnavailable(t *testing.T) {
	client := redisclient.NewClient(&redisclient.Options{Addr: "localhost:0"})
	t.Cleanup(func() { _ = client.Close() })

	policies := selectRateLimitPolicies(client, false, true, false)
	if _, ok := policies.global.(*middleware.RedisTokenBucketRateLimiter); !ok {
		t.Fatalf("global limiter type = %T, want Redis token bucket", policies.global)
	}
}
