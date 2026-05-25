package engine_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Rogercode97/scouter/internal/engine"
	"github.com/Rogercode97/scouter/internal/store"
	"github.com/stretchr/testify/assert"
)

func TestTruthEngine_HybridIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_hybrid.db")

	dbStore, err := store.New(context.Background(), dbPath)
	assert.NoError(t, err)

	eng := engine.NewTruthEngine(dbStore, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// 1. Python script with an import
	pySrc := `
import os
from my_rust_lib import my_function
`
	pyFile := filepath.Join(tmpDir, "main.py")
	err = os.WriteFile(pyFile, []byte(pySrc), 0644)
	assert.NoError(t, err)

	// 2. Rust file with a pyfunction
	rsSrc := `
use pyo3::prelude::*;

#[pyfunction]
fn my_function() -> PyResult<String> {
    Ok("hello".to_string())
}
`
	rsFile := filepath.Join(tmpDir, "lib.rs")
	err = os.WriteFile(rsFile, []byte(rsSrc), 0644)
	assert.NoError(t, err)

	// Index both files
	err = eng.Index(context.Background(), pyFile)
	assert.NoError(t, err)

	err = eng.Index(context.Background(), rsFile)
	assert.NoError(t, err)

	// Fetch all calls to see if FFI and imports are captured
	ctx := context.Background()
	itCalls := dbStore.GetAllCalls(ctx)

	hasOsImport := false
	hasMyRustLibImport := false
	hasMyFunctionFFI := false

	for c, err := range itCalls {
		assert.NoError(t, err)
		if c.LinkType == "import" {
			if c.CalleeName == "os" {
				hasOsImport = true
			}
			if c.CalleeName == "my_rust_lib" {
				hasMyRustLibImport = true
			}
		}
		if c.LinkType == "ffi" {
			if c.CallerName == "my_function" && c.CalleeName == "my_function_ffi" {
				hasMyFunctionFFI = true
			}
		}
	}

	assert.True(t, hasOsImport, "Should capture 'import os'")
	assert.True(t, hasMyRustLibImport, "Should capture 'from my_rust_lib import my_function' as 'my_rust_lib'")
	assert.True(t, hasMyFunctionFFI, "Should capture #[pyfunction] my_function")
}
