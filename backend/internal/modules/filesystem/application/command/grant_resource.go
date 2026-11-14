package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	filesystemport "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/port"
)

type GrantResource struct {
	CommandID             uuid.UUID
	ActorID               uuid.UUID
	OrganizationID        uuid.UUID
	GranteeOrganizationID uuid.UUID
	WorkspaceID           uuid.UUID
	NamespaceID           uuid.UUID
	NodeID                uuid.UUID
	GranteeID             uuid.UUID
	Permission            model.ResourcePermission
	Effect                model.GrantEffect
	ExpiresAt             *time.Time
	TraceID               string
	Anonymous             bool
}

type GrantResourceHandler struct {
	port filesystemport.FilesystemGrantPort
}

func NewGrantResourceHandler(port filesystemport.FilesystemGrantPort) *GrantResourceHandler {
	return &GrantResourceHandler{port: port}
}

func (h *GrantResourceHandler) Handle(ctx context.Context, cmd GrantResource) (model.ResourceGrant, error) {
	if cmd.Anonymous {
		return model.ResourceGrant{}, domain.ErrAnonymousLinkDisabled
	}
	if cmd.OrganizationID == uuid.Nil || cmd.GranteeOrganizationID == uuid.Nil || cmd.OrganizationID != cmd.GranteeOrganizationID {
		return model.ResourceGrant{}, domain.ErrCrossOrganizationGrant
	}
	if h == nil || h.port == nil {
		return model.ResourceGrant{}, domain.ErrPersistence
	}
	return h.port.Create(ctx, grantRequest(cmd))
}

func (h *GrantResourceHandler) List(ctx context.Context, cmd GrantResource) ([]model.ResourceGrant, error) {
	if h == nil || h.port == nil {
		return nil, domain.ErrPersistence
	}
	return h.port.List(ctx, grantRequest(cmd))
}

func (h *GrantResourceHandler) Revoke(ctx context.Context, cmd GrantResource, grantID uuid.UUID) error {
	if h == nil || h.port == nil {
		return domain.ErrPersistence
	}
	return h.port.Revoke(ctx, grantRequest(cmd), grantID)
}

func grantRequest(cmd GrantResource) filesystemport.GrantRequest {
	return filesystemport.GrantRequest{
		CommandID: cmd.CommandID, ActorID: cmd.ActorID, OrganizationID: cmd.OrganizationID,
		GranteeOrganizationID: cmd.GranteeOrganizationID,
		WorkspaceID:           cmd.WorkspaceID, NamespaceID: cmd.NamespaceID, NodeID: cmd.NodeID, TraceID: cmd.TraceID,
		Grant: model.ResourceGrant{GranteeID: cmd.GranteeID, Permission: cmd.Permission, Effect: cmd.Effect, ExpiresAt: cmd.ExpiresAt},
	}
}
