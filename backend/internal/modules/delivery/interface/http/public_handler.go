package http

import (
	"crypto/subtle"
	"io"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
)

type PublicHandler struct {
	queue  messaging.Queue
	secret string
}

func NewPublicHandler(queue messaging.Queue, secret string) *PublicHandler {
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("WEBHOOK_SECRET"))
	}
	return &PublicHandler{queue: queue, secret: secret}
}

func (h *PublicHandler) BuildWebhook(c *gin.Context) {
	if h.secret != "" && !h.validSecret(c) {
		httpx.Fail(c, "AUTH_FORBIDDEN", "invalid webhook secret", 403)
		return
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "failed to read body", 400)
		return
	}
	if h.queue == nil {
		httpx.Fail(c, "SERVICE_UNAVAILABLE", "message queue not configured", 503)
		return
	}
	if err := h.queue.Publish(c.Request.Context(), messaging.TopicHook, body); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"accepted": true})
}

func (h *PublicHandler) validSecret(c *gin.Context) bool {
	got := strings.TrimSpace(c.GetHeader("X-Webhook-Secret"))
	if got == "" {
		got = strings.TrimSpace(c.Query("secret"))
	}
	if got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(h.secret)) == 1
}
