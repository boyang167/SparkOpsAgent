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

const NamespaceEntryNodeLimit = 32

type ListNamespaces struct {
	ActorID        uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
}

type NamespaceSummary struct {
	ID                  uuid.UUID
	NamespaceType       model.NamespaceType
	OrganizationID      uuid.UUID
	WorkspaceID         uuid.UUID
	OwnerUserID         uuid.UUID
	RootNodeID          *uuid.UUID
	Status              string
	Capabilities        []model.ResourcePermission
	EntryNodeIDs        []uuid.UUID
	EntryNodesTruncated bool
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
	authorizer GovernanceAuthorizer
}

func NewListNamespacesHandler(repo ListNamespacesRepository, principals resourceauth.PrincipalResolver, authorizer GovernanceAuthorizer) *ListNamespacesHandler {
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
		out = append(out, h.rootSummary(ctx, q, user))
	} else if !errors.Is(err, domain.ErrNamespaceNotFound) {
		return nil, err
	}
	if q.WorkspaceID != uuid.Nil {
		workspace, workspaceErr := h.repo.GetNamespaceByWorkspace(ctx, q.WorkspaceID)
		if workspaceErr == nil {
			if workspace.OrganizationID != q.OrganizationID {
				return nil, resourceauth.ErrPrincipalScopeMismatch
			}
			out = append(out, h.rootSummary(ctx, q, workspace))
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
	organizationSummary := h.rootSummary(ctx, q, organization)
	if organizationSummary.RootNodeID != nil {
		return append(out, organizationSummary), nil
	}
	entryIDs := make([]uuid.UUID, 0)
	capabilities := make(map[model.ResourcePermission]struct{})
	cursor := ""
	scanned := 0
	hasMore := false
	for scanned < SharedCandidateScanBudget && len(entryIDs) <= NamespaceEntryNodeLimit {
		pageStart := cursor
		entries, next, listErr := h.repo.ListSharedNodes(ctx, q.OrganizationID, principals, SharedWithMeFilter{
			NamespaceID: organization.ID, Cursor: cursor, Limit: SharedCandidateChunkSize, Principals: principals,
		}, nowUTC())
		if listErr != nil {
			return nil, listErr
		}
		if remaining := SharedCandidateScanBudget - scanned; len(entries) > remaining {
			entries = entries[:remaining]
		}
		effectiveEntries := authorizeSharedCandidates(ctx, h.authorizer, SharedWithMe{
			ActorID: q.ActorID, OrganizationID: q.OrganizationID, WorkspaceID: q.WorkspaceID,
			NamespaceID: organization.ID,
		}, entries)
		for index, entry := range entries {
			scanned++
			effective := effectiveEntries[index]
			if !containsAction(effective.EffectiveActions, model.ResourcePermissionRead) {
				continue
			}
			entryIDs = append(entryIDs, entry.Node.ID)
			if len(entryIDs) > NamespaceEntryNodeLimit {
				break
			}
			for _, action := range effective.EffectiveActions {
				capabilities[action] = struct{}{}
			}
		}
		hasMore = next != ""
		if next == "" || next == pageStart {
			break
		}
		cursor = next
	}
	if len(entryIDs) == 0 && scanned == SharedCandidateScanBudget && hasMore {
		return nil, domain.ErrAuthorizationScanBudgetExceeded
	}
	if len(entryIDs) > 0 {
		if len(entryIDs) > NamespaceEntryNodeLimit {
			entryIDs = entryIDs[:NamespaceEntryNodeLimit]
			organizationSummary.EntryNodesTruncated = true
		}
		if scanned == SharedCandidateScanBudget && hasMore {
			organizationSummary.EntryNodesTruncated = true
		}
		organizationSummary.EntryNodeIDs = entryIDs
		for _, action := range resourceActions {
			if _, ok := capabilities[action]; ok {
				organizationSummary.Capabilities = append(organizationSummary.Capabilities, action)
			}
		}
		out = append(out, organizationSummary)
	}
	return out, nil
}

func (h *ListNamespacesHandler) rootSummary(ctx context.Context, q ListNamespaces, namespace *model.Namespace) NamespaceSummary {
	summary := NamespaceSummary{
		ID: namespace.ID, NamespaceType: namespace.NamespaceType, OrganizationID: namespace.OrganizationID,
		WorkspaceID: namespace.WorkspaceID, OwnerUserID: namespace.OwnerUserID, Status: namespace.Status,
	}
	if namespace.RootNodeID == nil {
		return summary
	}
	rootCapabilities := make([]model.ResourcePermission, 0, len(resourceActions))
	rootReadable := false
	for _, action := range resourceActions {
		if h.authorizer.Authorize(ctx, resourceauth.Request{
			ActorID: q.ActorID, WorkspaceID: q.WorkspaceID, NamespaceID: namespace.ID,
			NodeID: *namespace.RootNodeID, Permission: action,
		}).Allowed {
			rootCapabilities = append(rootCapabilities, action)
			if action == model.ResourcePermissionRead {
				rootReadable = true
				rootID := *namespace.RootNodeID
				summary.RootNodeID = &rootID
			}
		}
	}
	if rootReadable {
		summary.Capabilities = rootCapabilities
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
