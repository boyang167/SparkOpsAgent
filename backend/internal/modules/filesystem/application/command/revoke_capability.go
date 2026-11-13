package command

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/service"
)

type RevokeCapability struct {
	ID uuid.UUID
}

type capabilityVerifier interface {
	Verify(token string, now time.Time) (service.Claims, error)
}

type RevokeCapabilityHandler struct {
	store repository.CapabilityRepository
}

func NewRevokeCapabilityHandler(store repository.CapabilityRepository) *RevokeCapabilityHandler {
	if store == nil {
		panic("revoke capability requires store")
	}
	return &RevokeCapabilityHandler{store: store}
}

func (h *RevokeCapabilityHandler) Handle(ctx context.Context, cmd RevokeCapability) error {
	if cmd.ID == uuid.Nil {
		return domain.ErrCapabilityNotFound
	}
	return h.store.RevokeCapability(ctx, cmd.ID, time.Now().UTC())
}

func AuthorizeCapability(ctx context.Context, verifier capabilityVerifier, store repository.CapabilityRepository, token string, now time.Time) error {
	if verifier == nil || store == nil || token == "" {
		return domain.ErrCapabilityInvalid
	}
	if _, err := verifier.Verify(token, now); err != nil {
		return err
	}
	record, err := store.GetCapabilityByHash(ctx, service.HashToken(token))
	if err != nil {
		return err
	}
	if record.RevokedAt != nil {
		return domain.ErrCapabilityRevoked
	}
	if !record.ExpiresAt.After(now) {
		return domain.ErrCapabilityExpired
	}
	return nil
}
