package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Tee.Mode != "failures" {
		t.Errorf("expected tee mode 'failures', got %q", cfg.Tee.Mode)
	}
	if cfg.Tee.MaxFiles != 20 {
		t.Errorf("expected max_files 20, got %d", cfg.Tee.MaxFiles)
	}
	if !cfg.Display.Color {
		t.Error("expected color enabled by default")
	}
	if cfg.Tracking.DBPath == "" {
		t.Error("expected non-empty db path")
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Setenv("SCOUTER_CONFIG", "/tmp/nonexistent-scouter-config-test.toml")

	cfg, err := Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Tee.Mode != "failures" {
		t.Errorf("expected defaults when file missing, got tee.mode=%q", cfg.Tee.Mode)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[tracking]
db_path = "/custom/path.db"

[tee]
mode = "always"
max_files = 5
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SCOUTER_CONFIG", path)

	cfg, err := Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Tracking.DBPath != "/custom/path.db" {
		t.Errorf("expected custom db_path, got %q", cfg.Tracking.DBPath)
	}
	if cfg.Tee.Mode != "always" {
		t.Errorf("expected custom tee.mode, got %q", cfg.Tee.Mode)
	}
	if cfg.Tee.MaxFiles != 5 {
		t.Errorf("expected custom tee.max_files, got %d", cfg.Tee.MaxFiles)
	}
}

func TestMigrate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	legacyConfigDir := filepath.Join(home, ".config", "snip")
	legacyDBPath := filepath.Join(home, ".local", "share", "snip", "tracking.db")

	err := os.MkdirAll(legacyConfigDir, 0700)
	if err != nil {
		t.Fatal(err)
	}
	err = os.MkdirAll(filepath.Dir(legacyDBPath), 0700)
	if err != nil {
		t.Fatal(err)
	}

	configContent := `[display]
color = false
emoji = false
`
	err = os.WriteFile(filepath.Join(legacyConfigDir, "config.toml"), []byte(configContent), 0600)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(legacyDBPath, []byte("fake db content"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &Config{}
	err = cfg.Migrate(context.Background())
	if err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	newConfigDir := filepath.Join(home, ".config", "scouter")
	if _, err := os.Stat(newConfigDir); err != nil {
		t.Errorf("new config dir not created: %v", err)
	}

	if _, err := os.Stat(filepath.Join(newConfigDir, "scouter.db")); err != nil {
		t.Errorf("database not migrated: %v", err)
	}

	if cfg.Display.Color != false {
		t.Error("config values not loaded from legacy config")
	}

	if _, err := os.Stat(legacyConfigDir + ".migrated"); err != nil {
		t.Error("legacy config dir not renamed to .migrated")
	}
}
