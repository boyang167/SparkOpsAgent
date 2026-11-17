package command

import (
	"context"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

type EmptyTrash struct {
	CommandID      uuid.UUID
	ActorID        uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	NamespaceID    uuid.UUID
	Authorization
}

type EmptyTrashItemResult struct {
	NodeID uuid.UUID
	Error  error
}

type EmptyTrashResult struct {
	Items []EmptyTrashItemResult
}

type EmptyTrashHandler struct {
	repo  repository.FilesystemRepository
	audit auditPort
	purge *PermanentDeleteNodeHandler
}

func NewEmptyTrashHandler(repo repository.FilesystemRepository, tx postgres.Transactor, bus messaging.Bus, logger auditPort) *EmptyTrashHandler {
	return &EmptyTrashHandler{
		repo:  repo,
		audit: logger,
		purge: NewPermanentDeleteNodeHandler(repo, tx, bus, logger),
	}
}

func (h *EmptyTrashHandler) Handle(ctx context.Context, cmd EmptyTrash) (EmptyTrashResult, error) {
	if !cmd.Allowed {
		err := authorizationDenied(cmd.Reason)
		return EmptyTrashResult{}, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.trash.empty", err)
	}
	entries, err := h.listAllTrashEntries(ctx, cmd.NamespaceID)
	if err != nil {
		return EmptyTrashResult{}, err
	}
	results := make([]EmptyTrashItemResult, 0, len(entries))
	for _, entry := range entries {
		childCommandID := uuid.NewSHA1(cmd.CommandID, []byte("empty-trash:"+entry.Node.ID.String()))
		itemAuth := cmd.Authorization
		itemAuth.ResourceID = entry.Node.ID
		itemAuth.Permission = model.ResourcePermissionDelete
		itemErr := h.purge.Handle(ctx, PermanentDeleteNode{
			CommandID:      childCommandID,
			ActorID:        cmd.ActorID,
			OrganizationID: cmd.OrganizationID,
			WorkspaceID:    cmd.WorkspaceID,
			NamespaceID:    cmd.NamespaceID,
			NodeID:         entry.Node.ID,
			Authorization:  itemAuth,
		})
		results = append(results, EmptyTrashItemResult{NodeID: entry.Node.ID, Error: itemErr})
	}
	return EmptyTrashResult{Items: results}, nil
}

func (h *EmptyTrashHandler) listAllTrashEntries(ctx context.Context, namespaceID uuid.UUID) ([]repository.TrashEntry, error) {
	var all []repository.TrashEntry
	cursor := ""
	for {
		batch, next, err := h.repo.ListTrashEntries(ctx, repository.TrashListQuery{
			NamespaceID: namespaceID,
			Limit:       200,
			Cursor:      cursor,
		})
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if next == "" {
			break
		}
		cursor = next
	}
	return all, nil
}
