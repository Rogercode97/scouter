package engine

import (
	"context"
	"testing"
	"testing/synctest"

	"github.com/Rogercode97/scouter/internal/engine/lsp"
	"github.com/Rogercode97/scouter/internal/store"
)

func TestNewTruthEngine(t *testing.T) {
	var s store.Repository
	l := lsp.NewManager()

	engine := NewTruthEngine(s, nil, l, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	if engine == nil {
		t.Fatal("expected NewTruthEngine to return a non-nil engine")
	}

	if engine.store != s {
		t.Errorf("expected engine.store to be %v, got %v", s, engine.store)
	}

	if engine.lspMgr != l {
		t.Errorf("expected engine.lspMgr to be %v, got %v", l, engine.lspMgr)
	}
}

func TestIndex(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		engine := NewTruthEngine(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		err := engine.Index(context.Background(), "test.go")
		if err == nil {
			t.Error("expected error for nil store, but got nil")
		}
	})
}
