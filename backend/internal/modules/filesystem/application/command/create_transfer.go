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
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/service"
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

type transferPlacer interface {
	Place(context.Context, service.PlacementRequest) (service.PlacementDecision, error)
}

type CreateTransferHandler struct {
	repo            repository.FilesystemRepository
	tx              postgres.Transactor
	bus             messaging.Bus
	audit           auditPort
	placement       transferPlacer
	directLocalRoot string
}

func NewCreateTransferHandler(repo repository.FilesystemRepository, tx postgres.Transactor, bus messaging.Bus, logger auditPort, placement transferPlacer) *CreateTransferHandler {
	requireMutationDependencies(repo, tx)
	return &CreateTransferHandler{repo: repo, tx: tx, bus: bus, audit: logger, placement: placement}
}

func (h *CreateTransferHandler) WithDirectLocalProvider(root string) *CreateTransferHandler {
	h.directLocalRoot = strings.TrimSpace(root)
	return h
}

func (h *CreateTransferHandler) Handle(ctx context.Context, cmd CreateTransfer) (out TransferDescriptor, err error) {
	cmd.ExpiresAt = model.CanonicalTransferExpiry(cmd.ExpiresAt)
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.SHA256 = strings.ToLower(strings.TrimSpace(cmd.SHA256))
	ctx = withTraceID(ctx, cmd.TraceID)
	if cmd.ParentID != nil {
		ctx = withResourceID(ctx, cmd.ParentID.String())
	}
	ctx, finishAudit := beginMutationAudit(ctx, h.audit, cmd.ActorID, cmd.OrganizationID, cmd.WorkspaceID.String(), "filesystem.transfer.create")
	defer finishAudit(&err)

	ns, scopeErr := h.repo.GetNamespace(ctx, cmd.NamespaceID)
	if scopeErr != nil {
		return out, scopeErr
	}
	if ns.OrganizationID != cmd.OrganizationID ||
		(ns.NamespaceType == model.NamespaceTypeWorkspace && ns.WorkspaceID != cmd.WorkspaceID) {
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
	if strings.TrimSpace(cmd.IdempotencyKey) == "" {
		return out, model.ErrInvalidTransfer
	}

	protocol := cmd.Protocol
	var placementDecision *service.PlacementDecision
	providerID := cmd.ProviderID
	for attempt := 0; attempt < 2; attempt++ {
		placementDecision = nil
		if h.placement != nil {
			decision, placeErr := h.placement.Place(ctx, service.PlacementRequest{
				OrganizationID:          cmd.OrganizationID,
				NamespaceType:           ns.NamespaceType,
				WorkspaceID:             ns.WorkspaceID,
				UserID:                  ns.OwnerUserID,
				Classification:          cmd.Classification,
				Size:                    cmd.Size,
				RequiresPresignedUpload: protocol == model.TransferProtocolMultipart,
			})
			if placeErr != nil {
				return out, placeErr
			}
			providerID = decision.ProviderID
			placementDecision = &decision
		} else if providerID == uuid.Nil {
			providerID, err = h.defaultProviderID(ctx, cmd.OrganizationID)
			if err != nil {
				return out, err
			}
		}
		if protocol != model.TransferProtocolDirect {
			break
		}
		provider, providerErr := h.repo.GetStorageProvider(ctx, providerID)
		if providerErr != nil {
			return out, providerErr
		}
		if directLocalGatewayCompatible(provider, cmd.OrganizationID) {
			break
		}
		if h.placement == nil || attempt > 0 {
			if placementDecision != nil {
				return out, domain.ErrStorageGatewayMismatch
			}
			return out, model.ErrInvalidTransfer
		}
		protocol = model.TransferProtocolMultipart
		providerID = cmd.ProviderID
	}
	cmd.Protocol = protocol
	if protocol == model.TransferProtocolDirect {
		provider, providerErr := h.repo.GetStorageProvider(ctx, providerID)
		if providerErr != nil {
			return out, providerErr
		}
		if !directLocalGatewayCompatible(provider, cmd.OrganizationID) {
			if placementDecision != nil {
				return out, domain.ErrStorageGatewayMismatch
			}
			return out, model.ErrInvalidTransfer
		}
	}

	requestHash := transferRequestHash(cmd, *parentID, placementDecision)
	sessionID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-transfer:"+cmd.NamespaceID.String()+":"+cmd.IdempotencyKey))
	temporaryObjectKey := fmt.Sprintf("transfers/%s/content", sessionID)
	ctx = withResourceID(ctx, sessionID.String())
	var authoritative *model.TransferSession
	err = withinIdempotentMutation(ctx, h.tx, func(txCtx context.Context) error {
		existing, existingErr := h.repo.GetTransferByNamespaceIdempotency(txCtx, cmd.NamespaceID, cmd.IdempotencyKey)
		if existingErr == nil {
			if existing.RequestHash() != requestHash {
				return domain.ErrTransferIdempotencyConflict
			}
			authoritative = existing
			markMutationReplay(ctx)
			return nil
		}
		if !errors.Is(existingErr, domain.ErrTransferNotFound) {
			return existingErr
		}
		session, buildErr := model.NewTransferSession(model.TransferSessionInput{
			ID: sessionID, WorkspaceID: cmd.WorkspaceID, NamespaceID: cmd.NamespaceID,
			TargetParentID: parentID, TargetName: cmd.Name, CreatedBy: cmd.ActorID,
			Protocol: cmd.Protocol, ExpectedSize: cmd.Size, ExpectedSHA256: cmd.SHA256,
			IdempotencyKey: cmd.IdempotencyKey, RequestHash: requestHash, TemporaryObjectKey: temporaryObjectKey,
			ExpiresAt: cmd.ExpiresAt, ProviderID: &providerID, Classification: cmd.Classification,
		})
		if buildErr != nil {
			return buildErr
		}
		createErr := h.repo.CreateTransfer(txCtx, session)
		if errors.Is(createErr, domain.ErrTransferConflict) {
			existing, readErr := h.repo.GetTransferByNamespaceIdempotency(txCtx, cmd.NamespaceID, cmd.IdempotencyKey)
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
		accountID, reserveErr := h.repo.ReserveNamespaceQuota(txCtx, cmd.NamespaceID, cmd.Size, 1)
		if reserveErr != nil {
			return reserveErr
		}
		var workspaceID *uuid.UUID
		if cmd.WorkspaceID != uuid.Nil {
			workspaceID = &cmd.WorkspaceID
		}
		if reserveErr = h.repo.CreateQuotaReservation(txCtx, repository.QuotaReservation{
			ID:             uuid.NewSHA1(uuid.NameSpaceURL, []byte("filesystem-transfer-reservation:"+session.ID().String())),
			QuotaAccountID: accountID, NamespaceID: cmd.NamespaceID,
			WorkspaceID: workspaceID, SessionID: session.ID(),
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
			existing, readErr := h.repo.GetTransferByNamespaceIdempotency(ctx, cmd.NamespaceID, cmd.IdempotencyKey)
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

func transferRequestHash(cmd CreateTransfer, parentID uuid.UUID, placement *service.PlacementDecision) string {
	providerID := cmd.ProviderID
	poolID := uuid.Nil
	policyVersion := int64(0)
	matchedRules := ""
	if placement != nil {
		providerID = placement.ProviderID
		poolID = placement.PoolID
		policyVersion = placement.PolicyVersion
		matchedRules = strings.Join(placement.MatchedRuleIDs, ",")
	}
	return mutationRequestHash(
		"create_transfer", cmd.NamespaceID, parentID, strings.TrimSpace(cmd.Name), cmd.Protocol,
		cmd.Size, strings.ToLower(strings.TrimSpace(cmd.SHA256)), cmd.Classification, cmd.ActorID,
		model.CanonicalTransferExpiry(cmd.ExpiresAt).Format(time.RFC3339Nano), providerID,
		poolID, policyVersion, matchedRules,
	)
}

func DescriptorFromSession(session *model.TransferSession) TransferDescriptor {
	return descriptor(session)
}

func directLocalGatewayCompatible(provider *model.StorageProvider, organizationID uuid.UUID) bool {
	if provider == nil ||
		provider.OrganizationID != organizationID ||
		provider.ProviderType != model.ProviderTypeLocal ||
		strings.TrimSpace(provider.Status) != "ACTIVE" {
		return false
	}
	return strings.TrimSpace(provider.Endpoint) != ""
}

func (h *CreateTransferHandler) defaultProviderID(ctx context.Context, organizationID uuid.UUID) (uuid.UUID, error) {
	providers, err := h.repo.ListStorageProviders(ctx, organizationID)
	if err != nil {
		return uuid.Nil, err
	}
	for _, provider := range providers {
		if provider == nil || strings.TrimSpace(provider.Status) != "ACTIVE" {
			continue
		}
		if provider.ProviderType == model.ProviderTypeLocal {
			return provider.ID, nil
		}
	}
	if organizationID != uuid.Nil {
		return DefaultLocalProviderID(organizationID), nil
	}
	return uuid.Nil, model.ErrInvalidTransfer
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
