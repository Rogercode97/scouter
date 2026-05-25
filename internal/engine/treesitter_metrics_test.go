package engine

import (
	"context"
	"testing"
	"path/filepath"
	"os"

	"github.com/Rogercode97/scouter/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestSemanticMetrics_Go(t *testing.T) {
	content := `package test
func ComplexFunc(err error) {
	if err != nil {
		panic(err)
	}
	for i := 0; i < 10; i++ {
		if i == 5 {
			go func() {}()
		}
	}
}`
	tmpFile := filepath.Join(t.TempDir(), "test.go")
	os.WriteFile(tmpFile, []byte(content), 0644)

	pointersIter, _, err := StreamWithTreeSitter(context.Background(), tmpFile)
	assert.NoError(t, err)

	var ptrs []types.ASTPointer
	pointersIter(func(p types.ASTPointer) bool {
		ptrs = append(ptrs, p)
		return true
	})

	assert.Len(t, ptrs, 1)
	complexFunc := ptrs[0]
	assert.Equal(t, "ComplexFunc", complexFunc.Name)
	assert.NotNil(t, complexFunc.Metrics)
	assert.Equal(t, 4, complexFunc.Metrics.CyclomaticComplexity)
	assert.True(t, complexFunc.Metrics.HasErrorHandling)
	assert.True(t, complexFunc.Metrics.HasExceptions)
	assert.True(t, complexFunc.Metrics.IsAsync)
}

func TestSemanticMetrics_Python(t *testing.T) {
	content := `
async def process_data(data):
    try:
        if len(data) > 0 and data[0] == 1:
            pass
    except Exception as e:
        raise e
`
	tmpFile := filepath.Join(t.TempDir(), "test.py")
	os.WriteFile(tmpFile, []byte(content), 0644)

	pointersIter, _, err := StreamWithTreeSitter(context.Background(), tmpFile)
	assert.NoError(t, err)

	var ptrs []types.ASTPointer
	pointersIter(func(p types.ASTPointer) bool {
		ptrs = append(ptrs, p)
		return true
	})

	assert.Len(t, ptrs, 1)
	fn := ptrs[0]
	assert.Equal(t, "process_data", fn.Name)
	assert.NotNil(t, fn.Metrics)
	// Base (1) + if (1) + and (1) + except (1) = 4
	assert.Equal(t, 4, fn.Metrics.CyclomaticComplexity)
	assert.True(t, fn.Metrics.IsAsync)
	assert.True(t, fn.Metrics.HasErrorHandling)
	assert.True(t, fn.Metrics.HasExceptions)
}

func TestSemanticMetrics_TypeScript(t *testing.T) {
	content := `
async function fetchData(url: string) {
    try {
        if (!url) {
            throw new Error("Invalid URL");
        }
        return await fetch(url);
    } catch (e) {
        console.error(e);
    }
}
`
	tmpFile := filepath.Join(t.TempDir(), "test.ts")
	os.WriteFile(tmpFile, []byte(content), 0644)

	pointersIter, _, err := StreamWithTreeSitter(context.Background(), tmpFile)
	assert.NoError(t, err)

	var ptrs []types.ASTPointer
	pointersIter(func(p types.ASTPointer) bool {
		ptrs = append(ptrs, p)
		return true
	})

	assert.Len(t, ptrs, 1)
	fn := ptrs[0]
	assert.Equal(t, "fetchData", fn.Name)
	assert.NotNil(t, fn.Metrics)
	// Base (1) + try/catch (1) + if (1) = 3
	assert.Equal(t, 3, fn.Metrics.CyclomaticComplexity)
	assert.True(t, fn.Metrics.IsAsync)
	assert.True(t, fn.Metrics.HasErrorHandling)
	assert.True(t, fn.Metrics.HasExceptions)
}
