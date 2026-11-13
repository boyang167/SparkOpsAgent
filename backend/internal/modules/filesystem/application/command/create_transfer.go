package command

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/event"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	sharedevents "github.com/sparkops/sparkops-agent/backend/internal/shared/events"
)

const (
	maxTransferBytes = int64(20 * 1024 * 1024 * 1024)
	defaultPartSize  = int64(8 * 1024 * 1024)
)

type CreateTransfer struct {
	ActorID        uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	NamespaceID    uuid.UUID
	ParentID       *uuid.UUID
	Name           string
	Protocol       model.TransferProtocol
	Size           int64
	SHA256         string
	Classification model.TransferClassification
	IdempotencyKey string
	ExpiresAt      time.Time
	ProviderID     uuid.UUID
	Authorization
}

type TransferDescriptor struct {
	SessionID  uuid.UUID
	Protocol   model.TransferProtocol
	ExpiresAt  time.Time
	PartSize   int64
	TotalParts int
}

type CreateTransferHandler struct {
	repo  repository.FilesystemRepository
	tx    postgres.Transactor
	bus   messaging.Bus
	audit auditPort
}

func NewCreateTransferHandler(repo repository.FilesystemRepository, tx postgres.Transactor, bus messaging.Bus, logger auditPort) *CreateTransferHandler {
	requireMutationDependencies(repo, tx)
	return &CreateTransferHandler{repo: repo, tx: tx, bus: bus, audit: logger}
}

func (h *CreateTransferHandler) Handle(ctx context.Context, cmd CreateTransfer) (out TransferDescriptor, err error) {
	ctx = withTraceID(ctx, cmd.TraceID)
	if cmd.ParentID != nil {
		ctx = withResourceID(ctx, cmd.ParentID.String())
	}
	ctx, finishAudit := beginMutationAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.transfer.create")
	defer finishAudit(&err)

	ns, scopeErr := h.repo.GetNamespaceByWorkspace(ctx, cmd.WorkspaceID)
	if scopeErr != nil {
		return out, scopeErr
	}
	if ns.ID != cmd.NamespaceID || ns.OrganizationID != cmd.OrganizationID {
		return out, domain.ErrNamespaceMismatch
	}
	parentID := cmd.ParentID
	if parentID == nil {
		parentID = ns.RootNodeID
	}
	if parentID == nil {
		return out, domain.ErrParentNotFound
	}
	ctx = withResourceID(ctx, parentID.String())
	if authErr := requireAllowed(cmd.Authorization, cmd.WorkspaceID, *parentID, model.ResourcePermissionWrite); authErr != nil {
		return out, deniedWithAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.transfer.create", authErr)
	}
	requestHash := transferRequestHash(cmd, *parentID)
	existing, existingErr := h.repo.GetTransferByIdempotency(ctx, cmd.WorkspaceID, cmd.IdempotencyKey)
	if existingErr == nil {
		ctx = withResourceID(ctx, existing.ID().String())
		if existing.RequestHash() != requestHash {
			return out, domain.ErrTransferIdempotencyConflict
		}
		markMutationReplay(ctx)
		return descriptor(existing), nil
	}
	if !errors.Is(existingErr, domain.ErrTransferNotFound) {
		return out, existingErr
	}
	parent, parentErr := h.repo.GetNode(ctx, cmd.NamespaceID, *parentID)
	if parentErr != nil {
		if errors.Is(parentErr, domain.ErrNodeNotFound) {
			return out, domain.ErrParentNotFound
		}
		return out, parentErr
	}
	if parent.Type != model.NodeTypeDirectory {
		return out, domain.ErrParentNotDirectory
	}
	if cmd.ProviderID == uuid.Nil || strings.TrimSpace(cmd.IdempotencyKey) == "" {
		return out, model.ErrInvalidTransfer
	}

	sessionID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-transfer:"+cmd.WorkspaceID.String()+":"+cmd.IdempotencyKey))
	temporaryObjectKey := fmt.Sprintf("transfers/%s/content", sessionID)
	providerID := cmd.ProviderID
	expiresAt := model.CanonicalTransferExpiry(cmd.ExpiresAt)
	session, buildErr := model.NewTransferSession(model.TransferSessionInput{
		ID: sessionID, WorkspaceID: cmd.WorkspaceID, NamespaceID: cmd.NamespaceID,
		TargetParentID: parentID, TargetName: cmd.Name, CreatedBy: cmd.ActorID,
		Protocol: cmd.Protocol, ExpectedSize: cmd.Size, ExpectedSHA256: strings.ToLower(strings.TrimSpace(cmd.SHA256)),
		IdempotencyKey: cmd.IdempotencyKey, RequestHash: requestHash, TemporaryObjectKey: temporaryObjectKey,
		ExpiresAt: expiresAt, ProviderID: &providerID, Classification: cmd.Classification,
	})
	if buildErr != nil {
		return out, buildErr
	}
	ctx = withResourceID(ctx, session.ID().String())
	var authoritative *model.TransferSession
	err = withinIdempotentMutation(ctx, h.tx, func(txCtx context.Context) error {
		createErr := h.repo.CreateTransfer(txCtx, session)
		if errors.Is(createErr, domain.ErrTransferConflict) {
			existing, readErr := h.repo.GetTransferByIdempotency(txCtx, cmd.WorkspaceID, cmd.IdempotencyKey)
			if readErr != nil {
				return readErr
			}
			if existing.RequestHash() != requestHash {
				return domain.ErrTransferIdempotencyConflict
			}
			authoritative = existing
			markMutationReplay(ctx)
			return nil
		}
		if createErr != nil {
			return createErr
		}
		accountID, reserveErr := h.repo.ReserveTransferQuota(txCtx, cmd.WorkspaceID, cmd.Size, 1)
		if reserveErr != nil {
			return reserveErr
		}
		if reserveErr = h.repo.CreateQuotaReservation(txCtx, repository.QuotaReservation{
			ID:             uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-transfer-reservation:"+session.ID().String())),
			QuotaAccountID: accountID, WorkspaceID: cmd.WorkspaceID, SessionID: session.ID(),
			Bytes: cmd.Size, Files: 1, State: "RESERVED", ExpiresAt: session.ExpiresAt(),
		}); reserveErr != nil {
			return reserveErr
		}
		if auditErr := logMutation(txCtx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.transfer.create", true, nil); auditErr != nil {
			return auditErr
		}
		if h.bus != nil {
			if publishErr := h.bus.Publish(txCtx, event.TransferInitiated{
				BaseEvent: sharedevents.NewBaseEvent(event.TransferInitiatedName),
				SessionID: session.ID(), WorkspaceID: cmd.WorkspaceID, NamespaceID: cmd.NamespaceID,
			}); publishErr != nil {
				return publishErr
			}
		}
		authoritative = session
		return nil
	})
	if err != nil {
		if errors.Is(err, postgres.ErrCommitOutcomeUnknown) {
			existing, readErr := h.repo.GetTransferByIdempotency(ctx, cmd.WorkspaceID, cmd.IdempotencyKey)
			if readErr == nil && existing.RequestHash() == requestHash {
				markMutationReplay(ctx)
				return descriptor(existing), nil
			}
			return out, domain.ErrOutcomeUnknown
		}
		return out, err
	}
	return descriptor(authoritative), nil
}

func transferRequestHash(cmd CreateTransfer, parentID uuid.UUID) string {
	return mutationRequestHash(
		"create_transfer", cmd.NamespaceID, parentID, strings.TrimSpace(cmd.Name), cmd.Protocol,
		cmd.Size, strings.ToLower(strings.TrimSpace(cmd.SHA256)), cmd.Classification, cmd.ActorID,
		model.CanonicalTransferExpiry(cmd.ExpiresAt).Format(time.RFC3339Nano), cmd.ProviderID,
	)
}

func descriptor(session *model.TransferSession) TransferDescriptor {
	partSize := defaultPartSize
	if session.Protocol() == model.TransferProtocolDirect || session.ExpectedSize() < partSize {
		partSize = session.ExpectedSize()
	}
	totalParts := 1
	if session.ExpectedSize() == 0 {
		totalParts = 0
	} else if partSize > 0 {
		totalParts = int((session.ExpectedSize() + partSize - 1) / partSize)
	}
	return TransferDescriptor{
		SessionID: session.ID(), Protocol: session.Protocol(), ExpiresAt: session.ExpiresAt(),
		PartSize: partSize, TotalParts: totalParts,
	}
}
