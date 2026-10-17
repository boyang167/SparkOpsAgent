package product

import (
	"fmt"

	productoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

type OperationFactory struct {
	Transactor          postgres.Transactor
	WorkflowCompletion  productoutbound.WorkflowCompletionHandler
	JiraTransition      productoutbound.JiraTransitionHandler
	ReleasePlanWebhook  productoutbound.ReleasePlanWebhookHandler
	ReleasePlanApproval productoutbound.ReleasePlanApprovalHandler
}

func (f OperationFactory) Validate() error {
	switch {
	case f.Transactor == nil:
		return fmt.Errorf("product operation transactor is required")
	case f.WorkflowCompletion == nil:
		return fmt.Errorf("product workflow completion operation is required")
	case f.JiraTransition == nil:
		return fmt.Errorf("product Jira transition operation is required")
	case f.ReleasePlanWebhook == nil:
		return fmt.Errorf("product release plan webhook operation is required")
	case f.ReleasePlanApproval == nil:
		return fmt.Errorf("product release plan approval operation is required")
	default:
		return nil
	}
}

func (f OperationFactory) Register(registry messaging.OperationRegistry) error {
	if err := f.Validate(); err != nil {
		return err
	}
	if registry == nil {
		return fmt.Errorf("product operation registry is required")
	}
	handlers := []struct {
		operationType string
		handler       messaging.OperationHandler
	}{
		{productoutbound.OperationTypeWorkflowTrigger, productoutbound.NewWorkflowCompletionOperation(f.Transactor, f.WorkflowCompletion)},
		{productoutbound.OperationTypeJiraTransition, productoutbound.NewJiraTransitionOperation(f.JiraTransition)},
		{productoutbound.OperationTypeReleasePlanWebhookSend, productoutbound.NewReleasePlanWebhookOperation(f.ReleasePlanWebhook)},
		{productoutbound.OperationTypeReleasePlanApprovalSend, productoutbound.NewReleasePlanApprovalOperation(f.ReleasePlanApproval)},
	}
	for _, item := range handlers {
		if err := registry.Register(item.operationType, item.handler); err != nil {
			return fmt.Errorf("register product operation %s: %w", item.operationType, err)
		}
	}
	return nil
}
