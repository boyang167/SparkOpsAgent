package repository

import (
	"time"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
)

type BlobGCContext struct {
	BlobID             uuid.UUID
	State              model.BlobState
	RefCount           int64
	TombstonedAt       time.Time
	LegalHold          bool
	LiveRevisionCount  int64
	ActiveLinkCount    int64
}

func ValidateBlobGCEligible(ctx BlobGCContext, grace time.Duration, now time.Time) error {
	if ctx.LiveRevisionCount > 0 || ctx.ActiveLinkCount > 0 {
		return domain.ErrBlobStillReferenced
	}
	if ctx.LegalHold {
		return domain.ErrRetentionHold
	}
	switch ctx.State {
	case model.BlobStateTombstoned, model.BlobStateGCPending:
	default:
		return domain.ErrBlobGCNotReady
	}
	if ctx.TombstonedAt.IsZero() {
		return domain.ErrBlobGCNotReady
	}
	if ctx.TombstonedAt.Add(grace).After(now) {
		return domain.ErrBlobGCNotReady
	}
	return nil
}
