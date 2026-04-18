package config

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

type Config struct {
	Tracking TrackingConfig `toml:"tracking"`
	Display  DisplayConfig  `toml:"display"`
	Filters  FiltersConfig  `toml:"filters"`
	Tee      TeeConfig      `toml:"tee"`
}

type TrackingConfig struct {
	DBPath string `toml:"db_path"`
}

type DisplayConfig struct {
	Color bool `toml:"color"`
	Emoji bool `toml:"emoji"`
}

type FiltersConfig struct {
	Dir string `toml:"dir"`
}

type TeeConfig struct {
	Enabled     bool   `toml:"enabled"`
	Mode        string `toml:"mode"` // "failures", "always", "never"
	MaxFiles    int    `toml:"max_files"`
	MaxFileSize int64  `toml:"max_file_size"`
}

// DefaultConfig returns sensible defaults for scouter.
func DefaultConfig() *Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return &Config{
		Tracking: TrackingConfig{
			DBPath: filepath.Join(home, ".config", "scouter", "scouter.db"),
		},
		Display: DisplayConfig{
			Color: true,
			Emoji: true,
		},
		Filters: FiltersConfig{
			Dir: filepath.Join(home, ".config", "scouter", "filters"),
		},
		Tee: TeeConfig{
			Enabled:     true,
			Mode:        "failures",
			MaxFiles:    20,
			MaxFileSize: 1 << 20, // 1MB
		},
	}
}

// Load reads config from file, merging with defaults. Returns defaults if file missing.
func Load(ctx context.Context) (*Config, error) {
	cfg := DefaultConfig()

	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return cfg, nil
}

// Save writes config back to file.
func (c *Config) Save(ctx context.Context) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write atomicaly using a temporary file.
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write temporary config: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to commit config: %w", err)
	}

	return nil
}

// Migrate moves legacy 'scouter' configuration and database to 'scouter'.
func (c *Config) Migrate(ctx context.Context) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	legacyConfigDir := filepath.Join(home, ".config", "scouter")
	legacyDBPath := filepath.Join(home, ".local", "share", "scouter", "tracking.db")
	newConfigDir := filepath.Join(home, ".config", "scouter")
	newDBPath := filepath.Join(newConfigDir, "scouter.db")

	// 1. Check if migration is needed.
	if _, err := os.Stat(newConfigDir); err == nil {
		// Scouter config directory already exists, skip migration.
		return nil
	}

	if _, err := os.Stat(legacyConfigDir); os.IsNotExist(err) {
		// Legacy config doesn't exist, nothing to migrate.
		return nil
	}

	// 2. Create new config directory.
	if err := os.MkdirAll(newConfigDir, 0700); err != nil {
		return fmt.Errorf("failed to create new config directory: %w", err)
	}

	// 3. Copy config file if it exists.
	legacyConfigFile := filepath.Join(legacyConfigDir, "config.toml")
	if _, err := os.Stat(legacyConfigFile); err == nil {
		if err := copyFile(legacyConfigFile, filepath.Join(newConfigDir, "config.toml")); err != nil {
			return fmt.Errorf("failed to copy config file: %w", err)
		}

		// Reload the config from the copied file to ensure we have legacy settings.
		data, err := os.ReadFile(filepath.Join(newConfigDir, "config.toml"))
		if err == nil {
			_ = toml.Unmarshal(data, c)
		}
	}

	// 4. Update internal paths if they point to legacy locations.
	if c.Tracking.DBPath == filepath.Join(home, ".local", "share", "scouter", "tracking.db") || c.Tracking.DBPath == "" {
		c.Tracking.DBPath = newDBPath
	}
	if c.Filters.Dir == filepath.Join(home, ".config", "scouter", "filters") || c.Filters.Dir == "" {
		c.Filters.Dir = filepath.Join(newConfigDir, "filters")
	}

	// 5. Copy filters directory if it exists.
	legacyFiltersDir := filepath.Join(legacyConfigDir, "filters")
	if _, err := os.Stat(legacyFiltersDir); err == nil {
		if err := copyDir(legacyFiltersDir, c.Filters.Dir); err != nil {
			return fmt.Errorf("failed to copy filters directory: %w", err)
		}
	}

	// 6. Migrate database file.
	if _, err := os.Stat(legacyDBPath); err == nil {
		if err := os.Rename(legacyDBPath, newDBPath); err != nil {
			// Cross-device rename failed? Try copy.
			if err := copyFile(legacyDBPath, newDBPath); err != nil {
				return fmt.Errorf("failed to migrate database: %w", err)
			}
			os.Remove(legacyDBPath)
		}
	}

	// 7. Save the updated config with new paths.
	if err := c.Save(ctx); err != nil {
		return fmt.Errorf("failed to save migrated config: %w", err)
	}

	// 8. Mark legacy as migrated (renaming to avoid confusion).
	os.Rename(legacyConfigDir, legacyConfigDir+".migrated")

	return nil
}

func configPath() string {
	if p := os.Getenv("SCOUTER_CONFIG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "scouter", "config.toml")
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, 0700); err != nil {
		return err
	}

	for _, entry := range entries {
		sourcePath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(sourcePath, destPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(sourcePath, destPath); err != nil {
				return err
			}
		}
	}
	return nil
}
