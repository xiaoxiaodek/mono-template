package runtime

import (
	"context"
	"net/http"
	"time"

	"github.com/vort-ads/vort-ads-template/apps/internal/platform/config"
)

// NewHTTPServer applies the configured network timeouts to an HTTP server.
func NewHTTPServer(cfg config.HTTPConfig, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
}

// Shutdown drains in-flight HTTP requests for no longer than timeout.
func Shutdown(ctx context.Context, server *http.Server, timeout time.Duration) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
