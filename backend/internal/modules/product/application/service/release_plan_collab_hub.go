package service

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ReleasePlanCollabEditor struct {
	MemberID string `json:"member_id"`
	Section  string `json:"section"`
}

type releasePlanCollabConn struct {
	conn     *websocket.Conn
	memberID string
	section  string
}

type ReleasePlanCollabHub struct {
	mu    sync.RWMutex
	rooms map[string]map[*releasePlanCollabConn]struct{}
}

func NewReleasePlanCollabHub() *ReleasePlanCollabHub {
	return &ReleasePlanCollabHub{rooms: map[string]map[*releasePlanCollabConn]struct{}{}}
}

func (h *ReleasePlanCollabHub) Register(planID kernel.ReleasePlanID, conn *websocket.Conn, memberID kernel.MemberID) *releasePlanCollabConn {
	h.mu.Lock()
	defer h.mu.Unlock()
	key := planID.String()
	if h.rooms[key] == nil {
		h.rooms[key] = map[*releasePlanCollabConn]struct{}{}
	}
	session := &releasePlanCollabConn{conn: conn, memberID: memberID.String(), section: "plan"}
	h.rooms[key][session] = struct{}{}
	return session
}

func (h *ReleasePlanCollabHub) Unregister(planID kernel.ReleasePlanID, session *releasePlanCollabConn) {
	if session == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	key := planID.String()
	room := h.rooms[key]
	if room == nil {
		return
	}
	delete(room, session)
	if len(room) == 0 {
		delete(h.rooms, key)
	}
}

func (h *ReleasePlanCollabHub) SetSection(planID kernel.ReleasePlanID, session *releasePlanCollabConn, section string) {
	if session == nil {
		return
	}
	h.mu.Lock()
	session.section = strings.TrimSpace(section)
	h.mu.Unlock()
	h.BroadcastEditors(planID)
}

func (h *ReleasePlanCollabHub) Snapshot(planID kernel.ReleasePlanID) []ReleasePlanCollabEditor {
	h.mu.RLock()
	defer h.mu.RUnlock()
	room := h.rooms[planID.String()]
	out := make([]ReleasePlanCollabEditor, 0, len(room))
	for session := range room {
		out = append(out, ReleasePlanCollabEditor{
			MemberID: session.memberID,
			Section:  session.section,
		})
	}
	return out
}

func (h *ReleasePlanCollabHub) BroadcastEditors(planID kernel.ReleasePlanID) {
	if h == nil {
		return
	}
	editors := h.Snapshot(planID)
	msg, err := json.Marshal(map[string]any{"event": "editors", "payload": map[string]any{"editors": editors}})
	if err != nil {
		return
	}
	h.mu.RLock()
	room := h.rooms[planID.String()]
	conns := make([]*releasePlanCollabConn, 0, len(room))
	for session := range room {
		conns = append(conns, session)
	}
	h.mu.RUnlock()
	for _, session := range conns {
		_ = session.conn.WriteMessage(websocket.TextMessage, msg)
	}
}
