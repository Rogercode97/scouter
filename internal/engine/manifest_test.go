package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParseGoMod(t *testing.T) {
	content := `module github.com/test/project

go 1.25

require (
	github.com/google/uuid v1.3.0
	github.com/stretchr/testify v1.8.4 // indirect
)
`
	tmpDir, err := os.MkdirTemp("", "scouter-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	deps, err := ParseGoMod(context.Background(), goModPath)
	if err != nil {
		t.Fatalf("ParseGoMod failed: %v", err)
	}

	if len(deps) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(deps))
	}

	foundDirect := false
	foundIndirect := false
	for _, d := range deps {
		if d.Name == "github.com/google/uuid" {
			foundDirect = true
			if !d.Direct {
				t.Error("github.com/google/uuid should be direct")
			}
		}
		if d.Name == "github.com/stretchr/testify" {
			foundIndirect = true
			if d.Direct {
				t.Error("github.com/stretchr/testify should be indirect")
			}
		}
	}

	if !foundDirect || !foundIndirect {
		t.Error("Did not find both expected dependencies")
	}
}

func TestParsePackageJSON(t *testing.T) {
	content := `{
  "name": "test-project",
  "dependencies": {
    "lodash": "^4.17.21"
  },
  "devDependencies": {
    "typescript": "^5.0.0"
  }
}
`
	tmpDir, err := os.MkdirTemp("", "scouter-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	pkgJSONPath := filepath.Join(tmpDir, "package.json")
	if err := os.WriteFile(pkgJSONPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	deps, err := ParsePackageJSON(context.Background(), pkgJSONPath)
	if err != nil {
		t.Fatalf("ParsePackageJSON failed: %v", err)
	}

	if len(deps) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(deps))
	}

	foundLodash := false
	foundTS := false
	for _, d := range deps {
		if d.Name == "lodash" {
			foundLodash = true
		}
		if d.Name == "typescript" {
			foundTS = true
		}
		if !d.Direct {
			t.Errorf("NPM dependency %s should be marked as direct", d.Name)
		}
	}

	if !foundLodash || !foundTS {
		t.Error("Did not find both expected NPM dependencies")
	}
}
