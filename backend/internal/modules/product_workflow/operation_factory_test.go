package product_workflow

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	workflowoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type operationSchedulerStub struct{}

func (operationSchedulerStub) HandleBuildCompleted(context.Context, kernel.BuildID, string) error {
	return nil
}
func (operationSchedulerStub) HandleDeployCompleted(context.Context, kernel.DeploymentRunID, string) error {
	return nil
}

type operationWebhookResolverStub struct{}

func (operationWebhookResolverStub) ResolveWebhook(context.Context, workflowoutbound.NotificationPayload) (string, json.RawMessage, bool, error) {
	return "", nil, false, nil
}

type operationGitReporterStub struct{}

func (operationGitReporterStub) CurrentStatus(context.Context, workflowoutbound.GitLabStatusPayload) (string, error) {
	return "", nil
}
func (operationGitReporterStub) Report(context.Context, workflowoutbound.GitLabStatusPayload) error {
	return nil
}

func TestOperationFactoryRegistersEveryProductWorkflowType(t *testing.T) {
	registry := messaging.NewOperationRegistry()
	factory := OperationFactory{
		Transactor:      postgres.NewDirectTransactor(),
		Scheduler:       operationSchedulerStub{},
		WebhookResolver: operationWebhookResolverStub{},
		MailSender:      func(context.Context, workflowoutbound.NotificationPayload) error { return nil },
		FeishuSender:    func(context.Context, workflowoutbound.NotificationPayload) error { return nil },
		GitLabReporter:  operationGitReporterStub{},
	}

	require.NoError(t, factory.Register(registry))
	for _, operationType := range []string{
		workflowoutbound.OperationTypeBuildTrigger,
		workflowoutbound.OperationTypeDeployTrigger,
		workflowoutbound.OperationTypeWebhookSend,
		workflowoutbound.OperationTypeMailSend,
		workflowoutbound.OperationTypeFeishuUserSend,
		workflowoutbound.OperationTypeGitLabStatusReport,
	} {
		handler, ok := registry.Handler(operationType)
		require.True(t, ok, operationType)
		require.NotNil(t, handler, operationType)
	}
}

func TestOperationFactoryRejectsMissingDependency(t *testing.T) {
	err := (OperationFactory{}).Register(messaging.NewOperationRegistry())
	require.Error(t, err)
	require.Contains(t, err.Error(), "transactor")
}
