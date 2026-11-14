package http

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
)

type InternalSFTPHandlers struct {
	BeginSFTP func(context.Context, command.BeginSFTPUpload) (command.BeginSFTPUploadResult, error)
}

func RegisterInternalSFTPRoutes(group *gin.RouterGroup, h InternalSFTPHandlers) {
	if group == nil || h.BeginSFTP == nil {
		return
	}
	group.POST("/sftp/begin-upload", func(c *gin.Context) {
		var body struct {
			OrganizationID string `json:"organization_id"`
			WorkspaceID    string `json:"workspace_id"`
			ActorID        string `json:"actor_id"`
			RelPath        string `json:"rel_path"`
			SessionPath    string `json:"session_path"`
			Size           int64  `json:"size"`
			SHA256         string `json:"sha256"`
		}
		if !decodeJSON(c, &body) {
			return
		}
		orgID, ok1 := canonicalUUID(body.OrganizationID)
		wsID, ok2 := canonicalUUID(body.WorkspaceID)
		actorID, ok3 := canonicalUUID(body.ActorID)
		if !ok1 || !ok2 || !ok3 {
			invalidRequest(c)
			return
		}
		result, err := h.BeginSFTP(c.Request.Context(), command.BeginSFTPUpload{
			OrganizationID: orgID, WorkspaceID: wsID, ActorID: actorID,
			RelPath: body.RelPath, SessionPath: body.SessionPath,
			Size: body.Size, SHA256: body.SHA256,
		})
		if err != nil {
			filesystemError(c, err)
			return
		}
		httpx.OK(c, gin.H{
			"session_id":           result.SessionID.String(),
			"temporary_object_key": result.TemporaryObjectKey,
			"storage_root":         result.StorageRoot,
		})
	})
}