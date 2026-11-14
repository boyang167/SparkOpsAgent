package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type GrantRequest struct {
	CommandID             uuid.UUID
	ActorID               uuid.UUID
	OrganizationID        uuid.UUID
	GranteeOrganizationID uuid.UUID
	WorkspaceID           uuid.UUID
	NamespaceID           uuid.UUID
	NodeID                uuid.UUID
	Grant                 model.ResourceGrant
	TraceID               string
}

type FilesystemGrantPort interface {
	Create(context.Context, GrantRequest) (model.ResourceGrant, error)
	Revoke(context.Context, GrantRequest, uuid.UUID) error
	List(context.Context, GrantRequest) ([]model.ResourceGrant, error)
}
