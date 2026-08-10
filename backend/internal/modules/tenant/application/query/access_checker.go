package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type AccessChecker struct {
	store   *persistence.TenantStore
	orgRepo repository.OrganizationRepository
}

func NewAccessChecker(store *persistence.TenantStore, orgRepo repository.OrganizationRepository) *AccessChecker {
	return &AccessChecker{store: store, orgRepo: orgRepo}
}

func (a *AccessChecker) IsWorkspaceMember(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID) (bool, error) {
	return a.store.IsWorkspaceMember(ctx, workspaceID, memberID)
}
