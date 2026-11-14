package query

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	resourceauth "github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/authorization"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

var resourceActions = []model.ResourcePermission{
	model.ResourcePermissionRead,
	model.ResourcePermissionWrite,
	model.ResourcePermissionDelete,
	model.ResourcePermissionShare,
	model.ResourcePermissionAdmin,
}

type ListNamespaces struct {
	ActorID        uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
}

type NamespaceSummary struct {
	ID             uuid.UUID
	NamespaceType  model.NamespaceType
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	OwnerUserID    uuid.UUID
	RootNodeID     *uuid.UUID
	Status         string
	Capabilities   []model.ResourcePermission
	EntryNodeIDs   []uuid.UUID
}

type ListNamespacesRepository interface {
	SharedWithMeRepository
	GetNamespaceByUser(context.Context, uuid.UUID, uuid.UUID) (*model.Namespace, error)
	GetNamespaceByWorkspace(context.Context, uuid.UUID) (*model.Namespace, error)
	GetNamespaceByOrganization(context.Context, uuid.UUID) (*model.Namespace, error)
}

type ResourceAuthorizer interface {
	Authorize(context.Context, resourceauth.Request) resourceauth.Decision
}

type ListNamespacesHandler struct {
	repo       ListNamespacesRepository
	principals resourceauth.PrincipalResolver
	authorizer ResourceAuthorizer
}

func NewListNamespacesHandler(repo ListNamespacesRepository, principals resourceauth.PrincipalResolver, authorizer ResourceAuthorizer) *ListNamespacesHandler {
	return &ListNamespacesHandler{repo: repo, principals: principals, authorizer: authorizer}
}

func (h *ListNamespacesHandler) Handle(ctx context.Context, q ListNamespaces) ([]NamespaceSummary, error) {
	principals, err := h.principals.Resolve(ctx, resourceauth.ResolvePrincipalsInput{
		ActorID: q.ActorID, OrganizationID: q.OrganizationID, WorkspaceID: q.WorkspaceID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]NamespaceSummary, 0, 3)
	user, err := h.repo.GetNamespaceByUser(ctx, q.OrganizationID, q.ActorID)
	if err == nil {
		out = append(out, h.summary(ctx, q, user, nil))
	} else if !errors.Is(err, domain.ErrNamespaceNotFound) {
		return nil, err
	}
	if q.WorkspaceID != uuid.Nil {
		workspace, workspaceErr := h.repo.GetNamespaceByWorkspace(ctx, q.WorkspaceID)
		if workspaceErr == nil {
			if workspace.OrganizationID != q.OrganizationID {
				return nil, resourceauth.ErrPrincipalScopeMismatch
			}
			out = append(out, h.summary(ctx, q, workspace, nil))
		} else if !errors.Is(workspaceErr, domain.ErrNamespaceNotFound) {
			return nil, workspaceErr
		}
	}
	organization, err := h.repo.GetNamespaceByOrganization(ctx, q.OrganizationID)
	if errors.Is(err, domain.ErrNamespaceNotFound) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	entryIDs := make([]uuid.UUID, 0, 1)
	cursor := ""
	for len(entryIDs) == 0 {
		entries, next, listErr := h.repo.ListSharedNodes(ctx, q.OrganizationID, principals, SharedWithMeFilter{
			NamespaceID: organization.ID, Cursor: cursor, Limit: 200, Principals: principals,
		}, nowUTC())
		if listErr != nil {
			return nil, listErr
		}
		for _, entry := range entries {
			if containsAction(entry.EffectiveActions, model.ResourcePermissionRead) {
				entryIDs = append(entryIDs, entry.Node.ID)
				break
			}
		}
		if len(entryIDs) > 0 || next == "" || next == cursor {
			break
		}
		cursor = next
	}
	rootVisible := organization.RootNodeID != nil && h.authorizer.Authorize(ctx, resourceauth.Request{
		ActorID: q.ActorID, WorkspaceID: q.WorkspaceID, NamespaceID: organization.ID,
		NodeID: *organization.RootNodeID, Permission: model.ResourcePermissionRead,
	}).Allowed
	if rootVisible || len(entryIDs) > 0 {
		out = append(out, h.summary(ctx, q, organization, entryIDs))
	}
	return out, nil
}

func (h *ListNamespacesHandler) summary(ctx context.Context, q ListNamespaces, namespace *model.Namespace, entries []uuid.UUID) NamespaceSummary {
	summary := NamespaceSummary{
		ID: namespace.ID, NamespaceType: namespace.NamespaceType, OrganizationID: namespace.OrganizationID,
		WorkspaceID: namespace.WorkspaceID, OwnerUserID: namespace.OwnerUserID, RootNodeID: namespace.RootNodeID,
		Status: namespace.Status, EntryNodeIDs: entries,
	}
	if namespace.RootNodeID == nil {
		return summary
	}
	for _, action := range resourceActions {
		if h.authorizer.Authorize(ctx, resourceauth.Request{
			ActorID: q.ActorID, WorkspaceID: q.WorkspaceID, NamespaceID: namespace.ID,
			NodeID: *namespace.RootNodeID, Permission: action,
		}).Allowed {
			summary.Capabilities = append(summary.Capabilities, action)
		}
	}
	return summary
}

func containsAction(actions []model.ResourcePermission, action model.ResourcePermission) bool {
	for _, candidate := range actions {
		if candidate == action {
			return true
		}
	}
	return false
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
