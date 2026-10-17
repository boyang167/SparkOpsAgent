package workspace_app

import (
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	workspaceoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

type OperationFactory struct {
	Repository repository.ProjectRepository
	Teardown   func() appctx.ProjectTeardown
	Operations messaging.OperationWriter
	Transactor postgres.Transactor
}

func (f OperationFactory) Validate() error {
	switch {
	case f.Repository == nil:
		return fmt.Errorf("workspace_app operation repository is required")
	case f.Teardown == nil:
		return fmt.Errorf("workspace_app operation teardown provider is required")
	case f.Operations == nil:
		return fmt.Errorf("workspace_app operation writer is required")
	case f.Transactor == nil:
		return fmt.Errorf("workspace_app operation transactor is required")
	default:
		return nil
	}
}

func (f OperationFactory) Register(registry messaging.OperationRegistry) error {
	if err := f.Validate(); err != nil {
		return err
	}
	if registry == nil {
		return fmt.Errorf("workspace_app operation registry is required")
	}
	handlers := []struct {
		operationType string
		handler       messaging.OperationHandler
	}{
		{workspaceoutbound.OperationTypeProjectUndeploy,
			workspaceoutbound.NewProjectUndeployHandler(f.Repository, f.Teardown, f.Operations, f.Transactor)},
		{workspaceoutbound.OperationTypePipelineDisable,
			workspaceoutbound.NewPipelineDisableHandler(
				func() workspaceoutbound.PipelineDisabler { return f.Teardown() },
				f.Repository, f.Transactor,
			)},
	}
	for _, item := range handlers {
		if err := registry.Register(item.operationType, item.handler); err != nil {
			return fmt.Errorf("register workspace_app operation %s: %w", item.operationType, err)
		}
	}
	return nil
}
