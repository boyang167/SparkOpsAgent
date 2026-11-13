package command

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/service"
)

type IssueCapability struct {
	CommandID            uuid.UUID
	OrganizationID       uuid.UUID
	WorkspaceID          uuid.UUID
	ActorID              uuid.UUID
	AgentID              *uuid.UUID
	RunID                uuid.UUID
	ResourceIDs          []uuid.UUID
	DelegatorResourceIDs []uuid.UUID
	RunResourceIDs       []uuid.UUID
	Actions              []string
	RunActions           []string
	Audience             string
	ExpiresAt            time.Time
}

type IssuedCapability struct {
	ID          uuid.UUID
	Token       string
	Hash        string
	ResourceIDs []uuid.UUID
	ExpiresAt   time.Time
	Replayed    bool
}

type capabilityIssuer interface {
	Issue(context.Context, service.CapabilityRequest, service.RunScope) (service.CapabilityToken, error)
}

type IssueCapabilityHandler struct {
	issuer capabilityIssuer
	store  repository.CapabilityRepository
}

func NewIssueCapabilityHandler(issuer capabilityIssuer, store repository.CapabilityRepository) *IssueCapabilityHandler {
	if issuer == nil || store == nil {
		panic("issue capability requires issuer and store")
	}
	return &IssueCapabilityHandler{issuer: issuer, store: store}
}

func (h *IssueCapabilityHandler) Handle(ctx context.Context, cmd IssueCapability) (IssuedCapability, error) {
	if cmd.CommandID == uuid.Nil {
		cmd.CommandID = uuid.New()
	}
	if existing, err := h.store.GetCapability(ctx, cmd.CommandID); err == nil {
		return replayedCapability(existing), nil
	} else if !errors.Is(err, domain.ErrCapabilityNotFound) {
		return IssuedCapability{}, err
	}
	if cmd.Audience == "" || cmd.RunID == uuid.Nil || cmd.OrganizationID == uuid.Nil || cmd.WorkspaceID == uuid.Nil || !cmd.ExpiresAt.After(time.Now()) {
		return IssuedCapability{}, domain.ErrCapabilityInvalid
	}
	token, err := h.issuer.Issue(ctx, service.CapabilityRequest{
		OrganizationID: cmd.OrganizationID, WorkspaceID: cmd.WorkspaceID, ActorID: cmd.ActorID, AgentID: cmd.AgentID,
		RunID: cmd.RunID, ResourceIDs: cmd.ResourceIDs, DelegatorResourceIDs: cmd.DelegatorResourceIDs,
		Actions: cmd.Actions, Audience: cmd.Audience, ExpiresAt: cmd.ExpiresAt,
	}, service.RunScope{ResourceIDs: cmd.RunResourceIDs, Actions: cmd.RunActions})
	if err != nil {
		return IssuedCapability{}, err
	}
	if token.Token == "" || token.Hash == "" {
		return IssuedCapability{}, domain.ErrCapabilityInvalid
	}
	record := model.CapabilityRecord{
		ID: cmd.CommandID, TokenHash: token.Hash, OrganizationID: cmd.OrganizationID, WorkspaceID: cmd.WorkspaceID,
		ActorID: cmd.ActorID, AgentID: cmd.AgentID, RunID: cmd.RunID, ResourceIDs: token.ResourceIDs,
		Actions: token.Claims.Actions, Audience: cmd.Audience, ExpiresAt: cmd.ExpiresAt.UTC(),
	}
	if err := h.store.SaveCapability(ctx, record); err != nil {
		if errors.Is(err, domain.ErrIdempotencyConflict) {
			existing, getErr := h.store.GetCapability(ctx, cmd.CommandID)
			if getErr != nil {
				return IssuedCapability{}, getErr
			}
			return replayedCapability(existing), nil
		}
		return IssuedCapability{}, err
	}
	return IssuedCapability{ID: record.ID, Token: token.Token, Hash: token.Hash, ResourceIDs: token.ResourceIDs, ExpiresAt: record.ExpiresAt}, nil
}

func replayedCapability(record model.CapabilityRecord) IssuedCapability {
	return IssuedCapability{ID: record.ID, Hash: record.TokenHash, ResourceIDs: record.ResourceIDs, ExpiresAt: record.ExpiresAt, Replayed: true}
}
