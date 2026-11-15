package http

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

func validMutationNamespaceAudience(rc requestContext, namespace *model.Namespace) bool {
	if namespace == nil || namespace.OrganizationID != rc.namespace.OrganizationID {
		return false
	}
	switch namespace.NamespaceType {
	case model.NamespaceTypeUser:
		return namespace.OwnerUserID == rc.actorID
	case model.NamespaceTypeWorkspace:
		return namespace.WorkspaceID == rc.workspaceID
	case model.NamespaceTypeOrganization:
		return rc.workspaceID != uuid.Nil
	default:
		return false
	}
}

func (h *Handler) resolveMutationNamespace(c *gin.Context, rc requestContext, namespaceID string) (*model.Namespace, bool) {
	if namespaceID == "" {
		return rc.namespace, true
	}
	parsed, valid := canonicalUUID(namespaceID)
	if !valid {
		invalidRequest(c)
		return nil, false
	}
	if parsed == rc.namespace.ID {
		return rc.namespace, true
	}
	if h == nil || h.namespaceByID == nil {
		filesystemError(c, domain.ErrPersistence)
		return nil, false
	}
	namespace, err := h.namespaceByID(c.Request.Context(), parsed)
	if err != nil {
		filesystemError(c, err)
		return nil, false
	}
	if namespace == nil || !validMutationNamespaceAudience(rc, namespace) {
		filesystemError(c, domain.ErrNamespaceMismatch)
		return nil, false
	}
	return namespace, true
}

func (h *Handler) namespaceForReadRequest(c *gin.Context, rc requestContext) (*model.Namespace, bool) {
	return h.resolveMutationNamespace(c, rc, strings.TrimSpace(c.Query("namespace_id")))
}