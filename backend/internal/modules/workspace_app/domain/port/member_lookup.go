package port

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type MemberSummary struct {
	ID          kernel.MemberID
	Email       string
	DisplayName string
}

type MemberLookup interface {
	FindByIDs(ctx context.Context, ids []kernel.MemberID) (map[kernel.MemberID]MemberSummary, error)
}
