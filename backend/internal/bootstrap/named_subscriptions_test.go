package bootstrap

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProductionNamedSubscriptionsAreCompleteAndUnique(t *testing.T) {
	expected := map[string]string{
		"product.build-completed":                                "deliveryevent.BuildCompletedName",
		"product.workflow-run-completed":                         "pwfevent.WorkflowRunCompletedName",
		"product.deploy-completed":                               "deliveryevent.DeployCompletedName",
		"product-workflow.build-completed":                       "deliveryevent.BuildCompletedName",
		"product-workflow.deploy-completed":                      "deliveryevent.DeployCompletedName",
		"workspace-app.workspace-created.seed-default-templates": "integration.EventWorkspaceCreated",
		"workspace-app.deploy-completed.archive-coordinator":     "deliveryevent.DeployCompletedName",
		"delivery.build-completed.deployrun":                     "deliveryevent.BuildCompletedName",
		"delivery.build-completed.webhook":                       "deliveryevent.BuildCompletedName",
		"delivery.deploy-completed.webhook":                      "deliveryevent.DeployCompletedName",
		"delivery.secret-updated.credential-sync":                "integration.EventSecretUpdated",
		"notification.run-completed":                             "event.RunCompletedName",
		"notification.workflow-run-completed":                    "workflowevent.WorkflowRunCompletedName",
		"notification.build-completed":                           "deliveryevent.BuildCompletedName",
		"notification.project-review-submitted":                  "workspaceevent.ProjectReviewSubmittedName",
		"governance.workspace-created.bootstrap":                 "integration.EventWorkspaceCreated",
	}
	moduleFiles := []string{
		"../modules/product/module.go",
		"../modules/product_workflow/module.go",
		"../modules/delivery/module.go",
		"../modules/workspace_app/module.go",
		"../modules/governance/module.go",
		"../modules/notification/module.go",
	}
	subscribe := regexp.MustCompile(`(?s)\.Subscribe\(\s*([^,]+),\s*"([^"]+)"`)
	actual := make(map[string]string, len(expected))

	for _, relative := range moduleFiles {
		raw, err := os.ReadFile(filepath.Clean(relative))
		require.NoError(t, err)
		for _, match := range subscribe.FindAllStringSubmatch(string(raw), -1) {
			eventName := strings.TrimSpace(match[1])
			subscriber := match[2]
			_, duplicate := actual[subscriber]
			require.Falsef(t, duplicate, "duplicate production subscriber name %q", subscriber)
			actual[subscriber] = eventName
		}
	}

	require.Equal(t, expected, actual)
}

func TestExternalSubscribersScheduleOperationsWithoutSentinels(t *testing.T) {
	cases := []struct {
		subscriber  string
		handlerFile string
		operation   string
	}{
		{
			subscriber: "product.workflow-run-completed", handlerFile: "../modules/product/application/subscriber/on_workflow_run_completed.go",
			operation: "OperationTypeWorkflowTrigger",
		},
		{
			subscriber: "product-workflow.build-completed", handlerFile: "../modules/product_workflow/application/subscriber/on_build_completed.go",
			operation: "OperationTypeBuildTrigger",
		},
		{
			subscriber: "product-workflow.deploy-completed", handlerFile: "../modules/product_workflow/application/subscriber/on_deploy_completed.go",
			operation: "OperationTypeDeployTrigger",
		},
		{
			subscriber: "workspace-app.deploy-completed.archive-coordinator", handlerFile: "../modules/workspace_app/application/command/archive_coordinator.go",
			operation: "OperationTypeProjectUndeploy",
		},
		{
			subscriber: "delivery.build-completed.deployrun", handlerFile: "../modules/delivery/deployrun/application/subscriber/on_build_completed.go",
			operation: "OperationTypeDeployQueuePublish",
		},
		{
			subscriber: "delivery.build-completed.webhook", handlerFile: "../modules/delivery/infrastructure/outbound/dispatcher.go",
			operation: "OperationTypeWebhookSend",
		},
		{
			subscriber: "delivery.deploy-completed.webhook", handlerFile: "../modules/delivery/infrastructure/outbound/dispatcher.go",
			operation: "OperationTypeWebhookSend",
		},
		{
			subscriber: "delivery.secret-updated.credential-sync", handlerFile: "../modules/delivery/deployrun/application/subscriber/on_secret_updated.go",
			operation: "OperationTypeJenkinsCredentialSync",
		},
	}

	for _, tt := range cases {
		t.Run(tt.subscriber, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Clean(tt.handlerFile))
			require.NoError(t, err)
			source := string(raw)
			require.NotContains(t, source, "ErrOperationOutboxRequired")
			require.Contains(t, source, tt.operation)
			require.Contains(t, source, ".Schedule(ctx, messaging.OperationRequest{")
		})
	}
}
