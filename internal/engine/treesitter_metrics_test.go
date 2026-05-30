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

	assert.GreaterOrEqual(t, len(ptrs), 1)
	complexFunc := ptrs[0]
	assert.Equal(t, "ComplexFunc", complexFunc.Name)
	assert.NotNil(t, complexFunc.Metrics)
	assert.Equal(t, 4, complexFunc.Metrics.CyclomaticComplexity)
	assert.Equal(t, 4, complexFunc.Metrics.CognitiveComplexity)
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

	assert.GreaterOrEqual(t, len(ptrs), 1)
	fn := ptrs[0]
	assert.Equal(t, "process_data", fn.Name)
	assert.NotNil(t, fn.Metrics)
	// Base (1) + if (1) + and (1) + except (1) = 4
	assert.Equal(t, 4, fn.Metrics.CyclomaticComplexity)
	assert.Equal(t, 3, fn.Metrics.CognitiveComplexity) // if (1) + and (1) + except (1) = 3
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

	assert.GreaterOrEqual(t, len(ptrs), 1)
	fn := ptrs[0]
	assert.Equal(t, "fetchData", fn.Name)
	assert.NotNil(t, fn.Metrics)
	// Base (1) + try/catch (1) + if (1) = 3
	assert.Equal(t, 3, fn.Metrics.CyclomaticComplexity)
	assert.Equal(t, 2, fn.Metrics.CognitiveComplexity) // if (1) + catch (1) = 2
	assert.True(t, fn.Metrics.IsAsync)
	assert.True(t, fn.Metrics.HasErrorHandling)
	assert.True(t, fn.Metrics.HasExceptions)
}

func TestCognitiveComplexity_Nesting(t *testing.T) {
	content := `package test
func NestedFunc(score int) string {
	if score >= 60 { // +1 (nesting: 0)
		if score >= 70 { // +2 (1+(1*1)) (nesting: 1)
			if score >= 80 { // +5 (1+(2*2)) (nesting: 2)
				if score >= 90 { // +10 (1+(3*3)) (nesting: 3)
					return "A"
				}
				return "B"
			}
			return "C"
		}
		return "D"
	}
	return "F"
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

	assert.GreaterOrEqual(t, len(ptrs), 1)
	fn := ptrs[0]
	// 1 + 2 + 5 + 10 = 18
	assert.Equal(t, 18, fn.Metrics.CognitiveComplexity) 
}

func TestCognitiveComplexity_Goto(t *testing.T) {
	content := `package test
func GotoFunc(n int) {
	if n < 0 {
		goto end
	}
	for i := 0; i < n; i++ {
		if i == 5 {
			goto skip
		}
	skip:
		continue
	}
end:
	return
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

	assert.GreaterOrEqual(t, len(ptrs), 1)
	fn := ptrs[0]
	// if (1) + goto (1) + for (1) + if (2) + goto (3) + labeled_statement skip: (3) + labeled_statement end: (1) = 12
	// wait, nesting logic for goto:
	// if n < 0 (nest 0) = 1
	//   goto end (nest 1) = 1+1=2
	// for i := 0 (nest 0) = 1
	//   if i == 5 (nest 1) = 1+1=2
	//     goto skip (nest 2) = 1+2=3
	//   skip: (nest 1) = 1+1=2
	// end: (nest 0) = 1+0=1
	// Total: 1+2+1+2+3+2+1 = 12
	assert.Equal(t, 12, fn.Metrics.CognitiveComplexity)
}

func TestCognitiveComplexity_Closure(t *testing.T) {
	content := `package test
func ClosureFunc() {
	op := func() {
		if true {
			// nesting 1 inside closure because closure increments nesting
			println("test")
		}
	}
	op()
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

	assert.GreaterOrEqual(t, len(ptrs), 1)
	fn := ptrs[0]
	// The func_literal increments nesting. 
	// Inside func, if true -> base 1 + nesting(1) = 2.
	// Total should be 2.
	assert.Equal(t, 2, fn.Metrics.CognitiveComplexity)
}
