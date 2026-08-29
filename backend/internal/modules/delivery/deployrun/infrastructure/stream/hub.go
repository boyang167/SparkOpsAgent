package stream

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Hub struct {
	mu             sync.RWMutex
	chans          map[string][]chan string
	outputs        map[string]string
	completed      map[string]string
	completedOrder []string
}

const (
	subscriberBufferSize = 32
	completedRunLimit    = 256
)

func NewHub() *Hub {
	return &Hub{
		chans: make(map[string][]chan string), outputs: make(map[string]string),
		completed: make(map[string]string),
	}
}

func (h *Hub) Publish(runID kernel.DeploymentRunID, delta string) {
	key := runID.String()
	h.mu.Lock()
	h.outputs[key] += delta
	subscribers := h.chans[key]
	remaining := subscribers[:0]
	for _, ch := range h.chans[key] {
		select {
		case ch <- delta:
			remaining = append(remaining, ch)
		default:
			close(ch)
		}
	}
	if len(remaining) == 0 {
		delete(h.chans, key)
	} else {
		h.chans[key] = remaining
	}
	h.mu.Unlock()
}

type deploymentEventPayload struct {
	EventID uint64 `json:"event_id"`
	At      string `json:"at"`
	Status  string `json:"status"`
	Stage   string `json:"stage,omitempty"`
	Message string `json:"message"`
}

func EncodeEvent(event model.DeploymentRunEvent) string {
	raw, _ := json.Marshal(deploymentEventPayload{
		EventID: event.ID,
		At:      event.At.UTC().Format("2006-01-02T15:04:05.000000000Z07:00"),
		Status:  string(event.Status),
		Stage:   string(event.Stage),
		Message: event.Message,
	})
	return string(raw) + "\n"
}

func EncodeEvents(events []model.DeploymentRunEvent) string {
	var output strings.Builder
	for _, event := range events {
		output.WriteString(EncodeEvent(event))
	}
	return output.String()
}

func EventID(delta string) uint64 {
	var payload deploymentEventPayload
	if json.Unmarshal([]byte(strings.TrimSpace(delta)), &payload) != nil {
		return 0
	}
	return payload.EventID
}

func (h *Hub) PublishEvent(run *model.DeploymentRun, event model.DeploymentRunEvent) {
	delta := EncodeEvent(event)
	h.Publish(run.ID, delta)
	if run.Status == model.StatusSucceeded || run.Status == model.StatusFailed || run.Status == model.StatusAborted {
		h.Complete(run.ID, delta)
	}
}

func (h *Hub) Complete(runID kernel.DeploymentRunID, output string) {
	key := runID.String()
	h.mu.Lock()
	completedOutput := h.outputs[key]
	if completedOutput == "" {
		completedOutput = output
	}
	h.rememberCompleted(key, completedOutput)
	for _, ch := range h.chans[key] {
		close(ch)
	}
	delete(h.chans, key)
	delete(h.outputs, key)
	h.mu.Unlock()
}

func (h *Hub) Subscribe(runID kernel.DeploymentRunID) (<-chan string, string, bool, func()) {
	key := runID.String()
	h.mu.Lock()
	if output, ok := h.completed[key]; ok {
		delete(h.completed, key)
		h.removeCompletedOrder(key)
		ch := make(chan string)
		close(ch)
		h.mu.Unlock()
		return ch, output, true, func() {}
	}
	output := h.outputs[key]
	ch := make(chan string, subscriberBufferSize)
	h.chans[key] = append(h.chans[key], ch)
	h.mu.Unlock()

	unsubscribe := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		subscribers := h.chans[key]
		for index, subscriber := range subscribers {
			if subscriber != ch {
				continue
			}
			h.chans[key] = append(subscribers[:index], subscribers[index+1:]...)
			if len(h.chans[key]) == 0 {
				delete(h.chans, key)
			}
			close(ch)
			return
		}
	}
	return ch, output, false, unsubscribe
}

func (h *Hub) rememberCompleted(key, output string) {
	if _, exists := h.completed[key]; !exists {
		h.completedOrder = append(h.completedOrder, key)
	}
	h.completed[key] = output
	for len(h.completedOrder) > completedRunLimit {
		oldest := h.completedOrder[0]
		h.completedOrder = h.completedOrder[1:]
		delete(h.completed, oldest)
	}
}

func (h *Hub) removeCompletedOrder(key string) {
	for index, completedKey := range h.completedOrder {
		if completedKey == key {
			h.completedOrder = append(h.completedOrder[:index], h.completedOrder[index+1:]...)
			return
		}
	}
}
