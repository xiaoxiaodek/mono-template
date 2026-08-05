package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDirectoryHonorsEnvironment(t *testing.T) {
	t.Setenv("CONFIG_DIR", "/opt/vort/configs")
	if got := configDirectory(); got != "/opt/vort/configs" {
		t.Fatalf("configDirectory() = %q, want explicit path", got)
	}
}

func TestConfigDirectoryFindsParentConfigs(t *testing.T) {
	t.Setenv("CONFIG_DIR", "")
	root := t.TempDir()
	appsDir := filepath.Join(root, "apps")
	if err := os.MkdirAll(appsDir, 0o750); err != nil {
		t.Fatalf("create apps dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "configs"), 0o750); err != nil {
		t.Fatalf("create configs dir: %v", err)
	}
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(appsDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	want := filepath.Join("..", "configs")
	if got := configDirectory(); got != want {
		t.Fatalf("configDirectory() = %q, want %q", got, want)
	}
}
