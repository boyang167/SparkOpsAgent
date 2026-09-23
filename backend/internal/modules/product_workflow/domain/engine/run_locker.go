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

func (l *runLocker) tryLock(runID kernel.ProductWorkflowRunID) (unlock func(), ok bool) {
	value, _ := l.locks.LoadOrStore(runID.String(), &sync.Mutex{})
	mu := value.(*sync.Mutex)
	if !mu.TryLock() {
		return func() {}, false
	}
	return func() { mu.Unlock() }, true
}
