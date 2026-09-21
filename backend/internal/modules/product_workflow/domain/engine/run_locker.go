package engine

import (
	"sync"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type runLocker struct {
	locks sync.Map
}

func (l *runLocker) lock(runID kernel.ProductWorkflowRunID) func() {
	value, _ := l.locks.LoadOrStore(runID.String(), &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	return func() { mu.Unlock() }
}
