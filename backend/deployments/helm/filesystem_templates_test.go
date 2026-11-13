package helm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFilesystemProductionValuesRequireTwoReplicas(t *testing.T) {
	values := readChartValues(t, "values-production.yaml")
	workloads := map[string]string{
		"fileTransferGateway": "file-transfer-gateway-deployment.yaml",
		"fileWorker":          "file-worker-deployment.yaml",
		"sftpGateway":         "sftp-gateway-deployment.yaml",
	}
	for workload, templateName := range workloads {
		workloadValues, ok := values[workload].(map[string]any)
		require.True(t, ok, "%s values are missing", workload)
		replicas, ok := workloadValues["replicaCount"].(int)
		require.True(t, ok, "%s replicaCount is missing", workload)
		require.GreaterOrEqual(t, replicas, 2, "%s must have at least two production replicas", workload)
		require.Contains(t, readChartTemplate(t, templateName), "replicas: {{ .Values."+workload+".replicaCount }}")
	}

	networkPolicy := readChartTemplate(t, "file-networkpolicy.yaml")
	require.Contains(t, networkPolicy, `app.kubernetes.io/component: file-transfer-gateway`)
}

func TestFilesystemWorkloadsUseRestrictedPodSecurity(t *testing.T) {
	for _, name := range []string{
		"file-transfer-gateway-deployment.yaml",
		"file-worker-deployment.yaml",
		"sftp-gateway-deployment.yaml",
		"file-reconciler-deployment.yaml",
	} {
		template := readChartTemplate(t, name)
		require.Contains(t, template, "runAsNonRoot: true", name)
		require.Contains(t, template, "readOnlyRootFilesystem: true", name)
		require.Contains(t, template, "drop:", name)
		require.Contains(t, template, "- ALL", name)
		require.Contains(t, template, "topologySpreadConstraints:", name)
		require.Contains(t, template, "resources:", name)
	}
}

func TestFilesystemValuesSchemaExists(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(chartDir(), "values.schema.json"))
	require.NoError(t, err)
	var schema map[string]any
	require.NoError(t, json.Unmarshal(data, &schema))
	properties, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	for _, key := range []string{
		"filesystem", "fileTransferGateway", "fileWorker", "sftpGateway", "fileReconciler",
	} {
		require.Contains(t, properties, key)
	}
}

func readChartValues(t *testing.T, name string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(chartDir(), name))
	require.NoError(t, err)
	var values map[string]any
	require.NoError(t, yaml.Unmarshal(data, &values))
	return values
}

func chartDir() string {
	return filepath.Join("..", "..", "..", "deployments", "helm", "sparkops")
}
