package bootstrap

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/vort-ads/vort-ads-template/apps/internal/platform/config"
	platformlogger "github.com/vort-ads/vort-ads-template/apps/internal/platform/logger"
)

// App is the worker process shell. Business jobs are intentionally added by
// future modules rather than being hidden in bootstrap code.
type App struct {
	config config.Config
	log    zerolog.Logger
}

func New(configDir string) (*App, error) {
	cfg, err := config.Load(configDir)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return &App{config: cfg, log: platformlogger.New(cfg.App.Env)}, nil
}

// Run waits for the process lifecycle context. It starts no unbounded
// goroutines and contains no placeholder business jobs.
func (a *App) Run(ctx context.Context) error {
	a.log.Info().Str("app", a.config.App.Name).Msg("worker started")
	<-ctx.Done()
	a.log.Info().Msg("worker stopped")
	return nil
}
