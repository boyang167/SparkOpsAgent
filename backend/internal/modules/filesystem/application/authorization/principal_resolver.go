package authorization

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
)

var ErrPrincipalScopeMismatch = errors.New("principal workspace organization mismatch")

type ResolvePrincipalsInput struct {
	ActorID        uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
}

type PrincipalResolver interface {
	Resolve(context.Context, ResolvePrincipalsInput) ([]repository.PrincipalRef, error)
}

type WorkspaceAuthority interface {
	ResolveWorkspaceAccess(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, bool, error)
}

type WorkspaceRoleReader interface {
	ListWorkspaceRoleIDs(context.Context, uuid.UUID, uuid.UUID) ([]uuid.UUID, error)
}

type principalResolver struct {
	workspaces WorkspaceAuthority
	roles      WorkspaceRoleReader
}

func NewPrincipalResolver(workspaces WorkspaceAuthority, roles WorkspaceRoleReader) PrincipalResolver {
	return &principalResolver{workspaces: workspaces, roles: roles}
}

func (r *principalResolver) Resolve(ctx context.Context, input ResolvePrincipalsInput) ([]repository.PrincipalRef, error) {
	principals := []repository.PrincipalRef{{Type: model.PrincipalUser, ID: input.ActorID}}
	if input.WorkspaceID == uuid.Nil {
		if input.OrganizationID != uuid.Nil {
			principals = append(principals, repository.PrincipalRef{
				Type: model.PrincipalOrganization, ID: input.OrganizationID,
			})
		}
		return principals, nil
	}
	organizationID, active, err := r.workspaces.ResolveWorkspaceAccess(ctx, input.WorkspaceID, input.ActorID)
	if err != nil {
		return nil, err
	}
	if organizationID != input.OrganizationID {
		return nil, ErrPrincipalScopeMismatch
	}
	if !active {
		return principals, nil
	}
	principals = append(principals,
		repository.PrincipalRef{Type: model.PrincipalWorkspace, ID: input.WorkspaceID},
		repository.PrincipalRef{Type: model.PrincipalOrganization, ID: organizationID},
	)
	roleIDs, err := r.roles.ListWorkspaceRoleIDs(ctx, input.WorkspaceID, input.ActorID)
	if err != nil {
		return nil, err
	}
	seen := make(map[uuid.UUID]struct{}, len(roleIDs))
	for _, roleID := range roleIDs {
		if roleID == uuid.Nil {
			continue
		}
		if _, exists := seen[roleID]; exists {
			continue
		}
		seen[roleID] = struct{}{}
		principals = append(principals, repository.PrincipalRef{Type: model.PrincipalRole, ID: roleID})
	}
	return principals, nil
}
