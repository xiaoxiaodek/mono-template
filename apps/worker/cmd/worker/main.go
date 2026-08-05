package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/vort-ads/vort-ads-template/apps/worker/internal/bootstrap"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.New(configDirectory())
	if err != nil {
		log.Fatal(err)
	}
	if err := app.Run(ctx); err != nil {
		log.Fatal(err)
	}
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
