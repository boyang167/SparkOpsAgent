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
	repo              namespaceEnsureRepository
	tx                postgres.Transactor
	localStorageRoot  string
	ensureDefaultProv *EnsureDefaultLocalProviderHandler
}

type namespaceEnsureRepository interface {
	repository.FilesystemRepository
	CreateNamespaceIfAbsent(context.Context, *model.Namespace) error
	CreateNamespaceRootNodeIfAbsent(context.Context, *model.Node) error
	SetNamespaceRootByID(context.Context, uuid.UUID, uuid.UUID) error
	CreateNamespaceRootLink(context.Context, model.FileLink) error
}

func NewEnsureNamespaceHandler(repo namespaceEnsureRepository, tx postgres.Transactor) *EnsureNamespaceHandler {
	requireMutationDependencies(repo, tx)
	return &EnsureNamespaceHandler{repo: repo, tx: tx}
}

func (h *EnsureNamespaceHandler) WithDefaultLocalProvider(root string, ensure *EnsureDefaultLocalProviderHandler) *EnsureNamespaceHandler {
	h.localStorageRoot = root
	h.ensureDefaultProv = ensure
	return h
}

func (h *EnsureNamespaceHandler) Handle(ctx context.Context, cmd EnsureNamespace) (*model.Namespace, error) {
	cmd = normalizeEnsureNamespace(cmd)
	namespace := &model.Namespace{
		ID:             namespaceID(cmd),
		NamespaceType:  cmd.NamespaceType,
		OrganizationID: cmd.OrganizationID,
		WorkspaceID:    cmd.WorkspaceID,
		OwnerUserID:    cmd.OwnerUserID,
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
		namespace, err = canonicalNamespace(existing, cmd)
		if err != nil {
			return nil, err
		}
	}

	err = withinIdempotentMutation(ctx, h.tx, func(txCtx context.Context) error {
		if err := h.repo.CreateNamespaceIfAbsent(txCtx, namespace); err != nil {
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
		if err := h.repo.CreateNamespaceRootNodeIfAbsent(txCtx, root); err != nil {
			return err
		}
		if err := h.repo.SetNamespaceRootByID(txCtx, namespace.ID, root.ID); err != nil {
			return err
		}
		if err := h.repo.CreateNamespaceRootLink(txCtx, model.FileLink{
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
		if repaired, readErr := h.lookup(ctx, cmd); readErr == nil {
			canonical, canonicalErr := canonicalNamespace(repaired, cmd)
			if canonicalErr != nil {
				return nil, canonicalErr
			}
			if canonical.RootNodeID != nil {
				return canonical, nil
			}
		}
		return nil, err
	}
	if h.ensureDefaultProv != nil && h.localStorageRoot != "" &&
		cmd.NamespaceType == model.NamespaceTypeWorkspace {
		if _, providerErr := h.ensureDefaultProv.Handle(ctx, EnsureDefaultLocalProvider{
			OrganizationID: cmd.OrganizationID, StorageRoot: h.localStorageRoot,
		}); providerErr != nil {
			return nil, providerErr
		}
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

func canonicalNamespace(namespace *model.Namespace, cmd EnsureNamespace) (*model.Namespace, error) {
	if namespace == nil {
		return nil, domain.ErrNamespaceNotFound
	}
	if namespace.OrganizationID != cmd.OrganizationID {
		return nil, domain.ErrNamespaceMismatch
	}
	return namespace, nil
}

func namespaceID(cmd EnsureNamespace) uuid.UUID {
	cmd = normalizeEnsureNamespace(cmd)
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(fmt.Sprintf("filesystem-namespace:%s:%s:%s:%s",
		cmd.NamespaceType, cmd.OrganizationID, cmd.WorkspaceID, cmd.OwnerUserID)))
}

func normalizeEnsureNamespace(cmd EnsureNamespace) EnsureNamespace {
	switch cmd.NamespaceType {
	case model.NamespaceTypeOrganization:
		cmd.WorkspaceID, cmd.OwnerUserID = uuid.Nil, uuid.Nil
	case model.NamespaceTypeWorkspace:
		cmd.OwnerUserID = uuid.Nil
	case model.NamespaceTypeUser:
		cmd.WorkspaceID = uuid.Nil
	}
	return cmd
}
