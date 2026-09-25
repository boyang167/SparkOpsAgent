package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/application/service"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

const (
	sprintBoardReadTimeout = 70 * time.Second
	sprintBoardPingPeriod  = 25 * time.Second
)

type SprintBoardWSHandler struct {
	hub *service.SprintBoardHub
}

func NewSprintBoardWSHandler(hub *service.SprintBoardHub) *SprintBoardWSHandler {
	return &SprintBoardWSHandler{hub: hub}
}

func (h *Handler) SprintBoardWebSocket(c *gin.Context) {
	if h.sprintBoardWS == nil || h.sprintBoardWS.hub == nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	if _, ok := authn.MemberIDFromContext(c); !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	productID, sprintID, ok := parseProductSprintIDs(c)
	if !ok {
		return
	}
	_ = productID
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	h.sprintBoardWS.hub.Register(sprintID, conn)
	defer h.sprintBoardWS.hub.Unregister(sprintID, conn)
	_ = conn.SetReadDeadline(time.Now().Add(sprintBoardReadTimeout))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(sprintBoardReadTimeout))
	})
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(sprintBoardPingPeriod)
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
		if _, _, err := conn.ReadMessage(); err != nil {
			close(done)
			break
		}
	}
}
