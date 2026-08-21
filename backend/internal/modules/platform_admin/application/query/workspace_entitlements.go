package query

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/entitlements"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
)

type GetWorkspaceEntitlementsHandler struct {
	db *gorm.DB
}

func NewGetWorkspaceEntitlementsHandler(db *gorm.DB) *GetWorkspaceEntitlementsHandler {
	return &GetWorkspaceEntitlementsHandler{db: db}
}

func (h *GetWorkspaceEntitlementsHandler) Handle(ctx context.Context, workspaceID kernel.WorkspaceID) (entitlements.WorkspaceEntitlements, error) {
	return entitlements.ResolveCachedForWorkspace(ctx, h.db, workspaceID.String())
}
