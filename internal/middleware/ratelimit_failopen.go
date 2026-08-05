package middleware

import "context"

// failOpenLimiter wraps a KeyedRateLimiter so that backend errors are treated
// as allowed rather than blocked. Use it when availability takes priority over
// strict rate enforcement.
type failOpenLimiter struct {
	inner KeyedRateLimiter
}

func (l *failOpenLimiter) Allow(ctx context.Context, key string) (bool, error) {
	allowed, err := l.inner.Allow(ctx, key)
	if err != nil {
		return true, nil
	}
	return allowed, nil
}

// WithFailOpen returns a limiter that allows all requests when the inner
// limiter returns an error. Pass nil inner to get a no-op pass-through.
func WithFailOpen(inner KeyedRateLimiter) KeyedRateLimiter {
	if inner == nil {
		return nil
	}
	return &failOpenLimiter{inner: inner}
}
