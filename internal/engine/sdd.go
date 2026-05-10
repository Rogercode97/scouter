package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SDDRoadmap represents the project trajectory and current phase.
type SDDRoadmap struct {
	Phase         string   `yaml:"phase" json:"phase"`
	Trajectory    []string `yaml:"trajectory" json:"trajectory"`
	ActiveChanges []string `yaml:"active_changes" json:"active_changes"`
}

// SDDTask represents a single task in the SDD process.
type SDDTask struct {
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
}

// SpecResult represents a search result for a specification.
type SpecResult struct {
	Path    string `json:"path"`
	Content string   `json:"content"`
}

// SDDEngine handles parsing and searching of SDD artifacts.
type SDDEngine struct {
	rootDir string
}

// NewSDDEngine creates a new instance of SDDEngine.
func NewSDDEngine(rootDir string) *SDDEngine {
	return &SDDEngine{rootDir: rootDir}
}

// ParseRoadmap reads and parses the openspec/state.yaml file.
func (e *SDDEngine) ParseRoadmap(ctx context.Context) (*SDDRoadmap, error) {
	path := filepath.Join(e.rootDir, "openspec", "state.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read roadmap: %w", err)
	}

	var roadmap SDDRoadmap
	if err := yaml.Unmarshal(data, &roadmap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal roadmap: %w", err)
	}

	return &roadmap, nil
}

// ParseTasks reads and parses tasks from openspec/tasks.md.
func (e *SDDEngine) ParseTasks(ctx context.Context) ([]SDDTask, error) {
	path := filepath.Join(e.rootDir, "openspec", "tasks.md")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	var tasks []SDDTask
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- [") {
			completed := strings.HasPrefix(line, "- [x]") || strings.HasPrefix(line, "- [X]")
			title := ""
			if len(line) > 6 {
				title = strings.TrimSpace(line[6:])
			}
			tasks = append(tasks, SDDTask{
				Title:     title,
				Completed: completed,
			})
		}
	}

	return tasks, nil
}

// SearchSpecs searches for specifications in openspec/specs/ matching the query.
func (e *SDDEngine) SearchSpecs(ctx context.Context, query string, limit, offset int) ([]SpecResult, error) {
	specsDir := filepath.Join(e.rootDir, "openspec", "specs")
	var results []SpecResult

	// Check if specs directory exists
	if _, err := os.Stat(specsDir); os.IsNotExist(err) {
		return []SpecResult{}, nil
	}

	err := filepath.Walk(specsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		if filepath.Ext(path) == ".md" {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			content := string(data)
			if query == "" || strings.Contains(strings.ToLower(content), strings.ToLower(query)) || strings.Contains(strings.ToLower(path), strings.ToLower(query)) {
				results = append(results, SpecResult{
					Path:    path,
					Content: content,
				})
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Apply offset and limit
	if offset >= len(results) {
		return []SpecResult{}, nil
	}
	end := offset + limit
	if end > len(results) || limit <= 0 {
		end = len(results)
	}

	return results[offset:end], nil
}
