package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

type EnsureNamespace struct {
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	OwnerUserID    uuid.UUID
	NamespaceType  model.NamespaceType
	ActorID        uuid.UUID
}

type EnsureNamespaceHandler struct {
	repo repository.FilesystemRepository
	tx   postgres.Transactor
}

func NewEnsureNamespaceHandler(repo repository.FilesystemRepository, tx postgres.Transactor) *EnsureNamespaceHandler {
	requireMutationDependencies(repo, tx)
	return &EnsureNamespaceHandler{repo: repo, tx: tx}
}

func (h *EnsureNamespaceHandler) Handle(ctx context.Context, cmd EnsureNamespace) (*model.Namespace, error) {
	workspaceID, ownerUserID := cmd.WorkspaceID, cmd.OwnerUserID
	switch cmd.NamespaceType {
	case model.NamespaceTypeOrganization:
		workspaceID, ownerUserID = uuid.Nil, uuid.Nil
	case model.NamespaceTypeWorkspace:
		ownerUserID = uuid.Nil
	case model.NamespaceTypeUser:
		workspaceID = uuid.Nil
	}
	namespace := &model.Namespace{
		ID:             namespaceID(cmd),
		NamespaceType:  cmd.NamespaceType,
		OrganizationID: cmd.OrganizationID,
		WorkspaceID:    workspaceID,
		OwnerUserID:    ownerUserID,
		Status:         "ACTIVE",
	}
	if err := namespace.Validate(); err != nil {
		return nil, err
	}

	var existing *model.Namespace
	var err error
	switch cmd.NamespaceType {
	case model.NamespaceTypeOrganization:
		existing, err = h.repo.GetNamespaceByOrganization(ctx, cmd.OrganizationID)
	case model.NamespaceTypeWorkspace:
		existing, err = h.repo.GetNamespaceByWorkspace(ctx, cmd.WorkspaceID)
	case model.NamespaceTypeUser:
		existing, err = h.repo.GetNamespaceByUser(ctx, cmd.OrganizationID, cmd.OwnerUserID)
	}
	if err != nil && !errors.Is(err, domain.ErrNamespaceNotFound) {
		return nil, err
	}
	if existing != nil {
		namespace = existing
	}

	err = withinIdempotentMutation(ctx, h.tx, func(txCtx context.Context) error {
		if err := h.repo.CreateNamespace(txCtx, namespace); err != nil {
			return err
		}
		canonical, err := h.repo.GetNamespace(txCtx, namespace.ID)
		if err != nil && !errors.Is(err, domain.ErrNamespaceNotFound) {
			return err
		}
		if canonical != nil {
			namespace = canonical
		}
		root, rootErr := model.NewRootDirectory(namespace.ID, cmd.ActorID, ".root")
		if rootErr != nil {
			return rootErr
		}
		root.ID = uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-namespace-root:"+namespace.ID.String()))
		if idempotentRepo, ok := h.repo.(interface {
			CreateNodeIdempotent(context.Context, *model.Node) error
		}); ok {
			if err := idempotentRepo.CreateNodeIdempotent(txCtx, root); err != nil {
				return err
			}
		} else if _, err := h.repo.GetNodeIncludingTrashed(txCtx, namespace.ID, root.ID); errors.Is(err, domain.ErrNodeNotFound) {
			if err := h.repo.CreateNode(txCtx, root); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if err := h.repo.SetNamespaceRoot(txCtx, namespace.ID, root.ID); err != nil {
			return err
		}
		if err := h.repo.CreateFileLink(txCtx, model.FileLink{
			ID:        uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-namespace-root-link:"+namespace.ID.String())),
			NodeID:    root.ID,
			LinkType:  model.FileLinkTypeNamespaceRoot,
			LinkID:    namespace.ID,
			CreatedAt: time.Now().UTC(),
		}); err != nil {
			return err
		}
		namespace.RootNodeID = &root.ID
		return nil
	})
	if err != nil {
		if repaired, readErr := h.lookup(ctx, cmd); readErr == nil && repaired.RootNodeID != nil {
			return repaired, nil
		}
		return nil, err
	}
	return namespace, nil
}

func (h *EnsureNamespaceHandler) lookup(ctx context.Context, cmd EnsureNamespace) (*model.Namespace, error) {
	switch cmd.NamespaceType {
	case model.NamespaceTypeOrganization:
		return h.repo.GetNamespaceByOrganization(ctx, cmd.OrganizationID)
	case model.NamespaceTypeWorkspace:
		return h.repo.GetNamespaceByWorkspace(ctx, cmd.WorkspaceID)
	case model.NamespaceTypeUser:
		return h.repo.GetNamespaceByUser(ctx, cmd.OrganizationID, cmd.OwnerUserID)
	default:
		return nil, model.ErrInvalidNamespace
	}
}

func namespaceID(cmd EnsureNamespace) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(fmt.Sprintf("filesystem-namespace:%s:%s:%s:%s",
		cmd.NamespaceType, cmd.OrganizationID, cmd.WorkspaceID, cmd.OwnerUserID)))
}
