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
		ReleasePlanWebhook: func(context.Context, kernel.ReleasePlanID, string, string, string) error {
			return nil
		},
		ReleasePlanApproval: func(context.Context, kernel.ReleasePlanID, string) error { return nil },
	}

	descriptors, err := factory.Descriptors()
	require.NoError(t, err)
	require.NoError(t, factory.Register(registry))
	for _, descriptor := range descriptors {
		handler, ok := registry.Handler(descriptor.Type)
		require.True(t, ok, descriptor.Type)
		require.NotNil(t, handler, descriptor.Type)
	}
}

func TestOperationFactoryRejectsMissingDependency(t *testing.T) {
	err := (OperationFactory{}).Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "transactor")
}
