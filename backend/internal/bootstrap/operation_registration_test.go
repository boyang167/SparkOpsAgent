package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProductionModulesRegisterEveryTask10OperationExactlyOnce(t *testing.T) {
	root := filepath.Join("..", "modules")
	files := []string{
		"delivery/module.go",
		"workspace_app/module.go",
		"product_workflow/module.go",
		"product/module.go",
		"governance/module.go",
	}
	var source strings.Builder
	for _, name := range files {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		require.NoError(t, err)
		source.Write(raw)
	}
	for _, symbol := range []string{
		"OperationTypeJenkinsCredentialSync",
		"OperationTypeDeployQueuePublish",
		"OperationTypeProjectUndeploy",
		"OperationTypePipelineDisable",
		"OperationTypeBuildTrigger",
		"OperationTypeDeployTrigger",
		"OperationTypeWorkflowTrigger",
		"OperationTypeJiraTransition",
		"OperationTypeReleasePlanWebhookSend",
		"OperationTypeMemberRoleSync",
	} {
		require.Equalf(t, 1, strings.Count(source.String(), symbol), "operation registration %s", symbol)
	}
	require.Equal(t, 2, strings.Count(source.String(), "OperationTypeWebhookSend"),
		"delivery and product-workflow each own one webhook operation")
}
