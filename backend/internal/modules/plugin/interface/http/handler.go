package http

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

const (
	MaxUploadBytes  int64 = 50 << 20
	MaxRequestBytes int64 = 51 << 20
	multipartMemory int64 = 32 << 20
)

type installCommand interface {
	Handle(context.Context, []byte, kernel.MemberID) (*model.PluginRecord, error)
}

type listQuery interface {
	Handle(context.Context) ([]*model.PluginRecord, error)
}

type getQuery interface {
	Handle(context.Context, string) (*model.PluginRecord, error)
}

type statusCommand interface {
	Handle(context.Context, string) (*model.PluginRecord, error)
}

type uninstallCommand interface {
	Handle(context.Context, string, kernel.MemberID) (*model.PluginRecord, error)
}

type workspaceListQuery interface {
	Handle(context.Context, kernel.WorkspaceID) ([]query.WorkspacePlugin, error)
}

type workspaceCommand interface {
	Handle(context.Context, kernel.WorkspaceID, string, kernel.MemberID) (*model.WorkspaceBinding, error)
}

type Handler struct {
	install       installCommand
	list          listQuery
	get           getQuery
	markMerged    statusCommand
	markDeployed  statusCommand
	uninstall     uninstallCommand
	workspaceList workspaceListQuery
	enable        workspaceCommand
	disable       workspaceCommand
}

func NewHandler(
	install installCommand,
	list listQuery,
	get getQuery,
	markMerged statusCommand,
	markDeployed statusCommand,
	uninstall uninstallCommand,
) *Handler {
	return &Handler{
		install:      install,
		list:         list,
		get:          get,
		markMerged:   markMerged,
		markDeployed: markDeployed,
		uninstall:    uninstall,
	}
}

func NewWorkspaceHandler(list workspaceListQuery, enable, disable workspaceCommand) *Handler {
	return &Handler{workspaceList: list, enable: enable, disable: disable}
}

func (h *Handler) Install(c *gin.Context) {
	actor, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		httpx.Fail(c, "INVALID_REQUEST", "multipart/form-data is required", http.StatusBadRequest)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxRequestBytes)
	parseErr := c.Request.ParseMultipartForm(multipartMemory)
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	if parseErr != nil {
		status := http.StatusBadRequest
		var maxBytesErr *http.MaxBytesError
		if errors.As(parseErr, &maxBytesErr) {
			status = http.StatusRequestEntityTooLarge
		}
		httpx.Fail(c, "INVALID_REQUEST", parseErr.Error(), status)
		return
	}
	form := c.Request.MultipartForm
	if form == nil || len(form.Value) != 0 || len(form.File) != 1 || len(form.File["file"]) != 1 {
		httpx.Fail(c, "INVALID_REQUEST", "multipart body must contain only one file field named file", http.StatusBadRequest)
		return
	}
	fileHeader := form.File["file"][0]
	if fileHeader.Size > MaxUploadBytes {
		httpx.Fail(c, "INVALID_REQUEST", "plugin archive exceeds 50 MiB", http.StatusRequestEntityTooLarge)
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, MaxUploadBytes+1))
	if err != nil {
		httpx.Fail(c, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}
	if int64(len(raw)) > MaxUploadBytes {
		httpx.Fail(c, "INVALID_REQUEST", "plugin archive exceeds 50 MiB", http.StatusRequestEntityTooLarge)
		return
	}
	record, err := h.install.Handle(c.Request.Context(), raw, actor)
	if err != nil {
		handlePluginError(c, err)
		return
	}
	httpx.OK(c, record)
}

func (h *Handler) List(c *gin.Context) {
	items, err := h.list.Handle(c.Request.Context())
	if err != nil {
		handlePluginError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) Get(c *gin.Context) {
	record, err := h.get.Handle(c.Request.Context(), c.Param("id"))
	if err != nil {
		handlePluginError(c, err)
		return
	}
	httpx.OK(c, record)
}

func (h *Handler) MarkMerged(c *gin.Context) {
	record, err := h.markMerged.Handle(c.Request.Context(), c.Param("id"))
	if err != nil {
		handlePluginError(c, err)
		return
	}
	httpx.OK(c, record)
}

func (h *Handler) MarkDeployed(c *gin.Context) {
	record, err := h.markDeployed.Handle(c.Request.Context(), c.Param("id"))
	if err != nil {
		handlePluginError(c, err)
		return
	}
	if record == nil {
		httpx.OK(c, gin.H{"deleted": true})
		return
	}
	httpx.OK(c, record)
}

func (h *Handler) Uninstall(c *gin.Context) {
	actor, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	record, err := h.uninstall.Handle(c.Request.Context(), c.Param("id"), actor)
	if err != nil {
		handlePluginError(c, err)
		return
	}
	httpx.OK(c, record)
}

func (h *Handler) ListWorkspace(c *gin.Context) {
	workspaceID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	items, err := h.workspaceList.Handle(c.Request.Context(), workspaceID)
	if err != nil {
		handlePluginError(c, err)
		return
	}
	httpx.OK(c, gin.H{"items": items})
}

func (h *Handler) Enable(c *gin.Context) {
	h.changeWorkspaceBinding(c, h.enable)
}

func (h *Handler) Disable(c *gin.Context) {
	h.changeWorkspaceBinding(c, h.disable)
}

func (h *Handler) changeWorkspaceBinding(c *gin.Context, handler workspaceCommand) {
	workspaceID, ok := httpx.WorkspaceIDFromContext(c)
	if !ok {
		httpx.Fail(c, "TENANT_WORKSPACE_REQUIRED", "workspace required", http.StatusBadRequest)
		return
	}
	actor, ok := authn.MemberIDFromContext(c)
	if !ok {
		httpx.Fail(c, "AUTH_UNAUTHORIZED", "authorization required", http.StatusUnauthorized)
		return
	}
	binding, err := handler.Handle(c.Request.Context(), workspaceID, c.Param("id"), actor)
	if err != nil {
		handlePluginError(c, err)
		return
	}
	httpx.OK(c, binding)
}

func handlePluginError(c *gin.Context, err error) {
	domainErr, ok := kernel.IsDomainError(err)
	if !ok {
		httpx.HandleError(c, err)
		return
	}
	status := http.StatusBadRequest
	switch domainErr.Code() {
	case "PLUGIN_ID_CONFLICT", "PLUGIN_IN_USE", "PLUGIN_NOT_DEPLOYED":
		status = http.StatusConflict
	case "PLUGIN_NOT_FOUND":
		status = http.StatusNotFound
	}
	httpx.Fail(c, domainErr.Code(), domainErr.Error(), status)
}
