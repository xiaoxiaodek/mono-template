package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"

	"github.com/vort-ads/vort-ads-template/internal/platform/response"
	"github.com/vort-ads/vort-ads-template/internal/platform/security"
)

func TestRateLimitKeySelection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.RemoteAddr = "192.0.2.9:1234"

	tests := []struct {
		name string
		key  RateLimitKeyFunc
		want string
		ok   bool
	}{
		{name: "global", key: GlobalRateLimitKey("operation-api"), want: "global:operation-api", ok: true},
		{name: "client IP", key: ClientIPRateLimitKey(), want: "ip:192.0.2.9", ok: true},
		{name: "unauthenticated user skips", key: AuthenticatedUserRateLimitKey(), ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.key(c)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("key = %q, %v; want %q, %v", got, ok, tt.want, tt.ok)
			}
		})
	}

	c.Set(PrincipalKey, security.Principal{UserID: "usr_42"})
	if got, ok := AuthenticatedUserRateLimitKey()(c); got != "user:usr_42" || !ok {
		t.Fatalf("authenticated user key = %q, %v", got, ok)
	}
}

type stubKeyedRateLimiter struct {
	allowed bool
	err     error
	key     string
	ctx     context.Context
}

func (l *stubKeyedRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	l.ctx = ctx
	l.key = key
	return l.allowed, l.err
}

func TestKeyedRateLimitUsesRequestContextAndSelectedKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := &stubKeyedRateLimiter{allowed: true}
	router := gin.New()
	router.Use(KeyedRateLimit(limiter, GlobalRateLimitKey("api")))
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if limiter.key != "global:api" || limiter.ctx != request.Context() {
		t.Fatalf("limiter call = (%v, %q), want request context and global:api", limiter.ctx, limiter.key)
	}
}

func TestKeyedRateLimitFailsClosedWhenBackendErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID(), KeyedRateLimit(
		&stubKeyedRateLimiter{err: errors.New("redis unavailable")},
		GlobalRateLimitKey("api"),
	))
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	var envelope response.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Code != "DEPENDENCY_ERROR" {
		t.Fatalf("code = %q, want DEPENDENCY_ERROR", envelope.Code)
	}
}

type fakeRedisEvaler struct {
	result any
	err    error
	script string
	keys   []string
	args   []any
}

func (f *fakeRedisEvaler) Eval(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd {
	f.script = script
	f.keys = append([]string(nil), keys...)
	f.args = append([]any(nil), args...)
	cmd := redis.NewCmd(ctx)
	if f.err != nil {
		cmd.SetErr(f.err)
	} else {
		cmd.SetVal(f.result)
	}
	return cmd
}

func TestRedisTokenBucketRateLimiterUsesServerTimeAndBoundedState(t *testing.T) {
	client := &fakeRedisEvaler{result: int64(1)}
	limiter := NewRedisTokenBucketRateLimiter(client, "ads", 25, 50)

	allowed, err := limiter.Allow(context.Background(), "user:usr_42")
	if err != nil || !allowed {
		t.Fatalf("Allow() = %v, %v; want true, nil", allowed, err)
	}
	if len(client.keys) != 1 || client.keys[0] != "ads:user:usr_42" {
		t.Fatalf("keys = %#v", client.keys)
	}
	if len(client.args) != 3 || client.args[0] != int64(25) || client.args[1] != int64(50) {
		t.Fatalf("args = %#v, want rate, burst, and idle TTL", client.args)
	}
	ttl, ok := client.args[2].(int64)
	if !ok || ttl < 4_000 {
		t.Fatalf("idle TTL = %#v, want at least twice the 2s refill duration", client.args[2])
	}
	for _, fragment := range []string{
		`redis.call("TIME")`,
		`redis.call("HMGET", KEYS[1], "tokens_micro", "last_ms")`,
		`redis.call("HSET", KEYS[1]`,
		`redis.call("PEXPIRE", KEYS[1], ARGV[3])`,
		`if elapsed_ms >= fill_ms then`,
	} {
		if !strings.Contains(client.script, fragment) {
			t.Fatalf("Lua script missing %q: %s", fragment, client.script)
		}
	}
}

func TestRedisTokenBucketRateLimiterDeniesWhenScriptExhaustsBurst(t *testing.T) {
	limiter := NewRedisTokenBucketRateLimiter(&fakeRedisEvaler{result: int64(0)}, "ads", 1, 2)

	allowed, err := limiter.Allow(context.Background(), "global:api")
	if err != nil || allowed {
		t.Fatalf("Allow() = %v, %v; want false, nil", allowed, err)
	}
}

func TestRedisTokenBucketRateLimiterSurfacesRedisErrors(t *testing.T) {
	want := errors.New("redis unavailable")
	limiter := NewRedisTokenBucketRateLimiter(&fakeRedisEvaler{err: want}, "ads", 1, 2)

	if _, err := limiter.Allow(context.Background(), "global:api"); !errors.Is(err, want) {
		t.Fatalf("Allow() error = %v, want %v", err, want)
	}
}

func TestRedisTokenBucketRateLimiterAvoidsUnsafeLuaIntermediatesAtBoundary(t *testing.T) {
	client := &fakeRedisEvaler{result: int64(1)}
	maxTokens := maxLuaSafeInteger / tokenScale
	limiter := NewRedisTokenBucketRateLimiter(client, "ads", maxTokens, maxTokens)

	if _, err := limiter.Allow(context.Background(), "global:api"); err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	for _, fragment := range []string{
		`if elapsed_ms >= fill_ms then`,
		`local missing_micro = capacity - tokens_micro`,
		`if refill_micro >= missing_micro then`,
	} {
		if !strings.Contains(client.script, fragment) {
			t.Fatalf("Lua script missing overflow guard %q: %s", fragment, client.script)
		}
	}
	if strings.Contains(client.script, `elapsed_ms = math.min(elapsed_ms, fill_ms)`) {
		t.Fatalf("Lua script still multiplies at fill_ms, which can exceed 2^53: %s", client.script)
	}
	if client.args[0] != maxTokens || client.args[1] != maxTokens {
		t.Fatalf("boundary args = %#v, want max safe rate and burst", client.args)
	}
}

func TestRedisTokenBucketRateLimiterAcceptsMaximumSafeBurstWithoutGoOverflow(t *testing.T) {
	maxTokens := maxLuaSafeInteger / tokenScale
	limiter := NewRedisTokenBucketRateLimiter(&fakeRedisEvaler{result: int64(1)}, "ads", 1, maxTokens)

	if limiter.idleTTLMS <= 0 {
		t.Fatalf("idle TTL = %d, want positive value", limiter.idleTTLMS)
	}
	wantFillMS := maxTokens * 1000
	if limiter.idleTTLMS != wantFillMS*2 {
		t.Fatalf("idle TTL = %d, want %d", limiter.idleTTLMS, wantFillMS*2)
	}
}

func TestLocalKeyedRateLimiterStartsAtBurstAndRefills(t *testing.T) {
	limiter := NewLocalKeyedRateLimiter(rate.Limit(2), 2, time.Minute, 32)
	now := time.Unix(1_700_000_000, 0)
	limiter.limiter.now = func() time.Time { return now }

	for request := 1; request <= 2; request++ {
		if allowed, err := limiter.Allow(context.Background(), "user:usr_42"); err != nil || !allowed {
			t.Fatalf("request %d = %v, %v; want allowed", request, allowed, err)
		}
	}
	if allowed, err := limiter.Allow(context.Background(), "user:usr_42"); err != nil || allowed {
		t.Fatalf("exhausted bucket = %v, %v; want denied", allowed, err)
	}

	now = now.Add(500 * time.Millisecond)
	if allowed, err := limiter.Allow(context.Background(), "user:usr_42"); err != nil || !allowed {
		t.Fatalf("refilled bucket = %v, %v; want one token", allowed, err)
	}
}

func TestLocalKeyedRateLimiterBoundsKeysUnderConcurrency(t *testing.T) {
	limiter := NewLocalKeyedRateLimiter(rate.Inf, 1, time.Hour, 8)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			allowed, err := limiter.Allow(context.Background(), key)
			if err != nil || !allowed {
				t.Errorf("Allow(%q) = %v, %v", key, allowed, err)
			}
		}(fmt.Sprintf("ip:192.0.2.%d", i))
	}
	wg.Wait()

	limiter.limiter.mu.Lock()
	defer limiter.limiter.mu.Unlock()
	if got := len(limiter.limiter.visitors); got > 8 {
		t.Fatalf("visitor count = %d, want <= 8", got)
	}
}

func TestClientIPRateLimiterConcurrentAccess(t *testing.T) {
	limiter := NewClientIPRateLimiter(rate.Inf, 1, time.Minute, 32)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !limiter.Allow("192.0.2.1") {
				t.Error("rate.Inf limiter denied request")
			}
		}()
	}
	wg.Wait()
}
