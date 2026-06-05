package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sync"
)

// ShardManager manages multiple SQLite databases (shards) for large codebases.
type ShardManager struct {
	basePath string
	mu       sync.RWMutex
	shards   map[string]Store
}

func NewShardManager(basePath string) *ShardManager {
	return &ShardManager{
		basePath: basePath,
		shards:   make(map[string]Store),
	}
}

// GetShard returns the store responsible for the given file path.
func (m *ShardManager) GetShard(ctx context.Context, path string) (Store, error) {
	// Strategy: Shard by parent directory to keep packages/modules together
	shardKey := m.computeShardKey(path)

	m.mu.RLock()
	s, ok := m.shards[shardKey]
	m.mu.RUnlock()
	if ok {
		return s, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double check
	if s, ok := m.shards[shardKey]; ok {
		return s, nil
	}

	shardPath := filepath.Join(m.basePath, fmt.Sprintf("shard_%s.db", shardKey))
	newShard, err := NewStore(ctx, shardPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create shard %s: %w", shardKey, err)
	}

	m.shards[shardKey] = newShard
	return newShard, nil
}

func (m *ShardManager) computeShardKey(path string) string {
	// For now, let's just use the first 4 characters of the SHA256 of the parent directory.
	// This gives us up to 65536 shards, which is plenty.
	dir := filepath.Dir(path)
	h := sha256.Sum256([]byte(dir))
	return hex.EncodeToString(h[:])[:4]
}

func (m *ShardManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var errs []error
	for _, s := range m.shards {
		if err := s.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing shards: %v", errs)
	}
	return nil
}
