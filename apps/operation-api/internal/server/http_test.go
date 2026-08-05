package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"

	bizidentity "github.com/vort-ads/vort-ads-template/apps/control-api/internal/biz/identity"
	identitydata "github.com/vort-ads/vort-ads-template/apps/control-api/internal/data/identity"
	"github.com/vort-ads/vort-ads-template/apps/control-api/internal/data/identity/memory"
	identityservice "github.com/vort-ads/vort-ads-template/apps/control-api/internal/service/identity"
	"github.com/vort-ads/vort-ads-template/apps/internal/middleware"
	"github.com/vort-ads/vort-ads-template/apps/internal/platform/security"
	"github.com/vort-ads/vort-ads-template/apps/pkg/idgen"
)

type recordingKeyedLimiter struct {
	mu   sync.Mutex
	keys []string
}

func (l *recordingKeyedLimiter) Allow(_ context.Context, key string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.keys = append(l.keys, key)
	return true, nil
}

func (l *recordingKeyedLimiter) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.keys...)
}

func TestHealthzReturnsOK(t *testing.T) {
	router := NewHTTPServer(Dependencies{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestOperationalRoutesAreRegistered(t *testing.T) {
	engine := NewHTTPServer(Dependencies{})
	routes := make(map[string]bool)
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, endpoint := range []struct{ Method, Path string }{
		{http.MethodGet, "/healthz"},
		{http.MethodGet, "/readyz"},
	} {
		if !routes[endpoint.Method+" "+endpoint.Path] {
			t.Fatalf("routes = %#v, missing descriptor %+v", routes, endpoint)
		}
	}
}

func TestReadyzReturnsServiceUnavailableWhenDependencyFails(t *testing.T) {
	router := NewHTTPServer(Dependencies{
		ReadinessChecks: map[string]ReadinessCheck{
			"postgres": func(context.Context) error { return errors.New("unavailable") },
		},
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestMetricsEndpointIsRegistered(t *testing.T) {
	router := NewHTTPServer(Dependencies{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestRateLimitAppliesOnlyToAPIRoutes(t *testing.T) {
	manager := security.NewJWTManager("test-secret-with-enough-length", time.Minute, time.Hour)
	usecase := bizidentity.NewUsecase(
		memory.NewUserRepository(),
		memory.NewTokenStore(),
		security.BcryptPasswordHasher{Cost: 4},
		identitydata.NewTokenManager(manager),
		idgen.New,
		nil,
	)
	identityHandler := identityservice.NewHandler(usecase, middleware.Auth(manager))
	clientLimiter := middleware.NewClientIPRateLimiter(rate.Every(time.Hour), 1, time.Hour, 100)
	router := NewHTTPServer(Dependencies{IdentityHandler: &identityHandler, RateLimiter: clientLimiter})

	request := func(path string) int {
		response := httptest.NewRecorder()
		httpRequest := httptest.NewRequest(http.MethodGet, path, nil)
		httpRequest.RemoteAddr = "192.0.2.1:1234"
		router.ServeHTTP(response, httpRequest)
		return response.Code
	}

	if got := request("/api/v1/me"); got != http.StatusUnauthorized {
		t.Fatalf("first API status = %d", got)
	}
	if got := request("/api/v1/me"); got != http.StatusTooManyRequests {
		t.Fatalf("second API status = %d", got)
	}
	if got := request("/healthz"); got != http.StatusOK {
		t.Fatalf("first health status = %d", got)
	}
	if got := request("/healthz"); got != http.StatusOK {
		t.Fatalf("second health status = %d", got)
	}
}

func TestGlobalIPAndAuthenticatedUserRateLimitsAreWiredInOrder(t *testing.T) {
	manager := security.NewJWTManager("test-secret-with-enough-length", time.Minute, time.Hour)
	usecase := bizidentity.NewUsecase(
		memory.NewUserRepository(),
		memory.NewTokenStore(),
		security.BcryptPasswordHasher{Cost: 4},
		identitydata.NewTokenManager(manager),
		idgen.New,
		nil,
	)
	identityHandler := identityservice.NewHandler(usecase, middleware.Auth(manager))
	globalLimiter := &recordingKeyedLimiter{}
	ipLimiter := &recordingKeyedLimiter{}
	userLimiter := &recordingKeyedLimiter{}
	router := NewHTTPServer(Dependencies{
		IdentityHandler:      &identityHandler,
		GlobalRateLimiter:    globalLimiter,
		ClientIPKeyedLimiter: ipLimiter,
		UserRateLimiter:      userLimiter,
	})
	token, err := manager.SignAccessToken(security.Principal{UserID: "usr_42", Email: "user@example.com"})
	if err != nil {
		t.Fatalf("SignAccessToken: %v", err)
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.RemoteAddr = "192.0.2.9:1234"
	request.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(response, request)

	if got := globalLimiter.snapshot(); len(got) != 1 || got[0] != "global:control-api" {
		t.Fatalf("global keys = %#v", got)
	}
	if got := ipLimiter.snapshot(); len(got) != 1 || got[0] != "ip:192.0.2.9" {
		t.Fatalf("IP keys = %#v", got)
	}
	if got := userLimiter.snapshot(); len(got) != 1 || got[0] != "user:usr_42" {
		t.Fatalf("user keys = %#v; limiter must run after auth context is established", got)
	}
}

func TestOperationalEndpointsBypassAllAPIRateLimits(t *testing.T) {
	globalLimiter := &recordingKeyedLimiter{}
	ipLimiter := &recordingKeyedLimiter{}
	userLimiter := &recordingKeyedLimiter{}
	router := NewHTTPServer(Dependencies{
		GlobalRateLimiter:    globalLimiter,
		ClientIPKeyedLimiter: ipLimiter,
		UserRateLimiter:      userLimiter,
	})

	for _, path := range []string{"/healthz", "/readyz", "/metrics"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	}

	if len(globalLimiter.snapshot()) != 0 || len(ipLimiter.snapshot()) != 0 || len(userLimiter.snapshot()) != 0 {
		t.Fatalf("operational endpoints reached API rate limiters: global=%v ip=%v user=%v",
			globalLimiter.snapshot(), ipLimiter.snapshot(), userLimiter.snapshot())
	}
}

func TestRateLimitIgnoresForwardedForFromUntrustedPeer(t *testing.T) {
	router := newRateLimitedIdentityRouter(t, nil)

	if got := performAPIRequest(router, "192.0.2.1:1234", "198.51.100.1"); got != http.StatusUnauthorized {
		t.Fatalf("first API status = %d", got)
	}
	if got := performAPIRequest(router, "192.0.2.1:1234", "198.51.100.2"); got != http.StatusTooManyRequests {
		t.Fatalf("second API status = %d, want same untrusted-peer bucket", got)
	}
}

func TestRateLimitUsesForwardedForFromTrustedProxy(t *testing.T) {
	router := newRateLimitedIdentityRouter(t, []string{"192.0.2.0/24"})

	if got := performAPIRequest(router, "192.0.2.1:1234", "198.51.100.1"); got != http.StatusUnauthorized {
		t.Fatalf("first API status = %d", got)
	}
	if got := performAPIRequest(router, "192.0.2.1:1234", "198.51.100.2"); got != http.StatusUnauthorized {
		t.Fatalf("second API status = %d, want separate trusted-forwarded bucket", got)
	}
}

func newRateLimitedIdentityRouter(t *testing.T, trustedProxies []string) http.Handler {
	t.Helper()
	manager := security.NewJWTManager("test-secret-with-enough-length", time.Minute, time.Hour)
	usecase := bizidentity.NewUsecase(
		memory.NewUserRepository(),
		memory.NewTokenStore(),
		security.BcryptPasswordHasher{Cost: 4},
		identitydata.NewTokenManager(manager),
		idgen.New,
		nil,
	)
	identityHandler := identityservice.NewHandler(usecase, middleware.Auth(manager))
	clientLimiter := middleware.NewClientIPRateLimiter(rate.Every(time.Hour), 1, time.Hour, 100)
	return NewHTTPServer(Dependencies{
		IdentityHandler: &identityHandler,
		RateLimiter:     clientLimiter,
		TrustedProxies:  trustedProxies,
	})
}

func performAPIRequest(handler http.Handler, remoteAddr, forwardedFor string) int {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.RemoteAddr = remoteAddr
	request.Header.Set("X-Forwarded-For", forwardedFor)
	handler.ServeHTTP(response, request)
	return response.Code
}
