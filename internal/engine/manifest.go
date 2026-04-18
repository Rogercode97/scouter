package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Rogercode97/scouter/internal/types"
	"golang.org/x/mod/modfile"
)

// ParseGoMod reads a go.mod file and extracts its dependencies.
func ParseGoMod(ctx context.Context, filePath string) ([]types.Dependency, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read go.mod: %w", err)
	}

	f, err := modfile.Parse(filePath, data, nil)
	if err != nil {
		return nil, fmt.Errorf("parse go.mod: %w", err)
	}

	var deps []types.Dependency
	for _, req := range f.Require {
		deps = append(deps, types.Dependency{
			Name:    req.Mod.Path,
			Version: req.Mod.Version,
			Type:    "golang",
			Project: filePath,
			Direct:  !req.Indirect,
		})
	}

	return deps, nil
}

// ParsePackageJSON reads a package.json file and extracts its dependencies.
func ParsePackageJSON(ctx context.Context, filePath string) ([]types.Dependency, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read package.json: %w", err)
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parse package.json: %w", err)
	}

	var deps []types.Dependency
	// Standard dependencies
	for name, version := range pkg.Dependencies {
		deps = append(deps, types.Dependency{
			Name:    name,
			Version: version,
			Type:    "npm",
			Project: filePath,
			Direct:  true,
		})
	}
	// Dev dependencies
	for name, version := range pkg.DevDependencies {
		deps = append(deps, types.Dependency{
			Name:    name,
			Version: version,
			Type:    "npm",
			Project: filePath,
			Direct:  true,
		})
	}

	return deps, nil
}
