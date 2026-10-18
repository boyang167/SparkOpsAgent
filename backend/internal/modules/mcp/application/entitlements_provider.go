package application

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/entitlements"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
)

type WorkspaceEntitlementsProvider interface {
	Resolve(ctx context.Context, workspaceID kernel.WorkspaceID) (entitlements.WorkspaceEntitlements, error)
}

type DBEntitlementsProvider struct {
	DB *gorm.DB
}

func (p DBEntitlementsProvider) Resolve(ctx context.Context, workspaceID kernel.WorkspaceID) (entitlements.WorkspaceEntitlements, error) {
	return entitlements.ResolveCachedForWorkspace(ctx, p.DB, workspaceID.String())
}
