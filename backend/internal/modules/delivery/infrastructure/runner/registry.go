package runner

import (
	"context"
	"sync"
)

type Registry struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func NewRegistry() *Registry {
	return &Registry{cancels: make(map[string]context.CancelFunc)}
}

func (r *Registry) Register(buildID string, cancel context.CancelFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if old, ok := r.cancels[buildID]; ok {
		old()
	}
	r.cancels[buildID] = cancel
}

func (r *Registry) Cancel(buildID string) bool {
	r.mu.Lock()
	cancel, ok := r.cancels[buildID]
	if ok {
		delete(r.cancels, buildID)
	}
	r.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

func (r *Registry) Done(buildID string) {
	r.mu.Lock()
	delete(r.cancels, buildID)
	r.mu.Unlock()
}
