package tee

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Rogercode97/scouter/internal/utils"
)

// Config for tee behavior.
type Config struct {
	Enabled     bool
	Mode        string // "failures", "always", "never"
	MaxFiles    int
	MaxFileSize int64
	Dir         string
}

// DefaultConfig returns tee defaults.
func DefaultConfig() Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return Config{
		Enabled:     true,
		Mode:        "failures",
		MaxFiles:    50,                // RTK-inspired higher limit for unique commands
		MaxFileSize: 2 * 1024 * 1024,   // 2MB for deeper context
		Dir:         filepath.Join(home, ".local", "share", "scouter", "tee"),
	}
}

// MaybeSave saves raw output if conditions are met. Returns hint string if saved.
func MaybeSave(raw string, exitCode int, cmd string, cfg Config) string {
	if !cfg.Enabled || cfg.Mode == "never" {
		return ""
	}

	// Check SCOUTER_TEE env override
	if os.Getenv("SCOUTER_TEE") == "0" {
		return ""
	}

	shouldSave := cfg.Mode == "always" || (cfg.Mode == "failures" && exitCode != 0)
	if !shouldSave {
		return ""
	}

	// Skip if output is too small to be meaningful context
	if len(raw) < 200 {
		return ""
	}

	dir := cfg.Dir
	if envDir := os.Getenv("SCOUTER_TEE_DIR"); envDir != "" {
		dir = envDir
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "" // Silent failure
	}

	// CAS: Filename is the hash of the command string
	hash := utils.HashString(cmd)
	filename := fmt.Sprintf("%s.log", hash)
	path := filepath.Join(dir, filename)

	// Truncate if too large (rune-safe)
	data := raw
	if int64(len(data)) > cfg.MaxFileSize {
		runes := []rune(data)
		byteCount := 0
		for i, r := range runes {
			byteCount += len(string(r))
			if int64(byteCount) > cfg.MaxFileSize {
				data = string(runes[:i]) + "\n[scouter: output truncated due to MaxFileSize limit]"
				break
			}
		}
	}

	// Write latest trace for this specific command hash
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		return "" // Silent failure
	}

	// Rotate by access/modification time if we reach MaxFiles
	rotateFiles(dir, cfg.MaxFiles)

	return fmt.Sprintf("[full output: %s]", path)
}

func rotateFiles(dir string, maxFiles int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var logFiles []os.FileInfo
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
			if info, err := e.Info(); err == nil {
				logFiles = append(logFiles, info)
			}
		}
	}

	if len(logFiles) <= maxFiles {
		return
	}

	// Sort by modification time (oldest first)
	sort.Slice(logFiles, func(i, j int) bool {
		return logFiles[i].ModTime().Before(logFiles[j].ModTime())
	})

	// Remove oldest unique command traces
	toRemove := len(logFiles) - maxFiles
	for i := 0; i < toRemove; i++ {
		_ = os.Remove(filepath.Join(dir, logFiles[i].Name()))
	}
}
