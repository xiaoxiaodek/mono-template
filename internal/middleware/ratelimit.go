package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/vort-ads/vort-ads-template/internal/platform/apperrors"
	"github.com/vort-ads/vort-ads-template/internal/platform/response"
)

// KeyedRateLimiter allows a request identified by key. Implementations return
// backend errors so callers can choose an explicit availability policy.
type KeyedRateLimiter interface {
	Allow(context.Context, string) (bool, error)
}

// RateLimitKeyFunc selects the bucket for a request. A false result skips
// limiting, which is how authenticated-user limiting handles anonymous calls.
type RateLimitKeyFunc func(*gin.Context) (string, bool)

func GlobalRateLimitKey(scope string) RateLimitKeyFunc {
	return func(*gin.Context) (string, bool) {
		return "global:" + scope, true
	}
}

func ClientIPRateLimitKey() RateLimitKeyFunc {
	return func(c *gin.Context) (string, bool) {
		return "ip:" + c.ClientIP(), true
	}
}

func AuthenticatedUserRateLimitKey() RateLimitKeyFunc {
	return func(c *gin.Context) (string, bool) {
		principal, ok := GetPrincipal(c)
		if !ok || principal.UserID == "" {
			return "", false
		}
		return "user:" + principal.UserID, true
	}
}

// AuthEndpointIPKey limits IPs on authentication endpoints (login, register,
// refresh) independently from the general per-IP limiter. Use a much lower
// rate to prevent brute-force attacks.
func AuthEndpointIPKey() RateLimitKeyFunc {
	return func(c *gin.Context) (string, bool) {
		return "auth_ip:" + c.ClientIP(), true
	}
}

type rateLimitVisitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type ClientIPRateLimiter struct {
	mu          sync.Mutex
	limit       rate.Limit
	burst       int
	visitorTTL  time.Duration
	maxVisitors int
	visitors    map[string]*rateLimitVisitor
	nextCleanup time.Time
	now         func() time.Time
}

// LocalKeyedRateLimiter adapts the bounded in-process token bucket to the
// context-aware limiter interface used by global, IP, and user policies.
// Keys expire after visitorTTL and maxKeys bounds memory use under key churn.
type LocalKeyedRateLimiter struct {
	limiter *ClientIPRateLimiter
}

func NewLocalKeyedRateLimiter(limit rate.Limit, burst int, keyTTL time.Duration, maxKeys int) *LocalKeyedRateLimiter {
	return &LocalKeyedRateLimiter{
		limiter: NewClientIPRateLimiter(limit, burst, keyTTL, maxKeys),
	}
}

func (l *LocalKeyedRateLimiter) Allow(_ context.Context, key string) (bool, error) {
	return l.limiter.Allow(key), nil
}

func NewClientIPRateLimiter(limit rate.Limit, burst int, visitorTTL time.Duration, maxVisitors int) *ClientIPRateLimiter {
	if limit <= 0 || burst <= 0 || visitorTTL <= 0 || maxVisitors <= 0 {
		panic("client IP rate limiter requires positive limits")
	}
	return &ClientIPRateLimiter{
		limit:       limit,
		burst:       burst,
		visitorTTL:  visitorTTL,
		maxVisitors: maxVisitors,
		visitors:    make(map[string]*rateLimitVisitor),
		now:         time.Now,
	}
}

func (l *ClientIPRateLimiter) Allow(clientIP string) bool {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.nextCleanup.IsZero() || !now.Before(l.nextCleanup) {
		l.deleteExpired(now)
		l.nextCleanup = now.Add(l.visitorTTL)
	}
	visitor, exists := l.visitors[clientIP]
	if !exists {
		if len(l.visitors) >= l.maxVisitors {
			l.deleteOldest()
		}
		visitor = &rateLimitVisitor{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.visitors[clientIP] = visitor
	}
	visitor.lastSeen = now
	return visitor.limiter.AllowN(now, 1)
}

func (l *ClientIPRateLimiter) deleteExpired(now time.Time) {
	for clientIP, visitor := range l.visitors {
		if now.Sub(visitor.lastSeen) >= l.visitorTTL {
			delete(l.visitors, clientIP)
		}
	}
}

func (l *ClientIPRateLimiter) deleteOldest() {
	var oldestIP string
	var oldest time.Time
	for clientIP, visitor := range l.visitors {
		if oldestIP == "" || visitor.lastSeen.Before(oldest) {
			oldestIP = clientIP
			oldest = visitor.lastSeen
		}
	}
	if oldestIP != "" {
		delete(l.visitors, oldestIP)
	}
}

// KeyedRateLimit applies a context-aware limiter. Backend failures are
// fail-closed with 503 so a distributed limit is never silently bypassed.
func KeyedRateLimit(limiter KeyedRateLimiter, keyFor RateLimitKeyFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limiter == nil || keyFor == nil {
			c.Next()
			return
		}

		key, ok := keyFor(c)
		if !ok {
			c.Next()
			return
		}
		allowed, err := limiter.Allow(c.Request.Context(), key)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, response.Error(
				GetRequestID(c),
				apperrors.CodeDependencyError,
				"rate limiter unavailable",
			))
			return
		}
		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, response.Error(
				GetRequestID(c),
				apperrors.CodeRateLimited,
				"rate limit exceeded",
			))
			return
		}
		c.Next()
	}
}
