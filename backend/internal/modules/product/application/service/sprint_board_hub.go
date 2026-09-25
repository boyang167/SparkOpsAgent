package service

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type SprintBoardHub struct {
	mu    sync.RWMutex
	rooms map[string]map[*websocket.Conn]struct{}
}

func NewSprintBoardHub() *SprintBoardHub {
	return &SprintBoardHub{rooms: map[string]map[*websocket.Conn]struct{}{}}
}

func (h *SprintBoardHub) Register(sprintID kernel.SprintID, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	key := sprintID.String()
	if h.rooms[key] == nil {
		h.rooms[key] = map[*websocket.Conn]struct{}{}
	}
	h.rooms[key][conn] = struct{}{}
}

func (h *SprintBoardHub) Unregister(sprintID kernel.SprintID, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	key := sprintID.String()
	room := h.rooms[key]
	if room == nil {
		return
	}
	delete(room, conn)
	if len(room) == 0 {
		delete(h.rooms, key)
	}
}

func (h *SprintBoardHub) Notify(sprintID kernel.SprintID, event string, payload map[string]any) {
	if h == nil {
		return
	}
	msg, err := json.Marshal(map[string]any{"event": event, "payload": payload})
	if err != nil {
		return
	}
	h.mu.RLock()
	room := h.rooms[sprintID.String()]
	conns := make([]*websocket.Conn, 0, len(room))
	for conn := range room {
		conns = append(conns, conn)
	}
	h.mu.RUnlock()
	for _, conn := range conns {
		_ = conn.WriteMessage(websocket.TextMessage, msg)
	}
}

type SprintBoardNotifier struct {
	hub *SprintBoardHub
}

func NewSprintBoardNotifier(hub *SprintBoardHub) *SprintBoardNotifier {
	return &SprintBoardNotifier{hub: hub}
}

func (n *SprintBoardNotifier) BoardChanged(sprintID kernel.SprintID, reason string) {
	if n == nil || n.hub == nil {
		return
	}
	n.hub.Notify(sprintID, "board_changed", map[string]any{"reason": reason})
}
