package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/vort-ads/vort-ads-template/apps/operation-api/internal/bootstrap"
)

// @title          Vort Ads Operation API
// @version        0.1.0
// @description    Identity and control-plane API for the Vort Ads platform.
// @contact.name   Vort Ads Team
//
// @license.name  Proprietary
//
// @host      localhost:8080
// @BasePath  /api/v1
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and the JWT access token.

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	app, err := bootstrap.New(context.Background(), configDirectory())
	if err != nil {
		return err
	}

	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErrors := make(chan error, 1)
	go func() {
		app.Logger.Info().Str("addr", app.Server.Addr).Msg("control API starting")
		serverErrors <- app.Server.ListenAndServe()
	}()

	select {
	case <-signalContext.Done():
		app.Logger.Info().Msg("shutdown signal received")
	case serveErr := <-serverErrors:
		if !errors.Is(serveErr, http.ErrServerClosed) {
			_ = app.Shutdown(context.Background())
			return fmt.Errorf("serve control API: %w", serveErr)
		}
	}

	if err := app.Shutdown(context.Background()); err != nil {
		return fmt.Errorf("shutdown control API: %w", err)
	}
	return nil
}

func configDirectory() string {
	if configured := os.Getenv("CONFIG_DIR"); configured != "" {
		return configured
	}
	for _, candidate := range []string{"configs", filepath.Join("..", "configs")} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return "configs"
}
