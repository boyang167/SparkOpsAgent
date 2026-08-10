package repository

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ExternalIdentity struct {
	Provider string
	Subject  string
	MemberID kernel.MemberID
	Email    string
}

type ExternalIdentityRepository interface {
	FindByProviderSubject(ctx context.Context, provider, subject string) (*ExternalIdentity, error)
	Save(ctx context.Context, identity *ExternalIdentity) error
}
