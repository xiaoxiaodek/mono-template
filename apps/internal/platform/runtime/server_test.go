package runtime

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/vort-ads/vort-ads-template/apps/internal/platform/config"
)

func TestNewHTTPServerAppliesConfiguredTimeouts(t *testing.T) {
	cfg := config.HTTPConfig{
		Addr:         ":9090",
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 3 * time.Second,
		IdleTimeout:  4 * time.Second,
	}
	server := NewHTTPServer(cfg, http.NotFoundHandler())

	if server.Addr != cfg.Addr || server.ReadTimeout != cfg.ReadTimeout ||
		server.ReadHeaderTimeout != cfg.ReadTimeout || server.WriteTimeout != cfg.WriteTimeout ||
		server.IdleTimeout != cfg.IdleTimeout {
		t.Fatalf("server timeouts were not applied: %+v", server)
	}
}

func TestShutdownAcceptsAnUnstartedServer(t *testing.T) {
	server := &http.Server{} // #nosec G112 -- no listener is started in this shutdown unit test.
	if err := Shutdown(context.Background(), server, 50*time.Millisecond); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}
