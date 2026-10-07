package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

const (
	releasePlanCollabReadTimeout = 70 * time.Second
	releasePlanCollabPingPeriod  = 25 * time.Second
)

type ReleasePlanCollabWSHandler struct {
	hub *service.ReleasePlanCollabHub
}

func NewReleasePlanCollabWSHandler(hub *service.ReleasePlanCollabHub) *ReleasePlanCollabWSHandler {
	return &ReleasePlanCollabWSHandler{hub: hub}
}

func (h *Handler) ReleasePlanCollabWebSocket(c *gin.Context) {
	if h.releasePlanCollabWS == nil || h.releasePlanCollabWS.hub == nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	_, planID, ok := parseProductReleasePlanIDs(c)
	if !ok {
		return
	}
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	session := h.releasePlanCollabWS.hub.Register(planID, conn, memberID)
	defer h.releasePlanCollabWS.hub.Unregister(planID, session)
	h.releasePlanCollabWS.hub.BroadcastEditors(planID)
	_ = conn.SetReadDeadline(time.Now().Add(releasePlanCollabReadTimeout))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(releasePlanCollabReadTimeout))
	})
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(releasePlanCollabPingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second)); err != nil {
					close(done)
					return
				}
			case <-done:
				return
			}
		}
	}()
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			close(done)
			break
		}
		var msg struct {
			Action  string `json:"action"`
			Section string `json:"section"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		switch msg.Action {
		case "join", "heartbeat":
			h.releasePlanCollabWS.hub.SetSection(planID, session, msg.Section)
		case "leave":
			h.releasePlanCollabWS.hub.SetSection(planID, session, "")
		}
	}
	h.releasePlanCollabWS.hub.BroadcastEditors(planID)
}

func (h *Handler) ListReleasePlanCollabEditors(c *gin.Context) {
	if h.releasePlanCollabWS == nil || h.releasePlanCollabWS.hub == nil {
		httpx.OK(c, gin.H{"editors": []any{}})
		return
	}
	_, planID, ok := parseProductReleasePlanIDs(c)
	if !ok {
		return
	}
	httpx.OK(c, gin.H{"editors": h.releasePlanCollabWS.hub.Snapshot(planID)})
}
