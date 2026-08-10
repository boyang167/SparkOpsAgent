package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/notification/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type Handler struct {
	list        *query.ListNotificationsHandler
	markRead    *command.MarkReadHandler
	markAllRead *command.MarkAllReadHandler
}

func NewHandler(
	list *query.ListNotificationsHandler,
	markRead *command.MarkReadHandler,
	markAllRead *command.MarkAllReadHandler,
) *Handler {
	return &Handler{list: list, markRead: markRead, markAllRead: markAllRead}
}

func memberAndWorkspace(c *gin.Context) (kernel.WorkspaceID, kernel.MemberID, bool) {
	memberID, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	wsID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return kernel.WorkspaceID{}, kernel.MemberID{}, false
	}
	return wsID, memberID, true
}

func (h *Handler) ListNotifications(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	result, err := h.list.Handle(c.Request.Context(), memberID, wsID, 50)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, result)
}

func (h *Handler) MarkRead(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	notificationID, err := kernel.ParseNotificationID(c.Param("notificationId"))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", "invalid notification id", http.StatusBadRequest)
		return
	}
	if err := h.markRead.Handle(c.Request.Context(), memberID, wsID, notificationID); err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"read": true})
}

func (h *Handler) MarkAllRead(c *gin.Context) {
	wsID, memberID, ok := memberAndWorkspace(c)
	if !ok {
		return
	}
	count, err := h.markAllRead.Handle(c.Request.Context(), memberID, wsID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, gin.H{"updated": count})
}
