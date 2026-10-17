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

func (f OperationFactory) Descriptors() ([]messaging.OperationDescriptor, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	return []messaging.OperationDescriptor{
		{Type: productoutbound.OperationTypeWorkflowTrigger, Handler: productoutbound.NewWorkflowCompletionOperation(f.Transactor, f.WorkflowCompletion)},
		{Type: productoutbound.OperationTypeJiraTransition, Handler: productoutbound.NewJiraTransitionOperation(f.JiraTransition)},
		{Type: productoutbound.OperationTypeReleasePlanWebhookSend, Handler: productoutbound.NewReleasePlanWebhookOperation(f.ReleasePlanWebhook)},
		{Type: productoutbound.OperationTypeReleasePlanApprovalSend, Handler: productoutbound.NewReleasePlanApprovalOperation(f.ReleasePlanApproval)},
	}, nil
}

func (f OperationFactory) Register(registry messaging.OperationRegistry) error {
	descriptors, err := f.Descriptors()
	if err != nil {
		return err
	}
	return messaging.RegisterOperationDescriptors(registry, descriptors)
}
