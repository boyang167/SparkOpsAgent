package acl

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/port"
	wsacl "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/acl"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"gorm.io/gorm"
)

type TenantACL struct {
	inner *wsacl.TenantACL
}

func NewTenantACL(db *gorm.DB) port.WorkspaceChecker {
	return &TenantACL{inner: wsacl.NewTenantACL(db)}
}

func (a *TenantACL) MemberInWorkspace(ctx context.Context, memberID kernel.MemberID, wsID kernel.WorkspaceID) (bool, error) {
	return a.inner.MemberInWorkspace(ctx, memberID, wsID)
}
