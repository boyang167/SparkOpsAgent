package command

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func validateProductMembership(
	ctx context.Context,
	checker port.ProductChecker,
	wsID kernel.WorkspaceID,
	productID *kernel.ProductID,
) error {
	if productID == nil {
		return nil
	}
	if checker == nil {
		return domain.ErrProductNotInWorkspace
	}
	ok, err := checker.ProductInWorkspace(ctx, wsID, *productID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrProductNotInWorkspace
	}
	return nil
}
