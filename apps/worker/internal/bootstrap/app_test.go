package bootstrap

import (
	"context"
	"path/filepath"
	"testing"
)

func TestAppRunStopsWhenContextIsCanceled(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("JWT_SECRET", "test-secret-with-enough-length")

	app, err := New(filepath.Join("..", "..", "..", "..", "configs"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := app.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
}
