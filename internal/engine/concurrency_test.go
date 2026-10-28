package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStructuralSearch_Cancellation(t *testing.T) {
	// Corpus sizing is load-bearing: the "during execution" subtest cancels
	// mid-flight, so the search must reliably outlast the cancel delay on
	// any hardware. 500 files complete in ~10ms on fast CI runners and won
	// the race against a 10ms cancel (flaky FAIL with "got nil"); 5000 files
	// take two orders of magnitude longer than the 1ms cancel below, and
	// file enumeration plus parsing cannot complete in 1ms anywhere.
	tmpDir, err := os.MkdirTemp("", "scouter-concurrency-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	for i := 0; i < 5000; i++ {
		content := fmt.Sprintf("package main\nfunc Foo%d() {\n\tprintln(\"hello\")\n}\n", i)
		err := os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("file%d.go", i)), []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Run("Immediate cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := StructuralSearch(ctx, tmpDir, "println($X)", ".go")
		if err == nil {
			t.Error("expected error due to cancelled context, got nil")
		} else if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("Cancellation during execution", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		// Run in background and cancel almost immediately: the 5000-file
		// corpus guarantees the search is still in flight.
		go func() {
			time.Sleep(1 * time.Millisecond)
			cancel()
		}()

		_, err := StructuralSearch(ctx, tmpDir, "println($X)", ".go")
		if err == nil {
			t.Error("expected error due to cancelled context, got nil")
		}
	})
}

func BenchmarkStructuralSearch_Sequential(b *testing.B) {
	// Note: Since StructuralSearch is now concurrent, this name is just for comparison
	// if we wanted to revert to sequential.
	tmpDir, err := os.MkdirTemp("", "scouter-bench-seq-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	for i := 0; i < 50; i++ {
		content := fmt.Sprintf("package main\nfunc Foo%d() {\n\tprintln(\"hello\")\n}\n", i)
		os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("file%d.go", i)), []byte(content), 0644)
	}

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = StructuralSearch(ctx, tmpDir, "println($X)", ".go")
	}
}

func BenchmarkStructuralSearch_Concurrent(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "scouter-bench-conc-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	for i := 0; i < 50; i++ {
		content := fmt.Sprintf("package main\nfunc Foo%d() {\n\tprintln(\"hello\")\n}\n", i)
		os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("file%d.go", i)), []byte(content), 0644)
	}

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = StructuralSearch(ctx, tmpDir, "println($X)", ".go")
	}
}
