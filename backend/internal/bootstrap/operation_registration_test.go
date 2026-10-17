package bootstrap

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	deliverymodule "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery"
	deploypersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/infrastructure/persistence"
	deliveryoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/outbound"
	deliverypersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/persistence"
	governancemodule "github.com/sparkops/sparkops-agent/backend/internal/modules/governance"
	governanceoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/outbound"
	productmodule "github.com/sparkops/sparkops-agent/backend/internal/modules/product"
	productoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/outbound"
	productworkflow "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow"
	workflowoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/outbound"
	workspacemodule "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app"
	workspaceoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/outbound"
	workspacepersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/appctx"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type registrationScheduler struct{}

func (registrationScheduler) HandleBuildCompleted(context.Context, kernel.BuildID, string) error {
	return nil
}
func (registrationScheduler) HandleDeployCompleted(context.Context, kernel.DeploymentRunID, string) error {
	return nil
}

type registrationWebhookResolver struct{}

func (registrationWebhookResolver) ResolveWebhook(context.Context, workflowoutbound.NotificationPayload) (string, json.RawMessage, bool, error) {
	return "", nil, false, nil
}

type registrationGitReporter struct{}

func (registrationGitReporter) CurrentStatus(context.Context, workflowoutbound.GitLabStatusPayload) (string, error) {
	return "", nil
}
func (registrationGitReporter) Report(context.Context, workflowoutbound.GitLabStatusPayload) error {
	return nil
}

type registrationCredentialSyncer struct{}

func (registrationCredentialSyncer) SyncForSecret(context.Context, kernel.WorkspaceID, kernel.SecretID, string) error {
	return nil
}

type registrationPolicySyncer struct{}

func (registrationPolicySyncer) SyncMemberRole(context.Context, kernel.WorkspaceID, kernel.RoleID, kernel.MemberID, bool) error {
	return nil
}

type registrationOperationWriter struct{}

func (registrationOperationWriter) Schedule(context.Context, messaging.OperationRequest) error {
	return nil
}

type registrationPipelineDisabler struct{}

func (registrationPipelineDisabler) DisablePipeline(context.Context, kernel.WorkspaceID, kernel.ProjectID) error {
	return nil
}
func (registrationPipelineDisabler) PipelineDisabled(context.Context, kernel.WorkspaceID, kernel.ProjectID) (bool, error) {
	return true, nil
}
func (registrationPipelineDisabler) OnlineClusters(context.Context, kernel.WorkspaceID, kernel.ProjectID) ([]kernel.ClusterInfoID, error) {
	return nil, nil
}
func (registrationPipelineDisabler) HasActiveRun(context.Context, kernel.ProjectID) (bool, error) {
	return false, nil
}
func (registrationPipelineDisabler) StartUndeploy(context.Context, appctx.UndeployRequest) (kernel.DeploymentRunID, error) {
	return kernel.DeploymentRunID{}, nil
}

func TestProductionRegistryAcceptsEveryScheduledTask10HandlerExactlyOnce(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	registry := messaging.NewOperationRegistry()
	factory := productworkflow.OperationFactory{
		Transactor:      postgres.NewDirectTransactor(),
		Scheduler:       registrationScheduler{},
		WebhookResolver: registrationWebhookResolver{},
		MailSender:      func(context.Context, workflowoutbound.NotificationPayload) error { return nil },
		FeishuSender:    func(context.Context, workflowoutbound.NotificationPayload) error { return nil },
		GitLabReporter:  registrationGitReporter{},
	}
	require.NoError(t, factory.Validate())
	require.NoError(t, factory.Register(registry))
	productFactory := productmodule.OperationFactory{
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
	require.NoError(t, productFactory.Validate())
	require.NoError(t, productFactory.Register(registry))
	deployRepo := deploypersistence.NewDeploymentRunRepository(db)
	projectRepo := workspacepersistence.NewProjectRepository(db)
	deliveryFactory := deliverymodule.OperationFactory{
		Webhooks: deliverypersistence.NewWebhookRepository(db), CredentialSync: registrationCredentialSyncer{},
		Deployments: deployRepo, Queue: messaging.NewNoopQueue(),
	}
	require.NoError(t, deliveryFactory.Validate())
	require.NoError(t, deliveryFactory.Register(registry))
	teardown := registrationPipelineDisabler{}
	workspaceFactory := workspacemodule.OperationFactory{
		Repository: projectRepo, Teardown: func() appctx.ProjectTeardown { return teardown },
		Operations: registrationOperationWriter{}, Transactor: postgres.NewDirectTransactor(),
	}
	require.NoError(t, workspaceFactory.Validate())
	require.NoError(t, workspaceFactory.Register(registry))
	governanceFactory := governancemodule.OperationFactory{RoleSync: registrationPolicySyncer{}}
	require.NoError(t, governanceFactory.Validate())
	require.NoError(t, governanceFactory.Register(registry))

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
		require.Error(t, registry.Register(operationType, handler), operationType)
	}
	for _, operationType := range []string{
		productoutbound.OperationTypeWorkflowTrigger,
		productoutbound.OperationTypeJiraTransition,
		productoutbound.OperationTypeReleasePlanWebhookSend,
		productoutbound.OperationTypeReleasePlanApprovalSend,
	} {
		handler, ok := registry.Handler(operationType)
		require.True(t, ok, operationType)
		require.NotNil(t, handler, operationType)
		require.Error(t, registry.Register(operationType, handler), operationType)
	}
	for _, operationType := range []string{
		deliveryoutbound.OperationTypeWebhookSend,
		deliveryoutbound.OperationTypeJenkinsCredentialSync,
		deliveryoutbound.OperationTypeDeployQueuePublish,
		workspaceoutbound.OperationTypeProjectUndeploy,
		workspaceoutbound.OperationTypePipelineDisable,
		governanceoutbound.OperationTypeMemberRoleSync,
	} {
		handler, ok := registry.Handler(operationType)
		require.True(t, ok, operationType)
		require.NotNil(t, handler, operationType)
		require.Error(t, registry.Register(operationType, handler), operationType)
	}
}

func TestProductionOperationFactoryFailsFastOnMissingDependency(t *testing.T) {
	err := (productworkflow.OperationFactory{}).Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "transactor")
	err = (productmodule.OperationFactory{}).Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "transactor")
	err = (deliverymodule.OperationFactory{}).Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "webhook")
	err = (workspacemodule.OperationFactory{}).Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "repository")
	err = (governancemodule.OperationFactory{}).Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "role sync")
}
