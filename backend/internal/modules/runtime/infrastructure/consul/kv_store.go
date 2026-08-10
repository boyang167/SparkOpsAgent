package consul

import (
	"context"
	"time"
)

type KVEntry struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// KVStore abstracts Consul KV or DB-backed mock storage.
type KVStore interface {
	List(ctx context.Context, workspaceID, prefix string) ([]KVEntry, error)
	Put(ctx context.Context, workspaceID, key, value string) error
	Delete(ctx context.Context, workspaceID, key string) error
}
