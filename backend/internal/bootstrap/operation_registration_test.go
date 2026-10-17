package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/require"

	deliveryoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/infrastructure/outbound"
	governanceoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/outbound"
	productoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product/infrastructure/outbound"
	workflowoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/outbound"
	workspaceoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
)

func TestProductionRegistryAcceptsEveryScheduledTask10HandlerExactlyOnce(t *testing.T) {
	registry := messaging.NewOperationRegistry()
	handlers := map[string]messaging.OperationHandler{
		deliveryoutbound.OperationTypeWebhookSend:           deliveryoutbound.NewWebhookHandler(nil, nil).Handle,
		deliveryoutbound.OperationTypeJenkinsCredentialSync: deliveryoutbound.NewCredentialSyncHandler(nil),
		deliveryoutbound.OperationTypeDeployQueuePublish:    deliveryoutbound.NewDeployQueueHandler(nil, nil),
		workspaceoutbound.OperationTypeProjectUndeploy:      workspaceoutbound.NewProjectUndeployHandler(nil, nil, nil),
		workspaceoutbound.OperationTypePipelineDisable:      workspaceoutbound.NewPipelineDisableHandler(nil, nil, nil),
		workflowoutbound.OperationTypeBuildTrigger:          workflowoutbound.NewBuildCompletionHandler(nil, nil),
		workflowoutbound.OperationTypeDeployTrigger:         workflowoutbound.NewDeployCompletionHandler(nil, nil),
		workflowoutbound.OperationTypeWebhookSend:           workflowoutbound.NewWebhookHandler(nil),
		workflowoutbound.OperationTypeMailSend:              workflowoutbound.NewNotificationHandler(nil),
		workflowoutbound.OperationTypeFeishuUserSend:        workflowoutbound.NewNotificationHandler(nil),
		productoutbound.OperationTypeWorkflowTrigger:        productoutbound.NewWorkflowCompletionOperation(nil, nil),
		productoutbound.OperationTypeJiraTransition:         productoutbound.NewJiraTransitionOperation(nil),
		productoutbound.OperationTypeReleasePlanWebhookSend: productoutbound.NewReleasePlanWebhookOperation(nil),
		governanceoutbound.OperationTypeMemberRoleSync:      governanceoutbound.NewMemberRoleSyncHandler(nil),
	}
	require.Len(t, handlers, 14)
	for operationType, handler := range handlers {
		require.NoError(t, registry.Register(operationType, handler), operationType)
		registered, ok := registry.Handler(operationType)
		require.True(t, ok, operationType)
		require.NotNil(t, registered, operationType)
	}
	for operationType, handler := range handlers {
		require.Error(t, registry.Register(operationType, handler), operationType)
	}
}
