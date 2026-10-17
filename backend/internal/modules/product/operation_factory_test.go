package product

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	productoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestOperationFactoryRegistersEveryProductType(t *testing.T) {
	registry := messaging.NewOperationRegistry()
	factory := OperationFactory{
		Transactor: postgres.NewDirectTransactor(),
		WorkflowCompletion: func(context.Context, kernel.ProductWorkflowRunID, bool, string) error {
			return nil
		},
		JiraTransition: func(context.Context, productoutbound.JiraTransitionPayload) error { return nil },
		ReleasePlanWebhook: func(context.Context, kernel.ReleasePlanID, string, string) error {
			return nil
		},
		ReleasePlanApproval: func(context.Context, kernel.ReleasePlanID, string) error { return nil },
	}

	require.NoError(t, factory.Register(registry))
	for _, operationType := range []string{
		productoutbound.OperationTypeWorkflowTrigger,
		productoutbound.OperationTypeJiraTransition,
		productoutbound.OperationTypeReleasePlanWebhookSend,
		productoutbound.OperationTypeReleasePlanApprovalSend,
	} {
		handler, ok := registry.Handler(operationType)
		require.True(t, ok, operationType)
		require.NotNil(t, handler, operationType)
	}
}

func TestOperationFactoryRejectsMissingDependency(t *testing.T) {
	err := (OperationFactory{}).Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "transactor")
}
