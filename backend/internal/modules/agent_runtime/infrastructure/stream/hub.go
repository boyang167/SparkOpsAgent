package stream

import (
	"sync"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Hub struct {
	mu      sync.RWMutex
	chans   map[string][]chan string
	outputs map[string]string
	done    map[string]struct{}
}

func NewHub() *Hub {
	return &Hub{
		chans:   make(map[string][]chan string),
		outputs: make(map[string]string),
		done:    make(map[string]struct{}),
	}
}

func (h *Hub) Publish(runID kernel.RunID, delta string) {
	key := runID.String()
	h.mu.Lock()
	h.outputs[key] += delta
	subs := append([]chan string(nil), h.chans[key]...)
	h.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- delta:
		default:
		}
	}
}

func (h *Hub) Complete(runID kernel.RunID, output string) {
	key := runID.String()
	h.mu.Lock()
	h.outputs[key] = output
	h.done[key] = struct{}{}
	subs := append([]chan string(nil), h.chans[key]...)
	delete(h.chans, key)
	h.mu.Unlock()
	for _, ch := range subs {
		close(ch)
	}
}

func (h *Hub) Subscribe(runID kernel.RunID) (<-chan string, string, bool) {
	key := runID.String()
	h.mu.RLock()
	output := h.outputs[key]
	_, finished := h.done[key]
	h.mu.RUnlock()
	if finished {
		ch := make(chan string)
		close(ch)
		return ch, output, true
	}
	ch := make(chan string, 32)
	h.mu.Lock()
	h.chans[key] = append(h.chans[key], ch)
	h.mu.Unlock()
	return ch, output, false
}

func (h *Hub) Snapshot(runID kernel.RunID) (output string, done bool) {
	key := runID.String()
	h.mu.RLock()
	defer h.mu.RUnlock()
	output = h.outputs[key]
	_, done = h.done[key]
	return output, done
}
